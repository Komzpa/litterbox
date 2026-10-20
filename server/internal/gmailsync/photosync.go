package gmailsync

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

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
	photoFetchBudget = 20
	senderListBudget = 200
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
	// Incremental consent gate: without both photo scopes on record, make
	// no People call at all. The status endpoint reports
	// authorization_required; the consent route builds the grant URL.
	if !gmail.HasPhotoScopes(granted) {
		return nil
	}
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	httpClient := c.HTTP
	if PhotoHTTPClient != nil {
		httpClient = PhotoHTTPClient
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	peopleBase := PeopleBaseURL
	if peopleBase == "" {
		peopleBase = people.DefaultBaseURL
	}
	peopleClient := &people.Client{HTTP: httpClient, BaseURL: peopleBase}
	indexCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	index, err := peopleClient.BuildIndex(indexCtx, token)
	if err != nil {
		if isPhotoAuthError(err) {
			return errPhotoAuthRequired
		}
		return err
	}
	senders, err := recentSenders(ctx, s, a)
	if err != nil {
		return err
	}
	fetched, stored := 0, 0
	for _, sender := range senders {
		if fetched >= photoFetchBudget {
			break
		}
		key := senderphoto.NormalizeAddress(sender)
		if key == "" {
			continue
		}
		photoURL, ok := index[key]
		if !ok || strings.TrimSpace(photoURL) == "" {
			continue // explicitly uncovered: absent from both datasets
		}
		fetched++
		raw, contentType, err := people.FetchPhoto(indexCtx, httpClient, photoURL)
		if err != nil {
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
		stored++
	}
	_ = stored
	return nil
}

// recentSenders returns up to senderListBudget distinct non-empty sender
// addr-specs for the account, newest first, from messages then cards.
func recentSenders(ctx context.Context, s *Syncer, a Account) ([]string, error) {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT DISTINCT m.sender_address FROM messages m
JOIN cards c ON c.tenant_id=m.tenant_id AND c.id=m.card_id
WHERE m.tenant_id=$1 AND c.account_id=$2 AND m.sender_address<>''
ORDER BY m.sender_address LIMIT $3`, a.TenantID, a.ID, senderListBudget)
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
