package mailbody

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/Komzpa/litterbox/server/internal/mailhtml"
	"github.com/Komzpa/litterbox/server/internal/mailhtml/trackers"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
)

type Handler struct {
	DB     *sql.DB
	Images *mailhtml.ImageFetcher
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := agents.TenantFrom(r.Context())
	if !ok || tenantID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cardID := r.PathValue("id")
	if cardID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenantID); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	var source string
	var account, threadID, address sql.NullString
	var raw string
	err = tx.QueryRowContext(r.Context(), `SELECT c.source,c.account_id::text,c.gmail_thread_id,a.address,b.html FROM cards c JOIN card_bodies b ON b.tenant_id=c.tenant_id AND b.card_id=c.id LEFT JOIN accounts a ON a.tenant_id=c.tenant_id AND a.id=c.account_id WHERE c.tenant_id=$1 AND c.id=$2`, tenantID, cardID).Scan(&source, &account, &threadID, &address, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	// Mail bodies keep the Sanitize pipeline and the account-aware Gmail URL.
	// Any other source is a server-built document: serve it exactly as stored,
	// with no account, thread or external destination.
	var clean string
	if source == "mail" {
		clean, err = Sanitize(r.Context(), raw, h.Images)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
	} else {
		clean = raw
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := json.NewEncoder(w).Encode(struct {
		HTML      string `json:"html"`
		ThreadID  string `json:"threadId"`
		AccountID string `json:"accountId"`
		// SourceURL is the authoritative account-aware external destination:
		// the tenant's stored account address selects the session via
		// authuser, never an account UUID or a guessed /u/0 browser index.
		SourceURL string `json:"source_url,omitempty"`
	}{clean, threadID.String, account.String, gmailSourceURL(address.String, threadID.String)}); err != nil {
		return
	}
}

// gmailSourceURL builds the account-aware Gmail thread permalink from the
// authoritative stored account address and Gmail thread id. It returns ""
// when either input is missing instead of fabricating a destination.
func gmailSourceURL(address, threadID string) string {
	if address == "" || threadID == "" {
		return ""
	}
	return "https://mail.google.com/mail/?authuser=" + url.QueryEscape(address) + "#all/" + url.PathEscape(threadID)
}

func Sanitize(ctx context.Context, source string, fetcher *mailhtml.ImageFetcher) (string, error) {
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "iframe" || c.Data == "object" || c.Data == "embed" || c.Data == "form") {
				n.RemoveChild(c)
				c = next
				continue
			}
			if c.Type == html.ElementNode {
				attrs := c.Attr[:0]
				for _, a := range c.Attr {
					key := strings.ToLower(a.Key)
					value := strings.TrimSpace(a.Val)
					if strings.HasPrefix(key, "on") || key == "srcdoc" || key == "formaction" {
						continue
					}
					if key == "href" || key == "src" || key == "xlink:href" {
						low := strings.ToLower(value)
						if strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "data:text/html") {
							continue
						}
						if key == "href" {
							if matched, _ := trackers.IsTracker(value); matched {
								continue
							}
						}
					}
					if c.Data == "img" && key == "src" {
						if tracker(value, c.Attr) {
							n.RemoveChild(c)
							break
						}
						if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
							if fetcher == nil {
								continue
							}
							image, e := fetcher.FetchImage(ctx, value)
							if e != nil {
								continue
							}
							value = "data:" + image.ContentType + ";base64," + base64.StdEncoding.EncodeToString(image.Bytes)
						}
					}
					a.Val = value
					attrs = append(attrs, a)
				}
				c.Attr = attrs
			}
			walk(c)
			c = next
		}
	}
	walk(root)
	var b strings.Builder
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&b, c); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}
func tracker(src string, attrs []html.Attribute) bool {
	if strings.TrimSpace(src) == "" {
		return true
	}
	if matched, _ := trackers.IsTracker(src); matched {
		return true
	}
	values := make(map[string]string, len(attrs))
	for _, a := range attrs {
		values[strings.ToLower(a.Key)] = a.Val
	}
	return trackers.IsTinyOrHidden(values)
}
