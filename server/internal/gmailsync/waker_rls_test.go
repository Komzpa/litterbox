package gmailsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWakerAppRoleRestoresGmailAndIsolatesTenants(t *testing.T) {
	db, fixture := testPool(t)
	ctx := context.Background()
	migration, err := os.ReadFile("../../db/027_worker_tenants.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, fixture.Migration(string(migration))); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, fixture.Migration(string(migration))); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	tenants := []uuid.UUID{uuid.New(), uuid.New()}
	cards := []uuid.UUID{uuid.New(), uuid.New()}
	for i, tenant := range tenants {
		account := uuid.New()
		if _, err = db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,$3,'x')`, tenant, account, tenant.String()+"@example.test"); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,state,snooze_until) VALUES($1,$2,$3,$4,'mail','snoozed',now()-interval '1 minute')`, tenant, cards[i], account, cardThread(i)); err != nil {
			t.Fatal(err)
		}
	}
	var calls []struct {
		Add    []string `json:"addLabelIds"`
		Remove []string `json:"removeLabelIds"`
	}
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "expires_in": 3600})
		case strings.HasSuffix(r.URL.Path, "/labels"):
			json.NewEncoder(w).Encode(map[string]any{"labels": []any{map[string]string{"name": "Litterbox/Snoozed", "id": "snoozed"}}})
		case strings.HasSuffix(r.URL.Path, "/modify"):
			var call struct {
				Add    []string `json:"addLabelIds"`
				Remove []string `json:"removeLabelIds"`
			}
			if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
				t.Error(err)
			}
			calls = append(calls, call)
			if fail {
				http.Error(w, "refused", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	useTestOps(func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID) (*Client, string, error) {
		var thread string
		err := tx.QueryRow(ctx, `SELECT gmail_thread_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&thread)
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "x", ClientID: "id", ClientSecret: "secret"}, thread, err
	})
	cfg, err := pgxpool.ParseConfig(fixture.DSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET ROLE "+fixture.Role)
		return err
	}
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var bypass bool
	if err = app.QueryRow(ctx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&bypass); err != nil || bypass {
		t.Fatalf("role bypass=%v err=%v", bypass, err)
	}
	// Exercise the same registered operation as the user's snooze request.
	if _, err = db.Exec(ctx, `UPDATE cards SET state='open',snooze_until=NULL WHERE tenant_id=$1 AND id=$2`, tenants[0], cards[0]); err != nil {
		t.Fatal(err)
	}
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenants[0].String()); err != nil {
		t.Fatal(err)
	}
	handler, ok := ops.Lookup("snooze")
	if !ok {
		t.Fatal("snooze handler missing")
	}
	args, _ := json.Marshal(map[string]string{"until": time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)})
	if err = handler(ctx, tx, tenants[0], cards[0], args); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || len(calls[0].Add) != 1 || calls[0].Add[0] != "snoozed" || len(calls[0].Remove) != 1 || calls[0].Remove[0] != "INBOX" {
		t.Fatalf("snooze Gmail calls: %+v", calls)
	}
	if _, err = db.Exec(ctx, `UPDATE cards SET snooze_until=now()-interval '1 minute' WHERE tenant_id=$1 AND id=$2`, tenants[0], cards[0]); err != nil {
		t.Fatal(err)
	}
	calls = nil
	api := ops.API{DB: app}
	fail = true
	if err = api.WakeTenant(ctx, tenants[0]); err == nil {
		t.Fatal("provider failure accepted")
	}
	fail = false
	state := func(i int) string {
		var s string
		if err := db.QueryRow(ctx, `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, tenants[i], cards[i]).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if state(0) != "snoozed" || state(1) != "snoozed" {
		t.Fatal("failure or foreign tenant changed card")
	}
	if err = api.WakeTenant(ctx, tenants[0]); err != nil {
		t.Fatal(err)
	}
	if state(0) != "open" || state(1) != "snoozed" {
		t.Fatal("wake or tenant isolation failed")
	}
	if len(calls) != 2 || len(calls[1].Add) != 1 || calls[1].Add[0] != "INBOX" || len(calls[1].Remove) != 1 || calls[1].Remove[0] != "snoozed" {
		t.Fatalf("wake Gmail calls: %+v", calls)
	}
	// Exercise the production loop's tenant discovery as the restricted role.
	workerCtx, cancel := context.WithCancel(ctx)
	ops.StartWaker(workerCtx, app)
	deadline := time.Now().Add(5 * time.Second)
	for state(1) != "open" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if state(1) != "open" {
		t.Fatal("worker did not discover second tenant")
	}
}
func cardThread(i int) string {
	if i == 0 {
		return "thread-a"
	}
	return "thread-b"
}
