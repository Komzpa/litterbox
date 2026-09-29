package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOperationReplayIsIdempotent(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("set CARD_TEST_POSTGRES=1 to run PostgreSQL integration test")
	}
	dsn := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "ops_test_" + uuid.NewString()[:8]
	if _, err = pool.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	testPool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer testPool.Close()
	for _, ddl := range []string{
		`CREATE TABLE cards (tenant_id uuid NOT NULL, id uuid NOT NULL, state text NOT NULL DEFAULT 'open', note text NOT NULL DEFAULT '', applied integer NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,id))`,
		`CREATE TABLE ops (tenant_id uuid NOT NULL, op_id uuid NOT NULL, device_id uuid, payload_hash bytea NOT NULL, payload jsonb NOT NULL, PRIMARY KEY(tenant_id,op_id))`,
	} {
		if _, err = testPool.Exec(context.Background(), ddl); err != nil {
			t.Fatal(err)
		}
	}
	tenant, cardID, opID := uuid.New(), uuid.New(), uuid.New()
	if _, err = testPool.Exec(context.Background(), `INSERT INTO cards(tenant_id,id) VALUES($1,$2)`, tenant, cardID); err != nil {
		t.Fatal(err)
	}
	Register("test_count", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		_, err := tx.Exec(ctx, `UPDATE cards SET applied=applied+1 WHERE tenant_id=$1 AND id=$2`, tenant, card)
		return err
	})
	api := API{DB: testPool, Identity: func(context.Context) (string, string, bool) { return tenant.String(), "", true }}
	body := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":"test_count","args":{}}`, opID, cardID)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body))
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("replay %d: status %d: %s", i, w.Code, w.Body.String())
		}
	}
	var applied, opCount int
	if err = testPool.QueryRow(context.Background(), `SELECT applied FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, cardID).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if err = testPool.QueryRow(context.Background(), `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, tenant, opID).Scan(&opCount); err != nil {
		t.Fatal(err)
	}
	if applied != 1 || opCount != 1 {
		t.Fatalf("replay applied=%d stored_ops=%d, want 1 each", applied, opCount)
	}
}

func TestManualCreateAndReorderOperationsAreTenantScoped(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("set CARD_TEST_POSTGRES=1 to run PostgreSQL integration test")
	}
	dsn := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "ops_manual_test_" + uuid.NewString()[:8]
	if _, err = pool.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, ddl := range []string{
		`CREATE TABLE cards (tenant_id uuid NOT NULL, id uuid NOT NULL, account_id uuid, gmail_thread_id text, source text NOT NULL, external_id text, title text NOT NULL DEFAULT '', summary text NOT NULL DEFAULT '', state text NOT NULL DEFAULT 'open', note_order integer NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,source,external_id))`,
		`CREATE TABLE ops (tenant_id uuid NOT NULL, op_id uuid NOT NULL, device_id uuid, payload_hash bytea NOT NULL, payload jsonb NOT NULL, PRIMARY KEY(tenant_id,op_id))`,
	} {
		if _, err := db.Exec(context.Background(), ddl); err != nil {
			t.Fatal(err)
		}
	}
	tenant, otherTenant := uuid.New(), uuid.New()
	first, second, closed, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		tenant uuid.UUID
		id     uuid.UUID
		state  string
	}{
		{tenant, first, "open"}, {tenant, second, "open"}, {tenant, closed, "done"}, {otherTenant, foreign, "open"},
	} {
		if _, err := db.Exec(context.Background(), `INSERT INTO cards(tenant_id,id,source,external_id,state,note_order) VALUES($1,$2,'todo',$3,$4,9)`, row.tenant, row.id, row.id.String(), row.state); err != nil {
			t.Fatal(err)
		}
	}
	api := API{DB: db, Identity: func(context.Context) (string, string, bool) { return tenant.String(), "", true }}
	post := func(typ string, card uuid.UUID, args string) int {
		t.Helper()
		body := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":%q,"args":%s}`, uuid.NewString(), card, typ, args)
		req := httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body))
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response.Code
	}
	created := uuid.New()
	if status := post("create_card", created, `{"title":"  Manual task ","summary":"Details"}`); status != 200 {
		t.Fatalf("create status=%d, want 200", status)
	}
	if status := post("create_card", uuid.New(), `{"title":"   "}`); status != 422 {
		t.Fatalf("blank-title status=%d, want 422", status)
	}
	if status := post("reorder_cards", first, fmt.Sprintf(`{"cards":[%q,%q,%q]}`, second, created, first)); status != 200 {
		t.Fatalf("complete reorder status=%d, want 200", status)
	}
	for _, bad := range []struct {
		name string
		ids  string
	}{
		{"partial open set", fmt.Sprintf(`{"cards":[%q,%q]}`, first, second)},
		{"closed card", fmt.Sprintf(`{"cards":[%q,%q,%q]}`, first, second, closed)},
		{"foreign tenant card", fmt.Sprintf(`{"cards":[%q,%q,%q]}`, first, second, foreign)},
	} {
		if status := post("reorder_cards", first, bad.ids); status != 422 {
			t.Errorf("%s status=%d, want 422", bad.name, status)
		}
	}
	var ordered string
	if err := db.QueryRow(context.Background(), `SELECT string_agg(id::text,',' ORDER BY note_order) FROM cards WHERE tenant_id=$1 AND state='open'`, tenant).Scan(&ordered); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%s,%s,%s", second, created, first)
	if ordered != want {
		t.Fatalf("tenant order=%s, want %s", ordered, want)
	}
	var foreignOrder int
	if err := db.QueryRow(context.Background(), `SELECT note_order FROM cards WHERE tenant_id=$1 AND id=$2`, otherTenant, foreign).Scan(&foreignOrder); err != nil {
		t.Fatal(err)
	}
	if foreignOrder != 9 {
		t.Fatalf("foreign tenant order=%d, want unchanged 9", foreignOrder)
	}
}
