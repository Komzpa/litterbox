package reminders

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var tenantCounter atomic.Uint64

func reminderDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("set CARD_TEST_POSTGRES=1 under pg_virtualenv")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, path := range []string{"../../db/001_mail.sql", "../../db/002_security.sql", "../../db/003_agent_cards.sql", "../../db/004_card_time_note.sql", "../../db/005_card_notify.sql", "../../db/013_reminders.sql"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(b))); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	tenant := fmt.Sprintf("a0000000-0000-4000-8000-%012d", tenantCounter.Add(1))
	if _, err := db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	return db, tenant
}

func TestDueReminderAppearsAndRecurringDoneSchedulesNext(t *testing.T) {
	db, tenant := reminderDB(t)
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	ctx := cards.WithTenant(context.Background(), tenant)
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Daily check","due_at":"2026-09-28T08:00:00Z","recurrence":"daily"}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if err := Tick(context.Background(), db, tenant, now); err != nil {
		t.Fatal(err)
	}
	var cardID string
	if err := db.QueryRow(`SELECT id::text FROM cards WHERE tenant_id=$1 AND source='reminder'`, tenant).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	if cardID == "" {
		t.Fatal("due reminder produced no card")
	}
	if _, err := db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE cards SET state='done' WHERE tenant_id=$1 AND id=$2`, tenant, cardID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM reminders WHERE tenant_id=$1 AND state='scheduled' AND due_at='2026-09-29T08:00:00Z'`, tenant).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("next occurrence count=%d, want 1", count)
	}
}

func TestWeeklyReminderDoneSchedulesNextWeek(t *testing.T) {
	db, tenant := reminderDB(t)
	due := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	ctx := cards.WithTenant(context.Background(), tenant)
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Weekly review","due_at":"2026-09-28T08:00:00Z","recurrence":"weekly"}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if err := Tick(context.Background(), db, tenant, due); err != nil {
		t.Fatal(err)
	}
	var cardID string
	if err := db.QueryRow(`SELECT id::text FROM cards WHERE tenant_id=$1 AND source='reminder'`, tenant).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE cards SET state='done' WHERE tenant_id=$1 AND id=$2`, tenant, cardID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM reminders WHERE tenant_id=$1 AND state='scheduled' AND due_at='2026-10-05T08:00:00Z'`, tenant).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("next weekly occurrence count=%d, want 1", count)
	}
}

func TestMeetingClosesAtEnd(t *testing.T) {
	db, tenant := reminderDB(t)
	end := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	if err := UpsertMeeting(context.Background(), db, tenant, "event-1", "Planning", end); err != nil {
		t.Fatal(err)
	}
	if err := Tick(context.Background(), db, tenant, end); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM cards WHERE tenant_id=$1 AND source='meeting' AND external_id='event-1'`, tenant).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "done" {
		t.Fatalf("meeting state=%q, want done", state)
	}
}

func TestTickAllDiscoversTenantsAsAppRole(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("CARD_TEST_POSTGRES=1 required")
	}
	fixture := testdb.Setup(t)
	db, err := sql.Open("pgx", fixture.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "013_reminders.sql", "027_worker_tenants.sql"} {
		b, err := os.ReadFile("../../db/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(fixture.Migration(string(b))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "a0000000-0000-4000-8000-000000000001"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE ` + fixture.Role); err != nil {
		t.Fatal(err)
	}
	var bypass bool
	if err = db.QueryRow(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&bypass); err != nil || bypass {
		t.Fatalf("role bypass=%v err=%v", bypass, err)
	}
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Due reminder","due_at":"2026-09-28T08:00:00Z"}`)).WithContext(cards.WithTenant(context.Background(), tenant))
	response := httptest.NewRecorder()
	h.Create(response, req)
	if response.Code != 200 {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	if err = TickAll(context.Background(), db, time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow(`SELECT count(*) FROM cards WHERE source='reminder' AND title='Due reminder' AND state='open'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("due card count=%d err=%v", count, err)
	}
}
