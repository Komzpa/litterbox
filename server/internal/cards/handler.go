package cards

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type tenantContextKey struct{}

func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenant)
}
func TenantFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tenantContextKey{}).(string)
	return t, ok
}

type Handler struct {
	DB       *sql.DB
	Location *time.Location
	NoteSink string
	Events   *Events
}

func NewHandler(db *sql.DB, zone, noteSink string) (*Handler, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", zone, err)
	}
	return &Handler{DB: db, Location: loc, NoteSink: noteSink}, nil
}
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/cards", h.List)
	mux.HandleFunc("GET /v1/cards/notes", h.NotesSince)
	if h.Events != nil {
		mux.Handle("GET /v1/cards/events", h.Events)
	}
	mux.HandleFunc("POST /v1/cards/{id}/dismiss", h.Dismiss)
	mux.HandleFunc("POST /v1/cards/{id}/note", h.SetNote)
}
func (h *Handler) tenant(r *http.Request) (string, bool) {
	t, ok := r.Context().Value(tenantContextKey{}).(string)
	return t, ok && t != ""
}

type NoteUpdate struct {
	CardID    string    `json:"card_id"`
	Title     string    `json:"title"`
	Note      string    `json:"note"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotesSince returns the latest per-card note values changed since the supplied RFC3339 timestamp.
func (h *Handler) NotesSince(w http.ResponseWriter, r *http.Request) {
	tenant, ok := h.tenant(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	since := time.Time{}
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			http.Error(w, "invalid since", 400)
			return
		}
		since = parsed
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id::text,COALESCE(NULLIF(title,''),subject),note,note_updated_at FROM cards WHERE tenant_id=$1 AND note_updated_at > $2 AND note <> '' ORDER BY note_updated_at,id`, tenant, since)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer rows.Close()
	updates := make([]NoteUpdate, 0)
	for rows.Next() {
		var item NoteUpdate
		if err := rows.Scan(&item.CardID, &item.Title, &item.Note, &item.UpdatedAt); err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		updates = append(updates, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Notes []NoteUpdate `json:"notes"`
	}{updates})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenant, ok := h.tenant(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	loc := h.Location
	if zone := r.URL.Query().Get("tz"); zone != "" {
		loaded, err := time.LoadLocation(zone)
		if err != nil {
			http.Error(w, "invalid tz", 400)
			return
		}
		loc = loaded
	}
	now := time.Now().In(loc)
	if raw := r.URL.Query().Get("now"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid now", 400)
			return
		}
		now = parsed.In(loc)
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id::text,source,COALESCE(NULLIF(title,''),subject),summary,at,timed,state,note,created_at,note_order FROM cards WHERE tenant_id=$1 AND state='open' ORDER BY created_at DESC`, tenant)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer rows.Close()
	var all []Card
	for rows.Next() {
		var c Card
		var at sql.NullTime
		if err := rows.Scan(&c.ID, &c.Source, &c.Title, &c.Summary, &at, &c.Timed, &c.State, &c.Note, &c.createdAt, &c.order); err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		if at.Valid {
			v := at.Time.In(loc)
			c.At = &v
		}
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	sections := Section(all, now)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sections)
}

type feedback struct {
	Note string `json:"note"`
	Text string `json:"text"`
}

func (h *Handler) Dismiss(w http.ResponseWriter, r *http.Request) {
	var body feedback
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			http.Error(w, "invalid JSON", 400)
			return
		}
	}
	h.mutate(w, r, body.Note, true)
}
func (h *Handler) SetNote(w http.ResponseWriter, r *http.Request) {
	var body feedback
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	h.mutate(w, r, body.Text, false)
}
func (h *Handler) mutate(w http.ResponseWriter, r *http.Request, note string, dismiss bool) {
	tenant, ok := h.tenant(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	var title string
	query := `UPDATE cards SET note=$1,note_updated_at=now() WHERE tenant_id=$2 AND id::text=$3 RETURNING COALESCE(NULLIF(title,''),subject)`
	if dismiss {
		query = `UPDATE cards SET note=$1,note_updated_at=now(),state='done' WHERE tenant_id=$2 AND id::text=$3 RETURNING COALESCE(NULLIF(title,''),subject)`
	}
	if err = tx.QueryRowContext(r.Context(), query, note, tenant, r.PathValue("id")).Scan(&title); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if strings.TrimSpace(note) != "" && h.NoteSink != "" {
		if err := appendGeneratorFeedback(h.NoteSink, title, note); err != nil {
			http.Error(w, "note sink unavailable", 500)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// The daily-note generator reads invalid/do-not-repeat checkbox tails verbatim
// from yesterday's Markdown note, so this sink writes that accepted format.
func appendGeneratorFeedback(path, title, note string) error {
	title = strings.ReplaceAll(strings.TrimSpace(title), "\n", " ")
	note = strings.ReplaceAll(strings.TrimSpace(note), "\n", " ")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- [ ] %s — invalid: %s\n", title, note)
	return err
}
