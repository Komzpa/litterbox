package ops

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Use a private database, not a rewritten schema: this exercises the shipped
// migrations and literal public/litterbox_app privileges without touching any
// product database. These tests must stay serial because roles are cluster-wide.
func journalMigrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	port := os.Getenv("PGPORT")
	if os.Getenv("DATABASE_URL") != "" || os.Getenv("PGHOST") != "localhost" || port == "" || port == "5432" || port == "5435" || port == "5436" || os.Getenv("PGDATABASE") != "postgres" {
		t.Fatal("requires a throwaway pg_virtualenv cluster and an unset DATABASE_URL")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(ctx) })
	var existing bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_roles WHERE rolname='litterbox_app')`).Scan(&existing); err != nil {
		t.Fatal(err)
	}
	if existing {
		t.Fatal("throwaway cluster already has litterbox_app; refusing to alter it")
	}
	name := "journal_grants_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatal(err)
	}
	roleCreated := false
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP DATABASE `+name); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		if roleCreated {
			if _, err := admin.Exec(ctx, `DROP ROLE litterbox_app`); err != nil {
				t.Errorf("drop test role: %v", err)
			}
		}
	})
	cfg, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("discover migrations: files=%v error=%v", files, err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
		if filepath.Base(file) == "002_security.sql" {
			roleCreated = true
		}
	}
	return db
}

func TestAllPublicTablesGrantAppInsert(t *testing.T) {
	db := journalMigrationDatabase(t)
	rows, err := db.Query(context.Background(), `SELECT tablename, has_table_privilege('litterbox_app', format('%I.%I', schemaname, tablename), 'INSERT') FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var table string
		var granted bool
		if err := rows.Scan(&table, &granted); err != nil {
			t.Fatal(err)
		}
		count++
		if !granted {
			t.Errorf("public.%s: litterbox_app lacks INSERT", table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no public tables checked")
	}
	t.Logf("checked INSERT privilege on all %d public tables", count)
}

func TestJournalCreateAsAppRole(t *testing.T) {
	admin := journalMigrationDatabase(t)
	ctx := context.Background()
	tenant, device, card := uuid.New(), uuid.New(), uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, tenant, device, []byte("test-device")); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE litterbox_app`)
		return err
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var role string
	var bypass, super bool
	if err := db.QueryRow(ctx, `SELECT current_user, rolbypassrls, rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&role, &bypass, &super); err != nil {
		t.Fatal(err)
	}
	if role != "litterbox_app" || bypass || super {
		t.Fatalf("runtime identity=%q bypassrls=%v superuser=%v", role, bypass, super)
	}
	api := API{DB: db, Identity: func(context.Context) (string, string, bool) { return tenant.String(), device.String(), true }}
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		payload := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":"create_card","args":{"kind":"journal","title":"Private thought","body":%q}}`, uuid.New(), card, body)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(payload)))
		return response
	}
	if response := post("A private thought\nSaved from the inbox."); response.Code != 200 {
		t.Fatalf("POST /v1/ops as %s: status=%d body=%s, want 200", role, response.Code, response.Body.String())
	}
	var body string
	if err := admin.QueryRow(ctx, `SELECT j.body FROM journal_entries j JOIN cards c ON c.id=j.id AND c.tenant_id=j.tenant_id WHERE j.id=$1 AND j.tenant_id=$2 AND c.source='journal' AND c.state='open'`, card, tenant).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if body != "A private thought\nSaved from the inbox." {
		t.Fatalf("persisted journal body=%q", body)
	}
	if response := post("   "); response.Code != 422 || !strings.Contains(response.Body.String(), "journal body required") {
		t.Fatalf("blank-body negative control: status=%d body=%s, want 422 journal body required", response.Code, response.Body.String())
	}
	t.Log("POST /v1/ops as non-owner litterbox_app: 200; card and journal body persisted; blank-body negative control: 422")
	deleteOp := uuid.New()
	payload := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":"delete_journal","args":{}}`, deleteOp, card)
	for range 2 {
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(payload)))
		if response.Code != 200 {
			t.Fatalf("delete/replay as %s: status=%d body=%s", role, response.Code, response.Body.String())
		}
	}
	for _, table := range []string{"cards", "journal_entries"} {
		var count int
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s after app-role delete: count=%d error=%v", table, count, err)
		}
	}
}
