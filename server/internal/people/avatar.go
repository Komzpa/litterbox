// Gravatar + domain-icon fallback for sender photos.
//
// Order in the pipeline: People contacts, Workspace directory, BIMI, then
// this file. Gmail profile photos of non-contacts are not available to this
// app (see receipts/avatar-internet-research.md), so when every prior source
// misses, the sync tries, in order:
//
//  1. Gravatar: https://www.gravatar.com/avatar/<sha256(lower(trim(email)))>?d=404&s=128.
//     A 404 means no photo (ErrNoAvatar) and falls through.
//  2. Domain icon, skipped for personal-mail domains (IsFreeMailDomain):
//     Google s2 favicons, then DuckDuckGo icons, whichever returns a real
//     non-default icon first. The default globe (byte-identical to the
//     reference fetched for an invalid domain) or anything under 32 px is
//     rejected. DDG serves .ico, which image.DecodeConfig cannot verify, so
//     in practice s2 PNGs are what persist; Upsert re-verifies anyway.
//
// No OAuth: all lookups are public. Callers bound fetch counts with the
// existing senderListBudget loop, cache per-domain favicon results per run,
// and persist misses in the process-level miss cache below so the same
// address is not refetched on every sync.
package people

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Override roots in tests. Empty means the documented default.
var (
	GravatarBaseURL     string
	FaviconGoogleBase   string
	FaviconDDGBase      string
	avatarMissTTL       = 7 * 24 * time.Hour
	avatarFetchTimeout  = 15 * time.Second
	avatarInvalidDomain = "nonexistent-invalid-domain-12345.test"
)

func gravatarRoot() string {
	if GravatarBaseURL != "" {
		return strings.TrimSuffix(GravatarBaseURL, "/")
	}
	return "https://www.gravatar.com/avatar"
}

func faviconGoogleRoot() string {
	if FaviconGoogleBase != "" {
		return strings.TrimSuffix(FaviconGoogleBase, "/")
	}
	return "https://www.google.com/s2/favicons"
}

func faviconDDGRoot() string {
	if FaviconDDGBase != "" {
		return strings.TrimSuffix(FaviconDDGBase, "/")
	}
	return "https://icons.duckduckgo.com/ip3"
}

// ErrNoAvatar marks a definitive "no photo here" (Gravatar 404, rejected
// favicon, free-mail skip): safe to cache as a miss. Any other error is
// transient and must NOT be cached.
var ErrNoAvatar = fmt.Errorf("people avatar: no photo")

// GravatarURL returns the d=404 lookup URL for addr, or "" when addr is not
// a usable mailbox. The hash is sha256 over the lowercased trimmed address.
func GravatarURL(addr string) string {
	key := NormalizeEmail(addr)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return gravatarRoot() + "/" + hex.EncodeToString(sum[:]) + "?d=404&s=128"
}

// freeMailDomains are personal-mail providers whose domain icon says nothing
// about the sender, so the favicon pass skips them.
var freeMailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true,
	"outlook.com": true, "hotmail.com": true, "live.com": true, "msn.com": true,
	"yahoo.com": true, "ymail.com": true, "rocketmail.com": true,
	"icloud.com": true, "me.com": true, "mac.com": true,
	"proton.me": true, "protonmail.com": true, "pm.me": true,
	"mail.ru": true, "inbox.ru": true, "list.ru": true, "bk.ru": true,
	"yandex.ru": true, "yandex.com": true, "ya.ru": true,
	"gmx.de": true, "gmx.com": true, "gmx.net": true,
	"aol.com": true, "zoho.com": true, "tutanota.com": true, "tuta.com": true,
}

// IsFreeMailDomain reports whether domain is a personal-mail provider.
func IsFreeMailDomain(domain string) bool {
	return freeMailDomains[strings.ToLower(strings.TrimSpace(domain))]
}

// FetchGravatar downloads the Gravatar photo for addr. A 404 becomes
// ErrNoAvatar (fall through to the domain icon); any other failure is a
// transient error. Returned bytes are bounded and DecodeConfig-verified.
func FetchGravatar(ctx context.Context, client *http.Client, addr string) ([]byte, string, error) {
	rawURL := GravatarURL(addr)
	if rawURL == "" {
		return nil, "", ErrNoAvatar
	}
	return fetchAvatarImage(ctx, client, rawURL)
}

// FetchDefaultGlobe returns the reference default-icon bytes: what the
// Google s2 endpoint serves for a domain that has no icon. Callers
// byte-compare candidates against it. Nil when the reference itself is
// unfetchable; then only the 32 px rule applies.
func FetchDefaultGlobe(ctx context.Context, client *http.Client) []byte {
	u := faviconGoogleRoot() + "?domain=" + url.QueryEscape(avatarInvalidDomain) + "&sz=128"
	raw, _, _, err := fetchAvatarBytes(ctx, client, u)
	if err != nil {
		return nil
	}
	return raw
}

// FetchFavicon returns a real non-default icon for domain, or ErrNoAvatar.
// Google s2 is tried first, then DuckDuckGo; the first candidate that is a
// decodable image at least 32x32 px and not byte-identical to globeRef wins.
func FetchFavicon(ctx context.Context, client *http.Client, domain string, globeRef []byte) ([]byte, string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" || IsFreeMailDomain(domain) {
		return nil, "", ErrNoAvatar
	}
	candidates := []string{
		faviconGoogleRoot() + "?domain=" + url.QueryEscape(domain) + "&sz=128",
		faviconDDGRoot() + "/" + domain + ".ico",
	}
	sawTransient := false
	for _, u := range candidates {
		raw, ct, err := fetchAvatarImage(ctx, client, u)
		if err != nil {
			if err == ErrNoAvatar {
				continue
			}
			sawTransient = true
			continue
		}
		if len(globeRef) > 0 && bytes.Equal(raw, globeRef) {
			continue
		}
		if !atLeast32px(raw) {
			continue
		}
		return raw, ct, nil
	}
	if sawTransient {
		return nil, "", fmt.Errorf("people avatar: favicon fetch failed")
	}
	return nil, "", ErrNoAvatar
}

// atLeast32px rejects anything smaller than 32 px: the default globe and
// upsized dots are never icons.
func atLeast32px(raw []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return false
	}
	return cfg.Width >= 32 && cfg.Height >= 32
}

// fetchAvatarImage downloads one candidate URL: bounded, image/*, decodable.
// A 404/410 becomes ErrNoAvatar; anything else failing is transient.
func fetchAvatarImage(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, "", fmt.Errorf("people avatar: unsupported URL")
	}
	raw, contentType, status, err := fetchAvatarBytes(ctx, client, parsed.String())
	if err != nil {
		if status == http.StatusNotFound || status == http.StatusGone {
			return nil, "", ErrNoAvatar
		}
		return nil, "", err
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return nil, "", ErrNoAvatar
	}
	return raw, contentType, nil
}

// fetchAvatarBytes GETs url with a bounded body. It returns the HTTP status
// alongside errors so callers can map 404 to ErrNoAvatar.
func fetchAvatarBytes(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, int, error) {
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	fctx, cancel := context.WithTimeout(ctx, avatarFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("people avatar: build request")
	}
	req.Header.Set("Accept", "image/*")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("people avatar: fetch failed")
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, "", res.StatusCode, fmt.Errorf("people avatar: status %s", res.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, MaxPhotoBytes+1))
	if err != nil {
		return nil, "", res.StatusCode, fmt.Errorf("people avatar: read failed")
	}
	if len(raw) == 0 || len(raw) > MaxPhotoBytes {
		return nil, "", res.StatusCode, fmt.Errorf("people avatar: size out of bounds")
	}
	return raw, normalizeAvatarType(res.Header.Get("Content-Type")), res.StatusCode, nil
}

func normalizeAvatarType(header string) string {
	media := strings.ToLower(strings.TrimSpace(strings.SplitN(header, ";", 2)[0]))
	switch media {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return media
	default:
		return "image/jpeg"
	}
}

// avatarMissCache is the process-level negative cache: tenant/account/email
// keys that definitively yielded no photo (Gravatar 404 + no usable icon,
// or a free-mail skip). Syncs consult it before any external fetch so the
// same address is not refetched on every pass. Positive hits live in
// sender_photos; transient errors are never cached.
type avatarMissCache struct {
	mu   sync.Mutex
	hits map[string]time.Time
}

var avatarMisses = &avatarMissCache{hits: map[string]time.Time{}}

// MissKey builds the negative-cache key for one account-scoped sender.
func MissKey(tenant, account, sender string) string {
	return tenant + "\x00" + account + "\x00" + NormalizeEmail(sender)
}

// HasMiss reports whether key is a fresh (unexpired) miss.
func HasMiss(key string) bool {
	return avatarMisses.has(key)
}

// AddMiss records key as a miss.
func AddMiss(key string) {
	avatarMisses.add(key)
}

// ClearMisses drops the whole negative cache. Tests only.
func ClearMisses() {
	avatarMisses.mu.Lock()
	defer avatarMisses.mu.Unlock()
	avatarMisses.hits = map[string]time.Time{}
}

func (c *avatarMissCache) has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	at, ok := c.hits[key]
	if !ok {
		return false
	}
	if time.Since(at) > avatarMissTTL {
		delete(c.hits, key)
		return false
	}
	return true
}

func (c *avatarMissCache) add(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits[key] = time.Now()
}
