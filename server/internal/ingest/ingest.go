package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Card is the single write contract for non-mail sources.
type Card struct {
	ExternalID string          `json:"external_id"`
	Kind       string          `json:"kind"`
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	At         *time.Time      `json:"at"`
	Timed      bool            `json:"timed,omitempty"`
	Order      int             `json:"note_order,omitempty"`
	Actions    json.RawMessage `json:"actions,omitempty"`
	Files      []File          `json:"files,omitempty"`
}

// File is one deliverable attached to a research or brief card. The bytes
// travel base64-encoded in JSON. Only the name is producer-controlled; the
// server sniffs the media type itself and renders the bytes as a data link
// inside the server-built card body.
type File struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Data      []byte `json:"data"`
}

const (
	// maxIngestFiles caps the attachment batch on one card.
	maxIngestFiles = 10
	// maxFilesBytes caps the total decoded attachment payload.
	maxFilesBytes = 8 << 20
	// maxBodyBytes caps any ingest request body.
	maxBodyBytes = 12 << 20
	// maxPlainBodyBytes is the standing 1 MiB body budget; only the
	// researched/brief file batch may push a request past it.
	maxPlainBodyBytes = 1 << 20
)

type Request struct {
	Operation string `json:"operation,omitempty"` // upsert (default) or close
	Card
}

type Auth struct{ TenantID, Source, CallbackURL string }

// Handler authenticates exclusively with the token scoped to its one source.
type Handler struct {
	DB   *sql.DB
	Pool *pgxpool.Pool
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var a Auth
	err := h.DB.QueryRowContext(r.Context(), `SELECT tenant_id::text,source,COALESCE(callback_url,'') FROM litterbox_source_by_token($1)`, hashToken(token)).Scan(&a.TenantID, &a.Source, &a.CallbackURL)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req Request
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
		return
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Operation == "" {
		req.Operation = "upsert"
	}
	if req.ExternalID == "" {
		http.Error(w, "external_id required", http.StatusBadRequest)
		return
	}
	if req.Operation != "upsert" && req.Operation != "close" {
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}
	if req.Operation == "upsert" && (req.Kind == "" || req.Title == "") {
		http.Error(w, "kind and title required", http.StatusBadRequest)
		return
	}
	if req.Operation == "upsert" {
		if noise, reason := isNoiseSummary(req.Summary, req.Title); noise {
			http.Error(w, reason, http.StatusBadRequest)
			return
		}
	}
	files, code := checkedFiles(req, body)
	if code != 0 {
		msg := "invalid files"
		if code == http.StatusRequestEntityTooLarge {
			msg = "request body too large"
		}
		http.Error(w, msg, code)
		return
	}
	req.Files = files
	err = apply(r.Context(), h.DB, a, req)
	if err == nil && req.Operation == "upsert" && h.Pool != nil {
		err = h.afterIngest(r.Context(), a.TenantID)
	}
	if err != nil {
		http.Error(w, "ingest failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hashToken(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

// isNoiseSummary applies the agent-result noise contract at the ingest
// chokepoint, and also rejects summaries that duplicate their title.
func isNoiseSummary(summary, title string) (bool, string) {
	s := strings.TrimSpace(summary)
	if s == "" {
		return false, ""
	}
	t := strings.TrimSpace(title)
	if t != "" && strings.EqualFold(t, s) {
		return true, "summary duplicates title"
	}
	if agents.IsNoiseResult(s, "") {
		return true, "noise summary"
	}
	return false, ""
}

// checkedFiles validates the attachment batch and returns the cleaned batch.
// A non-zero status rejects the whole request, so a bad batch never stores
// anything. An absent batch enforces the standing 1 MiB body budget.
func checkedFiles(req Request, body []byte) ([]File, int) {
	if len(req.Files) == 0 {
		if len(body) > maxPlainBodyBytes {
			return nil, http.StatusRequestEntityTooLarge
		}
		return nil, 0
	}
	if req.Operation != "upsert" {
		return nil, http.StatusBadRequest
	}
	if req.Kind != "research_result" && req.Kind != "proactive_brief" {
		return nil, http.StatusBadRequest
	}
	if len(req.Files) > maxIngestFiles {
		return nil, http.StatusBadRequest
	}
	total := 0
	files := make([]File, 0, len(req.Files))
	for _, f := range req.Files {
		name, ok := cleanFileName(f.Name)
		if !ok {
			return nil, http.StatusBadRequest
		}
		total += len(f.Data)
		if total > maxFilesBytes {
			return nil, http.StatusRequestEntityTooLarge
		}
		files = append(files, File{Name: name, MediaType: f.MediaType, Data: f.Data})
	}
	return files, 0
}

// cleanFileName reduces a producer-supplied name to its base name and reports
// whether it is safe: 1-255 bytes, no separators or NUL, not a dotfile.
func cleanFileName(name string) (string, bool) {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || len(name) > 255 || name[0] == '.' ||
		strings.ContainsAny(name, `/\`) || strings.IndexByte(name, 0) >= 0 {
		return "", false
	}
	return name, true
}

// pctName percent-encodes a file name for the data link's name parameter.
// Every byte outside the unreserved set is encoded, so the client can recover
// the exact name without ambiguity from ';' or ',' bytes.
func pctName(name string) string {
	const hexdigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := range len(name) {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexdigits[c>>4])
			b.WriteByte(hexdigits[c&0x0F])
		}
	}
	return b.String()
}

// dataMediaType is the MIME type used in a file's data link. The declared
// type is ignored; the bytes decide, except that a text/* declaration keeps
// text/plain so a text payload can never be sniffed into a richer, executable
// media type.
func dataMediaType(declared string, data []byte) string {
	if strings.HasPrefix(strings.ToLower(declared), "text/") {
		return "text/plain"
	}
	mime := http.DetectContentType(data)
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}
	return mime
}

// renderBody builds the server-side HTML document for a card with files.
// Producer content never reaches it unescaped: the summary is escaped text
// and each file appears only as an RFC 2397 data link carrying its bytes.
func renderBody(summary string, files []File) string {
	var b strings.Builder
	b.WriteString("<pre style='white-space:pre-wrap'>")
	b.WriteString(html.EscapeString(summary))
	b.WriteString("</pre>")
	for _, f := range files {
		mime := dataMediaType(f.MediaType, f.Data)
		href := "data:" + mime + ";name=" + pctName(f.Name) + ";base64," +
			base64.StdEncoding.EncodeToString(f.Data)
		b.WriteString("<div><a href='")
		b.WriteString(href)
		b.WriteString("'>")
		b.WriteString(html.EscapeString(f.Name))
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(len(f.Data)))
		b.WriteString(" bytes)</a>")
		switch mime {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			b.WriteString("<img src='")
			b.WriteString(href)
			b.WriteString("'/>")
		}
		b.WriteString("</div>")
	}
	return b.String()
}

func (h Handler) afterIngest(ctx context.Context, tenant string) error {
	tenantID, err := uuid.Parse(tenant)
	if err != nil {
		return err
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		return err
	}
	if err := bundles.AfterIngest(ctx, tx, tenantID, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func apply(ctx context.Context, db *sql.DB, a Auth, req Request) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, a.TenantID); err != nil {
		return err
	}
	switch req.Operation {
	case "upsert":
		if req.Kind == "" || req.Title == "" {
			return errors.New("kind and title required")
		}
		sortAt := time.Now().UTC()
		if req.At != nil {
			sortAt = *req.At
		}
		if len(req.Actions) == 0 {
			req.Actions = json.RawMessage(`{}`)
		}
		var cardID string
		err = tx.QueryRowContext(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,source_kind,source_actions,title,summary,sort_at,at,timed,note_order,state) VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'open') ON CONFLICT(tenant_id,source,external_id) DO UPDATE SET source_kind=EXCLUDED.source_kind,source_actions=EXCLUDED.source_actions,title=EXCLUDED.title,summary=EXCLUDED.summary,sort_at=EXCLUDED.sort_at,at=EXCLUDED.at,timed=EXCLUDED.timed,note_order=EXCLUDED.note_order,state=CASE WHEN cards.source='ha' AND cards.state='done' THEN 'open' ELSE cards.state END RETURNING id::text`, a.TenantID, a.Source, req.ExternalID, req.Kind, []byte(req.Actions), req.Title, req.Summary, sortAt, req.At, req.Timed, req.Order).Scan(&cardID)
		if err != nil {
			return err
		}
		// The card and its body land or fail together. A batch replaces the
		// stored body; an upsert without files removes it.
		if len(req.Files) > 0 {
			_, err = tx.ExecContext(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,$3) ON CONFLICT(tenant_id,card_id) DO UPDATE SET html=EXCLUDED.html`, a.TenantID, cardID, renderBody(req.Summary, req.Files))
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM card_bodies WHERE tenant_id=$1 AND card_id=$2`, a.TenantID, cardID)
		}
	case "close":
		_, err = tx.ExecContext(ctx, `UPDATE cards SET state='done' WHERE tenant_id=$1 AND source=$2 AND external_id=$3`, a.TenantID, a.Source, req.ExternalID)
	default:
		return fmt.Errorf("unsupported operation %q", req.Operation)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
