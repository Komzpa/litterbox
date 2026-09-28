package ingest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/bundles"
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
}

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
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
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
		_, err = tx.ExecContext(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,source_kind,source_actions,title,summary,sort_at,at,timed,note_order,state) VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'open') ON CONFLICT(tenant_id,source,external_id) DO UPDATE SET source_kind=EXCLUDED.source_kind,source_actions=EXCLUDED.source_actions,title=EXCLUDED.title,summary=EXCLUDED.summary,sort_at=EXCLUDED.sort_at,at=EXCLUDED.at,timed=EXCLUDED.timed,note_order=EXCLUDED.note_order,state=CASE WHEN cards.source='ha' AND cards.state='done' THEN 'open' ELSE cards.state END`, a.TenantID, a.Source, req.ExternalID, req.Kind, []byte(req.Actions), req.Title, req.Summary, sortAt, req.At, req.Timed, req.Order)
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
