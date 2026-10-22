// Package people reads real Google sender photos through the documented
// People API contact surfaces only. It deliberately covers just the two
// supported photo sources:
//
//   - people.connections.list with personFields=emailAddresses,photos
//     (ScopeContactsReadonly): the authenticated user's saved contacts,
//     including linked profile photos where Google merges them.
//   - otherContacts.list with readMask=emailAddresses,photos and the
//     documented sources[] parameter READ_SOURCE_TYPE_CONTACT plus
//     READ_SOURCE_TYPE_PROFILE (ScopeOtherContactsReadonly): existing
//     "other contacts".
//
// There is no supported arbitrary-email lookup: Gmail messages expose no
// sender-photo field, people.get needs an account id (not an email), and
// undocumented endpoints are out of scope. A sender absent from both
// datasets stays explicitly uncovered by this file, even when Gmail itself
// renders a photo. The public fallbacks (Gravatar, sender-domain icon)
// live in avatar.go and run after the contact, directory, and BIMI passes.
package people

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the documented People API root. Tests override it.
	DefaultBaseURL = "https://people.googleapis.com/v1"
	// MaxPhotoBytes bounds a single provider image download (R67:
	// server-mediated, bounded decode; never stream unbounded bodies).
	MaxPhotoBytes = 512 * 1024
	// maxListPages caps pagination per surface so a huge address book
	// cannot monopolize a sync pass.
	maxListPages = 20
	// listPageSize balances round trips against response size.
	listPageSize = 500
)

// Photo is one People photo entry. Google defines default=true as a
// placeholder image; only default=false is a user-provided photo.
// https://developers.google.com/people/api/rest/v1/people#Photo
type Photo struct {
	URL      string `json:"url"`
	Default  bool   `json:"default"`
	Metadata struct {
		Primary bool `json:"primary"`
	} `json:"metadata"`
}

// Person carries the two fields the pipeline requests. No other
// personFields are fetched.
type Person struct {
	EmailAddresses []struct {
		Value string `json:"value"`
	} `json:"emailAddresses"`
	Photos []Photo `json:"photos"`
}

// Client fetches People resources with a caller-supplied access token.
// The token comes from the existing Gmail OAuth refresh flow; this package
// never touches refresh tokens or OAuth grants.
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return DefaultBaseURL
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) get(ctx context.Context, accessToken, path string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("people API %s: %s: %s", path, res.Status, truncate(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("people API %s: decode: %w", path, err)
	}
	return nil
}

func truncate(b []byte) string {
	if len(b) > 512 {
		return string(b[:512])
	}
	return string(b)
}

// ListConnections pages people.connections.list with
// personFields=emailAddresses,photos. Callers pass the opaque
// nextPageToken; "" starts. Returns the page and the next token ("" ends).
func (c *Client) ListConnections(ctx context.Context, accessToken, pageToken string) ([]Person, string, error) {
	q := url.Values{
		"resourceName": {"people/me"},
		"personFields": {"emailAddresses,photos"},
		"pageSize":     {fmt.Sprint(listPageSize)},
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var out struct {
		Connections    []Person `json:"connections"`
		NextPageToken  string   `json:"nextPageToken"`
		NextSyncToken  string   `json:"nextSyncToken"`
	}
	_ = out.NextSyncToken
	if err := c.get(ctx, accessToken, "/people/me/connections", q, &out); err != nil {
		return nil, "", err
	}
	return out.Connections, out.NextPageToken, nil
}

// ListOtherContacts pages otherContacts.list with
// readMask=emailAddresses,photos and the documented repeated sources[] query
// values READ_SOURCE_TYPE_CONTACT and READ_SOURCE_TYPE_PROFILE. Profile-only
// is disallowed; otherContacts.search cannot return photos, so the list
// endpoint is the photo pipeline.
func (c *Client) ListOtherContacts(ctx context.Context, accessToken, pageToken string) ([]Person, string, error) {
	q := url.Values{
		"readMask": {"emailAddresses,photos"},
		"pageSize": {fmt.Sprint(listPageSize)},
		"sources": {"READ_SOURCE_TYPE_CONTACT", "READ_SOURCE_TYPE_PROFILE"},
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var out struct {
		OtherContacts []Person `json:"otherContacts"`
		NextPageToken string   `json:"nextPageToken"`
	}
	if err := c.get(ctx, accessToken, "/otherContacts", q, &out); err != nil {
		return nil, "", err
	}
	return out.OtherContacts, out.NextPageToken, nil
}

// ListDirectoryPeople pages people.listDirectoryPeople with
// readMask=emailAddresses,photos over the Workspace domain directory and
// domain profiles. It needs directory.readonly; errors go to the caller.
func (c *Client) ListDirectoryPeople(ctx context.Context, accessToken, pageToken string) ([]Person, string, error) {
	q := url.Values{
		"readMask": {"emailAddresses,photos"},
		"pageSize": {fmt.Sprint(listPageSize)},
		"sources":  {"DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT", "DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE"},
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var out struct {
		People        []Person `json:"people"`
		NextPageToken string   `json:"nextPageToken"`
	}
	if err := c.get(ctx, accessToken, "/people:listDirectoryPeople", q, &out); err != nil {
		return nil, "", err
	}
	return out.People, out.NextPageToken, nil
}

// BuildIndex paginates both surfaces and builds an account-scoped
// normalized-email -> non-default photo URL index. Exact mailbox match
// only: keys are lowercased trimmed addr-specs; display names are never
// matched and dots/plus-tags are never rewritten. When several eligible
// photos exist for one address the primary one wins; default=true entries
// are rejected. Later surfaces do not overwrite an earlier primary hit.
//
// A failure on one surface does not discard the other: production 503 on
// /otherContacts previously aborted after a completed connections list and
// stored nothing. Empty index plus a surface error still returns that error.
func (c *Client) BuildIndex(ctx context.Context, accessToken string) (map[string]string, error) {
	return c.buildIndex(ctx, accessToken, c.ListConnections, c.ListOtherContacts)
}

// BuildDirectoryIndex pages people.listDirectoryPeople and applies the same
// exact-match and default-rejection rules as BuildIndex. It needs
// directory.readonly; callers treat any error as "no directory photos".
func (c *Client) BuildDirectoryIndex(ctx context.Context, accessToken string) (map[string]string, error) {
	return c.buildIndex(ctx, accessToken, c.ListDirectoryPeople)
}

type personLister func(context.Context, string, string) ([]Person, string, error)

func (c *Client) buildIndex(ctx context.Context, accessToken string, surfaces ...personLister) (map[string]string, error) {
	index := map[string]string{}
	primaries := map[string]bool{}
	absorb := func(persons []Person) {
		for _, p := range persons {
			url, primary := selectPhoto(p.Photos)
			if url == "" {
				continue
			}
			for _, e := range p.EmailAddresses {
				key := NormalizeEmail(e.Value)
				if key == "" {
					continue
				}
				if _, ok := index[key]; !ok || (primary && !primaries[key]) {
					index[key] = url
					primaries[key] = primary
				}
			}
		}
	}
	var firstErr error
	page := func(list personLister) {
		token := ""
		for range maxListPages {
			persons, next, err := list(ctx, accessToken, token)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			absorb(persons)
			if next == "" {
				return
			}
			token = next
			if ctx.Err() != nil {
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				return
			}
		}
	}
	for _, list := range surfaces {
		page(list)
	}
	if len(index) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return index, nil
}

// selectPhoto picks the eligible photo URL: first non-default, preferring
// a primary entry. Returns ("", false) when every entry is a default
// placeholder or no URL is present.
func selectPhoto(photos []Photo) (string, bool) {
	fallback := ""
	for _, p := range photos {
		if p.Default || strings.TrimSpace(p.URL) == "" {
			continue
		}
		if p.Metadata.Primary {
			return p.URL, true
		}
		if fallback == "" {
			fallback = p.URL
		}
	}
	return fallback, false
}

// NormalizeEmail lowercases and trims one addr-spec for exact matching.
// Empty or obviously non-mailbox values yield "".
func NormalizeEmail(addr string) string {
	key := strings.ToLower(strings.TrimSpace(addr))
	if key == "" || !strings.Contains(key, "@") {
		return ""
	}
	return key
}
