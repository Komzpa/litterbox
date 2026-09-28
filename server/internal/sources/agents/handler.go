package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

type tenantContextKey struct{}

// WithTenant attaches the authenticated tenant UUID for the cards handler.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

func TenantFrom(ctx context.Context) (string, bool) {
	tenant, ok := ctx.Value(tenantContextKey{}).(string)
	return tenant, ok
}

// CardsHandler serves cards for an authenticated tenant. Authentication
// middleware must attach the tenant with WithTenant before this handler runs.
func CardsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := r.Context().Value(tenantContextKey{}).(string)
		if !ok || tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenantID); err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		rows, err := tx.QueryContext(r.Context(), `SELECT id,source,COALESCE(external_id,gmail_thread_id),COALESCE(NULLIF(title,''),subject),summary,sort_at,state FROM cards WHERE tenant_id=$1 ORDER BY sort_at DESC`, tenantID)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		defer rows.Close()
		cards := make([]Card, 0)
		for rows.Next() {
			var c Card
			if err := rows.Scan(&c.ID, &c.Source, &c.ExternalID, &c.Title, &c.Summary, &c.SortAt, &c.State); err != nil {
				http.Error(w, "internal server error", 500)
				return
			}
			cards = append(cards, c)
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
		_ = json.NewEncoder(w).Encode(map[string][]Card{"cards": cards})
	}
}

// DismissHandler closes the tenant-owned card selected by its UUID. It resolves
// the external ID and delegates lifecycle handling to Dismiss.
func DismissHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := r.Context().Value(tenantContextKey{}).(string)
		if !ok || tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenantID); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		var externalID string
		err = tx.QueryRowContext(r.Context(), `SELECT external_id FROM cards WHERE id::text=$1 AND tenant_id=$2 AND source IN ('agent','todo')`, r.PathValue("id"), tenantID).Scan(&externalID)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if err = Dismiss(r.Context(), db, tenantID, externalID); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
