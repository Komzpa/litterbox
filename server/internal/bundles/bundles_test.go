package bundles

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestSenderKey(t *testing.T) {
	if got := SenderKey(`Jane Doe <Jane@Example.COM>`); got != "jane@example.com" {
		t.Fatalf("SenderKey()=%q", got)
	}
}

func TestPostgresBundleActions(t *testing.T) {
	if os.Getenv("BUNDLE_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with BUNDLE_TEST_POSTGRES=1")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, path := range []string{"../../db/001_mail.sql", "../../db/002_security.sql", "../../db/003_agent_cards.sql", "../../db/008_bundles.sql"} {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	tenant, account := uuid.New(), uuid.New()
	if _, err = conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'a@example.com','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	addCard := func() uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,'jane@example.com','hello')`, tenant, id, account, id.String())
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	c1, c2, c3, c4, c5 := addCard(), addCard(), addCard(), addCard(), addCard()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = Assign(ctx, tx, tenant, c1, "Jane <jane@example.com>", "example.com", "", "important"); err != nil {
		t.Fatal(err)
	}
	var b1 uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, c1).Scan(&b1); err != nil {
		t.Fatal(err)
	}
	if err = TakeOut(ctx, tx, tenant, c1); err != nil {
		t.Fatal(err)
	}
	if err = Assign(ctx, tx, tenant, c2, "jane@example.com", "example.com", "", "normal"); err != nil {
		t.Fatal(err)
	}
	var b2 *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, c2).Scan(&b2); err != nil {
		t.Fatal(err)
	}
	if b2 != nil {
		t.Fatal("take-out did not teach future sender assignment")
	}
	if err = Pin(ctx, tx, tenant, c2); err != nil {
		t.Fatal(err)
	}
	if err = Unpin(ctx, tx, tenant, c2); err != nil {
		t.Fatal(err)
	}
	if err = Pin(ctx, tx, tenant, c4); err != nil {
		t.Fatal(err)
	}
	if err = Assign(ctx, tx, tenant, c4, "other@example.net", "other.net", "", "normal"); err != nil {
		t.Fatal(err)
	}
	var b4 uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, c4).Scan(&b4); err != nil {
		t.Fatal(err)
	}
	if err = ReorderPins(ctx, tx, tenant, []uuid.UUID{c4}); err != nil {
		t.Fatal(err)
	}
	if err = Assign(ctx, tx, tenant, c5, "other2@example.net", "other.net", "", "normal"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, tenant, c5, b4); err != nil {
		t.Fatal(err)
	}
	until := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	if err = Snooze(ctx, tx, tenant, c3, json.RawMessage(`{"until":"`+until+`"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, tenant, c3, b4); err != nil {
		t.Fatal(err)
	}
	if err = ArchiveBundle(ctx, tx, tenant, b4); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, c4).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "open" {
		t.Fatalf("archive changed pinned card state to %q", state)
	}
	if err = tx.QueryRow(ctx, `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, c5).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "archived" {
		t.Fatalf("archive left unpinned card state %q", state)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
