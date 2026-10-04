package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHandlerErrorLoggingAndRollback(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("set CARD_TEST_POSTGRES=1 to run PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { t.Fatal(err) }
	defer pool.Close()
	schema := "ops_logging_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil { t.Fatal(err) }
	defer pool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil { t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if _, err = db.Exec(ctx, `CREATE TABLE ops (tenant_id uuid, op_id uuid, device_id uuid, payload_hash bytea, payload jsonb, PRIMARY KEY(tenant_id,op_id))`); err != nil { t.Fatal(err) }
	const typ = "test_gmail_archive_logging"
	const gmailError = "gmail API POST /threads/thread-a/modify: 403 Forbidden: insufficientPermissions"
	Register(typ, func(_ context.Context, _ pgx.Tx, _, _ uuid.UUID, args json.RawMessage) error {
		if string(args) == `{"success":true}` { return nil }
		return errors.New(gmailError)
	})
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	tenant, card := uuid.New(), uuid.New()
	api := API{DB: db, Identity: func(context.Context) (string,string,bool) { return tenant.String(), "", true }}
	for _, success := range []bool{false, true} {
		output.Reset()
		op := uuid.New()
		body := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":%q,"args":{"success":%t}}`, op, card, typ, success)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
		var count int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, tenant, op).Scan(&count); err != nil { t.Fatal(err) }
		if success {
			if w.Code != 200 || count != 1 || output.Len() != 0 { t.Fatalf("success negative control: status=%d rows=%d log=%q", w.Code, count, output.String()) }
			continue
		}
		if w.Code != 422 || count != 0 { t.Fatalf("failed handler: status=%d rows=%d",w.Code,count) }
		if strings.Contains(w.Body.String(), gmailError) { t.Fatal("internal Gmail error leaked to client") }
		for _, want := range []string{"type="+typ,"card_id="+card.String(),"error="+gmailError} {
			if !strings.Contains(output.String(), want) { t.Errorf("log=%q missing %q", output.String(), want) }
		}
	}
}
