package reminders

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/google/uuid"
)

type Handler struct { DB *sql.DB }

func (h *Handler) Routes(mux *http.ServeMux) { mux.HandleFunc("POST /v1/reminders", h.Create) }

// CreateInput is shared by POST /v1/reminders and the create_reminder operation.
type CreateInput struct {
	Title string `json:"title"`
	DueAt time.Time `json:"due_at"`
	Recurrence string `json:"recurrence"`
}

func (in CreateInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" || in.DueAt.IsZero() || (in.Recurrence != "" && in.Recurrence != "daily" && in.Recurrence != "weekly") {
		return fmt.Errorf("title, due_at and optional daily/weekly recurrence required")
	}
	return nil
}

// CreateInTx inserts once per tenant and reminder ID, without changing an
// existing reminder. The caller sets tenant RLS and commits its transaction.
// exec bridges database/sql (the legacy route) and pgx (durable operations).
func CreateInTx(ctx context.Context, exec func(context.Context, string, ...any) error, tenant, id string, in CreateInput) error {
	if err := in.Validate(); err != nil { return err }
	return exec(ctx, `INSERT INTO reminders(tenant_id,id,title,due_at,recurrence) VALUES($1,$2,$3,$4,$5) ON CONFLICT (tenant_id,id) DO NOTHING`, tenant, id, strings.TrimSpace(in.Title), in.DueAt.UTC(), in.Recurrence)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenant, ok := cards.TenantFrom(r.Context())
	if !ok || tenant == "" { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	var in CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Validate() != nil {
		http.Error(w, "title, due_at and optional daily/weekly recurrence required", http.StatusBadRequest); return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil { http.Error(w, "internal server error", 500); return }
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`,tenant); err != nil { http.Error(w, "internal server error",500); return }
	id := uuid.NewString()
	err = CreateInTx(r.Context(), func(ctx context.Context, query string, args ...any) error {
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	}, tenant, id, in)
	if err != nil { http.Error(w, "internal server error", http.StatusInternalServerError); return }
	if err=tx.Commit(); err != nil { http.Error(w,"internal server error",500); return }
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// Tick emits due reminders and closes expired meetings for one tenant. Both
// mutations update cards, so migration 005 notifies existing SSE subscribers.
func Tick(ctx context.Context, db *sql.DB, tenant string, now time.Time) error {
	tx,err:=db.BeginTx(ctx,nil); if err!=nil{return err}; defer tx.Rollback()
	if _,err=tx.ExecContext(ctx,`SELECT set_config('litterbox.tenant_id',$1,true)`,tenant); err!=nil{return err}
	_,err=tx.ExecContext(ctx,`WITH due AS (SELECT tenant_id,id,title,due_at FROM reminders WHERE state='scheduled' AND due_at <= $1 ORDER BY due_at FOR UPDATE SKIP LOCKED), made AS (INSERT INTO cards(tenant_id,id,source,external_id,title,summary,at,timed,state,sort_at) SELECT tenant_id,gen_random_uuid(),'reminder',id::text,title,'',due_at,true,'open',due_at FROM due RETURNING tenant_id,external_id,id) UPDATE reminders r SET state='emitted',card_id=made.id FROM made WHERE r.tenant_id=made.tenant_id AND r.id::text=made.external_id`,now.UTC())
	if err!=nil{return err}
	_,err=tx.ExecContext(ctx,`UPDATE cards SET state='done' WHERE tenant_id=$1 AND source='meeting' AND state='open' AND at <= $2`,tenant,now.UTC()); if err!=nil{return err}
	return tx.Commit()
}

func Run(ctx context.Context, db *sql.DB, tenant string, every time.Duration) error {
	if every<=0{return fmt.Errorf("scheduler interval must be positive")}
	ticker:=time.NewTicker(every); defer ticker.Stop()
	for { if err:=Tick(ctx,db,tenant,time.Now()); err!=nil{return err}; select {case <-ctx.Done():return ctx.Err(); case <-ticker.C:} }
}
