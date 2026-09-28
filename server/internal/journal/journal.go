package journal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CardIngest interface {
	UpsertCard(ctx context.Context, tenant, externalID, title, summary string) error
}

type Store struct {
	DB    *pgxpool.Pool
	Cards CardIngest
}

type SQLCardIngest struct{ DB *pgxpool.Pool }

func (s SQLCardIngest) UpsertCard(ctx context.Context, tenant, externalID, title, summary string) error {
	tx, err := (&Store{DB: s.DB}).tenantTx(ctx, tenant)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,external_id,title,summary,state) VALUES($1,gen_random_uuid(),NULL,NULL,'mcp',$2,$3,$4,'open') ON CONFLICT (tenant_id,source,external_id) DO UPDATE SET title=EXCLUDED.title,summary=EXCLUDED.summary`, tenant, externalID, title, summary)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Entry struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type tenantKey struct{}

func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}
func (s Store) tenantTx(ctx context.Context, tenant string) (pgx.Tx, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}
func (s Store) Create(ctx context.Context, tenant, body string) (Entry, error) {
	tx, err := s.tenantTx(ctx, tenant)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback(ctx)
	var e Entry
	err = tx.QueryRow(ctx, `INSERT INTO journal_entries(tenant_id,body) VALUES($1,$2) RETURNING id::text,body,created_at,updated_at`, tenant, body).Scan(&e.ID, &e.Body, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return e, err
	}
	err = tx.Commit(ctx)
	return e, err
}
func (s Store) List(ctx context.Context, tenant string) ([]Entry, error) {
	tx, err := s.tenantTx(ctx, tenant)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id::text,body,created_at,updated_at FROM journal_entries WHERE tenant_id=$1 ORDER BY created_at DESC`, tenant)
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err = rows.Scan(&e.ID, &e.Body, &e.CreatedAt, &e.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return out, err
}
func (s Store) Update(ctx context.Context, tenant, id, body string) (Entry, error) {
	tx, err := s.tenantTx(ctx, tenant)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback(ctx)
	var e Entry
	err = tx.QueryRow(ctx, `UPDATE journal_entries SET body=$3,updated_at=now() WHERE tenant_id=$1 AND id=$2 RETURNING id::text,body,created_at,updated_at`, tenant, id, body).Scan(&e.ID, &e.Body, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return e, err
	}
	err = tx.Commit(ctx)
	return e, err
}
func (s Store) Delete(ctx context.Context, tenant, id string) error {
	tx, err := s.tenantTx(ctx, tenant)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM journal_entries WHERE tenant_id=$1 AND id=$2`, tenant, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s Store) CreateToken(ctx context.Context, tenant string) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := "lbm_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	var id string
	err := s.DB.QueryRow(ctx, `INSERT INTO mcp_tokens(tenant_id,token_hash,scopes) VALUES($1,$2,$3) RETURNING id::text`, tenant, sum[:], []string{"journal:read", "cards:create"}).Scan(&id)
	return id, token, err
}
func (s Store) RevokeToken(ctx context.Context, tenant, id string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE mcp_tokens SET revoked_at=now() WHERE tenant_id=$1 AND id=$2 AND revoked_at IS NULL`, tenant, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

type authToken struct {
	Tenant string
	Scopes map[string]bool
}

func (s Store) authenticate(ctx context.Context, raw string) (authToken, error) {
	sum := sha256.Sum256([]byte(raw))
	var a authToken
	var scopes []string
	err := s.DB.QueryRow(ctx, `SELECT tenant_id::text,scopes FROM mcp_tokens WHERE token_hash=$1 AND revoked_at IS NULL`, sum[:]).Scan(&a.Tenant, &scopes)
	if err != nil {
		return a, err
	}
	a.Scopes = map[string]bool{}
	for _, x := range scopes {
		a.Scopes[x] = true
	}
	return a, nil
}
func (s Store) Register(mux *http.ServeMux) { mux.Handle("/mcp", s.mcpHandler()) }
func (s Store) mcpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		a, err := s.authenticate(r.Context(), raw)
		if raw == "" || err != nil {
			http.Error(w, "unauthorized", 401)
			return
		}
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  struct {
				Name      string                     `json:"name"`
				Arguments map[string]json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.JSONRPC != "2.0" {
			reply(w, req.ID, nil, errors.New("invalid request"))
			return
		}
		if req.Method == "tools/list" {
			reply(w, req.ID, map[string]any{"tools": []any{map[string]any{"name": "journal_read"}, map[string]any{"name": "card_create"}}}, nil)
			return
		}
		if req.Method != "tools/call" {
			reply(w, req.ID, nil, errors.New("unknown method"))
			return
		}
		var result any
		var callErr error
		ctx := r.Context()
		switch req.Params.Name {
		case "journal_read":
			if !a.Scopes["journal:read"] {
				callErr = errors.New("scope denied")
				break
			}
			result, callErr = s.List(ctx, a.Tenant)
		case "card_create":
			if !a.Scopes["cards:create"] {
				callErr = errors.New("scope denied")
				break
			}
			var in struct {
				ExternalID string `json:"external_id"`
				Title      string `json:"title"`
				Summary    string `json:"summary"`
			}
			callErr = json.Unmarshal(mustArgs(req.Params.Arguments), &in)
			if callErr == nil {
				if in.ExternalID == "" {
					callErr = errors.New("external_id is required")
				} else if s.Cards == nil {
					callErr = errors.New("card ingest unavailable")
				} else {
					callErr = s.Cards.UpsertCard(ctx, a.Tenant, in.ExternalID, in.Title, in.Summary)
				}
			}
		default:
			callErr = errors.New("unknown tool")
		}
		reply(w, req.ID, result, callErr)
	})
}
func mustArgs(a map[string]json.RawMessage) json.RawMessage { b, _ := json.Marshal(a); return b }
func reply(w http.ResponseWriter, id json.RawMessage, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{"jsonrpc": "2.0", "id": id}
	if err != nil {
		out["error"] = map[string]any{"code": -32000, "message": err.Error()}
	} else {
		out["result"] = result
	}
	_ = json.NewEncoder(w).Encode(out)
}
func (s Store) RegisterPrivate(mux *http.ServeMux) {
	mux.Handle("/v1/journal", s.privateHandler(false))
	mux.Handle("/v1/journal/", s.privateHandler(true))
	mux.HandleFunc("POST /v1/mcp-tokens", func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := r.Context().Value(tenantKey{}).(string)
		if !ok || tenant == "" {
			http.Error(w, "unauthorized", 401)
			return
		}
		id, token, err := s.CreateToken(r.Context(), tenant)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "token": token})
	})
	mux.HandleFunc("DELETE /v1/mcp-tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := r.Context().Value(tenantKey{}).(string)
		if !ok || tenant == "" {
			http.Error(w, "unauthorized", 401)
			return
		}
		if err := s.RevokeToken(r.Context(), tenant, r.PathValue("id")); err != nil {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
func (s Store) privateHandler(item bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := r.Context().Value(tenantKey{}).(string)
		if !ok || tenant == "" {
			http.Error(w, "unauthorized", 401)
			return
		}
		if item {
			id := strings.TrimPrefix(r.URL.Path, "/v1/journal/")
			switch r.Method {
			case "PUT":
				var v struct {
					Body string `json:"body"`
				}
				if json.NewDecoder(r.Body).Decode(&v) != nil {
					http.Error(w, "bad request", 400)
					return
				}
				e, err := s.Update(r.Context(), tenant, id, v.Body)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(e)
			case "DELETE":
				if s.Delete(r.Context(), tenant, id) != nil {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "method not allowed", 405)
			}
			return
		}
		switch r.Method {
		case "GET":
			v, err := s.List(r.Context(), tenant)
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			_ = json.NewEncoder(w).Encode(v)
		case "POST":
			var v struct {
				Body string `json:"body"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				http.Error(w, "bad request", 400)
				return
			}
			e, err := s.Create(r.Context(), tenant, v.Body)
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(e)
		default:
			http.Error(w, "method not allowed", 405)
		}
	})
}
