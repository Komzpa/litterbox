package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Run the actual server without dev authentication against the app role.
func TestProductionWorkers(t *testing.T) {
	binary := os.Getenv("LITTERBOX_TEST_BINARY")
	if binary == "" || os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("requires LITTERBOX_TEST_BINARY and pg_virtualenv")
	}
	fixture := testdb.Setup(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, fixture.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, fixture.Migration(string(b))); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	tenant, id := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO reminders(tenant_id,id,title,due_at) VALUES($1,$2,'Production reminder',now()-interval '1 minute')`, tenant, id); err != nil {
		t.Fatal(err)
	}
	wakeID := uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,state,snooze_until) VALUES($1,$2,'manual','wake-fixture','snoozed',now()-interval '1 minute')`, tenant, wakeID); err != nil {
		t.Fatal(err)
	}
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "DATABASE_URL=") && !strings.HasPrefix(entry, "LITTERBOX_DEV_TENANT_ID=") && !strings.HasPrefix(entry, "GMAIL_OAUTH_CREDENTIALS=") {
			env = append(env, entry)
		}
	}
	// The connection starts as the restricted application role on every pool.
	appDSN := fixture.DSN + " options='-c role=" + fixture.Role + "'"
	app, err := pgx.Connect(ctx, appDSN)
	if err != nil {
		t.Fatal(err)
	}
	var role string
	var bypass bool
	if err = app.QueryRow(ctx, `SELECT current_user, rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&role, &bypass); err != nil || role != fixture.Role || bypass {
		t.Fatalf("role=%s bypass=%v err=%v", role, bypass, err)
	}
	app.Close(ctx)
	env = append(env, "DATABASE_URL="+appDSN)
	cmd := exec.Command(binary, "-listen", "127.0.0.1:0")
	cmd.Env = env
	var logs strings.Builder
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(35 * time.Second)
	var count int
	var state string
	for time.Now().Before(deadline) {
		if err = db.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND source='reminder' AND state='open'`, tenant).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if err = db.QueryRow(ctx, `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, wakeID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if count == 1 && state == "open" {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	t.Fatalf("production worker: reminder cards=%d, snooze state=%s; %s", count, state, logs.String())
}
