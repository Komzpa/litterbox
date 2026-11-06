package gmailsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/bimi"
	"github.com/Komzpa/litterbox/server/internal/gmail"
	"github.com/Komzpa/litterbox/server/internal/people"
	"github.com/Komzpa/litterbox/server/internal/senderphoto"
)

// PeopleBaseURL overrides the People API root in tests. Empty means the
// documented default. PhotoHTTPClient overrides the HTTP client for People
// and provider image fetches in tests; nil reuses the Gmail client's HTTP.
var (
	PeopleBaseURL    string
	PhotoHTTPClient  *http.Client
	senderListBudget = 200
	// bimiLogoURL resolves a sender addr-spec to a BIMI l= URL. Tests swap
	// it; production uses bimi.LogoURLForEmail (public DNS).
	bimiLogoURL = bimi.LogoURLForEmail
)

// syncSenderPhotos is a best-effort R67 pass at the end of SyncAccount. It
// never fails the account sync: missing grants stay in
// authorization_required, transient People errors are logged by count (never
// with addresses, URLs, or tokens), and stored photos are bounded decoded
// provider images only.
func (s *Syncer) syncSenderPhotos(ctx context.Context, a Account, c *Client) {
	if err := s.trySenderPhotos(ctx, a, c); err != nil {
		// Ground truth for the operator is the status endpoint, not this
		// log line: no addresses, URLs, or tokens here, only counts.
		log.Printf("sender-photos: build=%s account=%s: %v", s.Build, a.ID, err)
	}
}

// errPhotoAuthRequired marks a People 401/403: the stored grant lacks the
// photo scopes (user denied or revoked). The account stays usable for
// Gmail; photos stay in authorization_required until incremental consent.
var errPhotoAuthRequired = photoAuthRequiredError{}

type photoAuthRequiredError struct{}

func (photoAuthRequiredError) Error() string { return "sender photos require incremental consent" }
func isPhotoAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAuthStatus(msg)
}
func containsAuthStatus(msg string) bool {
	for _, token := range []string{"401", "403", "insufficient", "PERMISSION_DENIED", "access_denied"} {
		found := false
		for i := 0; i+len(token) <= len(msg); i++ {
			match := true
			for j := 0; j < len(token); j++ {
				c1 := msg[i+j]
				c2 := token[j]
				if c1 >= 'a' && c1 <= 'z' {
					c1 -= 'a' - 'A'
				}
				if c2 >= 'a' && c2 <= 'z' {
					c2 -= 'a' - 'A'
				}
				if c1 != c2 {
					match = false
					break
				}
			}
			if match {
				found = true
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}
func (s *Syncer) trySenderPhotos(ctx context.Context, a Account, c *Client) error {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	var granted string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(granted_photo_scopes,'') FROM accounts WHERE tenant_id=$1 AND id=$2`, a.TenantID, a.ID).Scan(&granted); err != nil {
		tx.Rollback(ctx)
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	httpClient := c.HTTP
	if PhotoHTTPClient != nil {
		httpClient = PhotoHTTPClient
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	senders, err := recentSenders(ctx, s, a)
	if err != nil {
		return err
	}
	index := map[string]string{}
	// People contact surfaces need both photo scopes. BIMI brand logos are
	// public DNS+HTTPS (no OAuth) and still run when People is gated off.
	if gmail.HasPhotoScopes(granted) {
		token, tokErr := c.token(ctx)
		if tokErr != nil {
			return tokErr
		}
		peopleBase := PeopleBaseURL
		if peopleBase == "" {
			peopleBase = people.DefaultBaseURL
		}
		peopleClient := &people.Client{HTTP: httpClient, BaseURL: peopleBase}
		built, buildErr, _ := s.peopleIndex.get(peopleIndexKey{accountKey{a.TenantID, a.ID}, "people"}, func() (map[string]string, error) {
			pctx, pcancel := context.WithTimeout(ctx, 90*time.Second)
			defer pcancel()
			return peopleClient.BuildIndex(pctx, token)
		})
		if buildErr != nil {
			if isPhotoAuthError(buildErr) {
				return errPhotoAuthRequired
			}
			// Keep an empty index and still try BIMI for brand senders.
			log.Printf("sender-photos: build=%s account=%s: people index: %v (next attempt backs off)", s.Build, a.ID, buildErr)
		}
		// A failed rebuild keeps serving the last good index.
		if built != nil {
			for k, v := range built {
				index[k] = v
			}
		}
	}
	// Workspace colleagues need directory.readonly. This pass is best-effort:
	// any error is logged by message only and never blocks People or BIMI, and
	// never counts as a photo-auth error for the account.
	directoryKeys := map[string]bool{}
	if gmail.HasDirectoryScope(granted) {
		token, tokErr := c.token(ctx)
		if tokErr != nil {
			log.Printf("sender-photos: build=%s account=%s: directory token: %v", s.Build, a.ID, tokErr)
		} else {
			base := PeopleBaseURL
			if base == "" {
				base = people.DefaultBaseURL
			}
			dir, dErr, _ := s.peopleIndex.get(peopleIndexKey{accountKey{a.TenantID, a.ID}, "directory"}, func() (map[string]string, error) {
				dctx, dcancel := context.WithTimeout(ctx, 90*time.Second)
				defer dcancel()
				return (&people.Client{HTTP: httpClient, BaseURL: base}).BuildDirectoryIndex(dctx, token)
			})
			if dErr != nil {
				log.Printf("sender-photos: build=%s account=%s: directory index skipped: %v (next attempt backs off)", s.Build, a.ID, dErr)
			}
			if dir != nil {
				for k, v := range dir {
					if _, ok := index[k]; !ok {
						index[k] = v
						directoryKeys[k] = true
					}
				}
			}
		}
	}
	indexCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	fetched, stored, bimiStored := 0, 0, 0
	gravatarStored, faviconStored := 0, 0
	globeFetched := false
	var globeRef []byte
	var samples []string
	for _, sender := range senders {
		key := senderphoto.NormalizeAddress(sender)
		if key == "" {
			continue
		}
		senderKey := photoCacheKey{accountKey{a.TenantID, a.ID}, "sender", key + "|" + granted + "|" + index[key]}
		if _, fresh := s.photoResults.get(senderKey); fresh {
			continue
		}
		transientFailure := false
		var raw []byte
		var contentType string
		source := ""
		if photoURL, ok := index[key]; ok && strings.TrimSpace(photoURL) != "" {
			fetched++
			raw, contentType, err = people.FetchPhoto(indexCtx, httpClient, photoURL)
			if err != nil {
				raw = nil
				transientFailure = true
			} else {
				if directoryKeys[key] {
					source = "directory"
				} else {
					source = "people"
				}
			}
		}
		if raw == nil {
			at := strings.LastIndexByte(key, '@')
			if at > 0 && at < len(key)-1 {
				dom := key[at+1:]
				domainKey := photoCacheKey{accountKey{a.TenantID, a.ID}, "bimi", dom}
				logo, ok := s.photoResults.get(domainKey)
				if !ok {
					logo.url, _ = bimiLogoURL(indexCtx, key)
					// DNS failure and absence share a shorter retry period.
					s.photoResults.put(domainKey, logo, senderPhotoRetryTTL)
				}
				logoURL := logo.url
				if logoURL != "" {
					if len(logo.raw) > 0 {
						raw, contentType = logo.raw, logo.ct
						err = nil
					} else {
						fetched++
						raw, contentType, err = bimi.FetchLogo(indexCtx, httpClient, logoURL)
					}
					if err != nil {
						raw = nil
						transientFailure = true
					} else {
						source = "bimi"
						s.photoResults.put(domainKey, photoCacheEntry{url: logoURL, raw: raw, ct: contentType}, senderPhotoTTL)
					}
				}
			}
		}
		// Gravatar + domain-icon fallback: only when People, directory
		// and BIMI all missed. Gravatar first; its 404 falls through to
		// the domain icon (skipped for personal-mail domains). Definitive
		// negatives use a six-hour sender TTL. Transient failures retry in
		// fifteen minutes; domain results survive subsequent account polls.
		if raw == nil {
			if indexCtx.Err() == nil {
				transient := false
				fetched++
				raw, contentType, err = people.FetchGravatar(indexCtx, httpClient, key)
				if err != nil {
					raw = nil
					if errors.Is(err, people.ErrNoAvatar) {
						at := strings.LastIndexByte(key, '@')
						dom := ""
						if at > 0 && at < len(key)-1 {
							dom = key[at+1:]
						}
						if dom != "" && !people.IsFreeMailDomain(dom) {
							faviconKey := photoCacheKey{accountKey{a.TenantID, a.ID}, "favicon", dom}
							fr, ok := s.photoResults.get(faviconKey)
							ferr := people.ErrNoAvatar
							if !ok {
								if !globeFetched {
									globeRef = people.FetchDefaultGlobe(indexCtx, httpClient)
									globeFetched = true
								}
								fetched++
								fraw, fct, ferr2 := people.FetchFavicon(indexCtx, httpClient, dom, globeRef)
								ferr = ferr2
								if ferr2 == nil {
									fr = photoCacheEntry{raw: fraw, ct: fct}
								} else if errors.Is(ferr2, people.ErrNoAvatar) {
									fr = photoCacheEntry{}
								} else {
									transient = true
								}
								if len(fr.raw) > 0 || errors.Is(ferr, people.ErrNoAvatar) {
									s.photoResults.put(faviconKey, fr, senderPhotoMissTTL)
								}
							}
							if len(fr.raw) > 0 {
								raw, contentType = fr.raw, fr.ct
								source = "favicon"
							} else if !ok && !errors.Is(ferr, people.ErrNoAvatar) {
								transient = true
							}
						}
					}
				} else {
					source = "gravatar"
				}
				if err != nil && !errors.Is(err, people.ErrNoAvatar) {
					transient = true
				}
				transientFailure = transientFailure || transient
			}
		}
		if raw == nil || source == "" {
			ttl := senderPhotoMissTTL
			if transientFailure {
				ttl = senderPhotoRetryTTL
			}
			if indexCtx.Err() == nil {
				s.photoResults.put(senderKey, photoCacheEntry{}, ttl)
			}
			continue
		}
		wtx, err := s.beginTenant(ctx, a.TenantID)
		if err != nil {
			continue
		}
		if err = senderphoto.Upsert(ctx, wtx, a.TenantID.String(), a.ID.String(), key, raw, contentType); err != nil {
			wtx.Rollback(ctx)
			continue
		}
		if err = wtx.Commit(ctx); err != nil {
			continue
		}
		ttl := senderPhotoTTL
		if transientFailure {
			ttl = senderPhotoRetryTTL
		}
		s.photoResults.put(senderKey, photoCacheEntry{}, ttl)
		stored++
		switch source {
		case "bimi":
			bimiStored++
		case "gravatar":
			gravatarStored++
		case "favicon":
			faviconStored++
		}
		if len(samples) < 10 {
			sum := sha256.Sum256([]byte(key))
			samples = append(samples, hex.EncodeToString(sum[:])[:8]+":"+source)
		}
	}
	log.Printf("sender-photos: build=%s account=%s: ok index=%d senders=%d fetched=%d stored=%d bimi=%d gravatar=%d favicon=%d samples=%v", s.Build, a.ID, len(index), len(senders), fetched, stored, bimiStored, gravatarStored, faviconStored, samples)
	return nil
}

// recentSenders returns up to senderListBudget distinct effective senders
// for the account: latest message addr-spec, else cards.sender_address.
// That is the same key markSenderPhotos and GET /sender-photo use, so a
// stored photo can flag a card. Older-in-thread addresses are not fetched.
func recentSenders(ctx context.Context, s *Syncer, a Account) ([]string, error) {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT addr FROM (
SELECT DISTINCT LOWER(TRIM(COALESCE(
  (SELECT m.sender_address FROM messages m
   WHERE m.tenant_id=c.tenant_id AND m.card_id=c.id AND m.sender_address<>''
   ORDER BY m.received_at DESC LIMIT 1),
  NULLIF(c.sender_address,'')))) AS addr
FROM cards c
WHERE c.tenant_id=$1 AND c.account_id=$2
) s WHERE addr IS NOT NULL AND addr<>'' LIMIT $3`, a.TenantID, a.ID, senderListBudget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var addr string
		if err = rows.Scan(&addr); err != nil {
			return nil, err
		}
		if strings.TrimSpace(addr) != "" {
			out = append(out, addr)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
