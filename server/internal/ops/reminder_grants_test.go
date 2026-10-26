package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/Komzpa/litterbox/server/internal/reminders"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestCreateReminderAsAppRole(t *testing.T) {
	admin := journalMigrationDatabase(t)
	ctx := context.Background()
	tenant, other, device, id := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO tenants(id) VALUES($1),($2)`, tenant, other); err != nil { t.Fatal(err) }
	if _, err := admin.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, tenant, device, []byte("reminder-device")); err != nil { t.Fatal(err) }
	cfg := admin.Config().Copy()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE litterbox_app`)
		return err
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	var role string
	var bypass, super bool
	if err := db.QueryRow(ctx, `SELECT current_user, rolbypassrls, rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&role, &bypass, &super); err != nil { t.Fatal(err) }
	if role != "litterbox_app" || bypass || super { t.Fatalf("runtime identity=%q bypassrls=%v superuser=%v", role, bypass, super) }
	api := API{DB: db, Identity: func(context.Context) (string, string, bool) { return tenant.String(), device.String(), true }}
	payload := func(op uuid.UUID, args string) string {
		return fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":"create_reminder","args":%s}`, op, id, args)
	}
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
		return response
	}
	args := fmt.Sprintf(`{"id":%q,"title":"  Private reminder  ","due_at":"2026-10-27T12:00:00+04:00","recurrence":"weekly"}`, id)
	op := uuid.New()
	first := post(payload(op, args))
	if first.Code != http.StatusOK { t.Fatalf("create: status=%d body=%s", first.Code, first.Body.String()) }
	replay := post(payload(op, args))
	if replay.Code != first.Code || replay.Body.String() != first.Body.String() { t.Fatalf("replay: status=%d body=%s; original=%d %s", replay.Code, replay.Body.String(), first.Code, first.Body.String()) }
	changed := strings.Replace(args, "Private reminder", "Replacement must not win", 1)
	if response := post(payload(uuid.New(), changed)); response.Code != http.StatusOK { t.Fatalf("same reminder ID: status=%d body=%s", response.Code, response.Body.String()) }
	var count int
	var title, recurrence, state string
	var due time.Time
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM reminders WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("reminder count=%d, want 1", count) }
	if err := admin.QueryRow(ctx, `SELECT title,due_at,recurrence,state FROM reminders WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&title, &due, &recurrence, &state); err != nil { t.Fatal(err) }
	if title != "Private reminder" || !due.Equal(time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)) || recurrence != "weekly" || state != "scheduled" { t.Fatalf("persisted title=%q due=%v recurrence=%q state=%q", title, due, recurrence, state) }
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1`, tenant).Scan(&count); err != nil { t.Fatal(err) }
	if count != 2 { t.Fatalf("operation count=%d, want 2", count) }
	if response := post(payload(op, changed)); response.Code != http.StatusConflict { t.Fatalf("changed op replay: status=%d body=%s", response.Code, response.Body.String()) }

	for _, visible := range []struct{ tenant uuid.UUID; count int }{{tenant, 1}, {other, 0}} {
		tx, err := db.Begin(ctx)
		if err != nil { t.Fatal(err) }
		if _, err := tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, visible.tenant.String()); err != nil { tx.Rollback(ctx); t.Fatal(err) }
		err = tx.QueryRow(ctx, `SELECT count(*) FROM reminders WHERE id=$1`, id).Scan(&count)
		tx.Rollback(ctx)
		if err != nil { t.Fatal(err) }
		if count != visible.count { t.Fatalf("RLS tenant=%s sees %d reminders, want %d", visible.tenant, count, visible.count) }
	}

	for _, invalid := range []string{
		`{"title":"Missing ID","due_at":"2026-10-27T08:00:00Z"}`,
		`{"id":"not-a-uuid","title":"Bad ID","due_at":"2026-10-27T08:00:00Z"}`,
		fmt.Sprintf(`{"id":%q,"title":" ","due_at":"2026-10-27T08:00:00Z"}`, uuid.New()),
		fmt.Sprintf(`{"id":%q,"title":"Missing due"}`, uuid.New()),
		fmt.Sprintf(`{"id":%q,"title":"Bad recurrence","due_at":"2026-10-27T08:00:00Z","recurrence":"monthly"}`, uuid.New()),
	} {
		if response := post(payload(uuid.New(), invalid)); response.Code != http.StatusUnprocessableEntity { t.Fatalf("invalid args %s: status=%d body=%s", invalid, response.Code, response.Body.String()) }
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1`, tenant).Scan(&count); err != nil { t.Fatal(err) }
	if count != 2 { t.Fatalf("rejected ops persisted: count=%d, want 2", count) }

	legacyDB := stdlib.OpenDBFromPool(db)
	defer legacyDB.Close()
	mux := http.NewServeMux()
	(&reminders.Handler{DB: legacyDB}).Routes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("POST", "/v1/reminders", strings.NewReader(`{"title":"Legacy reminder","due_at":"2026-10-28T08:00:00Z"}`)).WithContext(cards.WithTenant(ctx, tenant.String())))
	if response.Code != http.StatusOK { t.Fatalf("legacy create: status=%d body=%s", response.Code, response.Body.String()) }
	var created struct { ID uuid.UUID `json:"id"` }
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.ID == uuid.Nil { t.Fatalf("legacy id: %s error=%v", response.Body.String(), err) }
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM reminders WHERE tenant_id=$1 AND id=$2 AND title='Legacy reminder' AND recurrence=''`, tenant, created.ID).Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("legacy persisted count=%d, want 1", count) }
	t.Log("create_reminder as litterbox_app: creates one normalized reminder; identical op replay returns original response; second op preserves reminder; other tenant sees zero; invalid args roll back; legacy POST persists server-generated ID")
}
