package diagnostics

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is the real auth -> registered HTTP route -> application-role Postgres
// transaction, not a helper upload pretending to prove the consumer boundary.
func TestPostgresAuthenticatedReceiptIsolationAndRestart(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("requires disposable PostgreSQL: CARD_TEST_POSTGRES=1")
	}
	ctx := context.Background()
	fixture := testdb.Setup(t)
	admin, err := pgxpool.New(ctx, fixture.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	for _, name := range []string{"001_mail.sql", "002_security.sql", "010_auth.sql", "026_crash_reports.sql"} {
		raw, err := os.ReadFile("../../db/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, fixture.Migration(string(raw))); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	tenant, otherTenant := uuid.NewString(), uuid.NewString()
	device, sibling, foreign := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := admin.Exec(ctx, `INSERT INTO tenants(id) VALUES($1::uuid),($2::uuid)`, tenant, otherTenant); err != nil {
		t.Fatal(err)
	}
	for _, d := range []struct{ tenant, id, token string }{{tenant, device, "first"}, {tenant, sibling, "sibling"}, {otherTenant, foreign, "foreign"}} {
		hash := sha256.Sum256([]byte(d.token))
		if _, err := admin.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1::uuid,$2::uuid,$3)`, d.tenant, d.id, hash[:]); err != nil {
			t.Fatal(err)
		}
	}
	cfg := admin.Config().Copy()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE `+fixture.Role)
		return err
	}
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	var role string
	var bypass, super bool
	if err := app.QueryRow(ctx, `SELECT current_user, rolbypassrls, rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&role, &bypass, &super); err != nil {
		t.Fatal(err)
	}
	if role != fixture.Role || bypass || super {
		t.Fatalf("unsafe test identity: %s %v %v", role, bypass, super)
	}
	newConsumer := func() http.Handler {
		mux := http.NewServeMux()
		Register(mux, PostgresStore{DB: app})
		return auth.NewHandler(auth.NewPostgresStore(app), "").Middleware(mux)
	}
	h := newConsumer()
	r := sample()
	for _, token := range []string{"", "unknown"} {
		if w := request(h, "POST", Path, token, wire(r)); w.Code != 401 {
			t.Fatalf("unauthorized=%d", w.Code)
		}
	}
	for _, token := range []string{"first", "sibling", "foreign"} {
		if w := request(h, "POST", Path, token, wire(r)); w.Code != 200 || w.Body.String() != "{\"ack\":\"java-123\"}\n" {
			t.Fatalf("receipt %s: %d %s", token, w.Code, w.Body.String())
		}
	}
	// New handler/store instance represents service restart. A changed retry
	// cannot overwrite the immutable original or its first receipt timestamp.
	var before string
	if err := admin.QueryRow(ctx, `SELECT received_at::text FROM crash_reports WHERE tenant_id=$1::uuid AND device_id=$2::uuid`, tenant, device).Scan(&before); err != nil {
		t.Fatal(err)
	}
	r.Body = exitBody
	if w := request(newConsumer(), "POST", Path, "first", wire(r)); w.Code != 200 {
		t.Fatalf("duplicate receipt=%d %s", w.Code, w.Body.String())
	}
	var count int
	var body, after, kind, version, build string
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM crash_reports`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("attribution or duplicate row count=%d", count)
	}
	if err := admin.QueryRow(ctx, `SELECT body,received_at::text,kind,app_version,uploader_build FROM crash_reports WHERE tenant_id=$1::uuid AND device_id=$2::uuid AND report_id=$3`, tenant, device, r.ID).Scan(&body, &after, &kind, &version, &build); err != nil {
		t.Fatal(err)
	}
	if body != javaBody || before != after || kind != "java" || version != "0.1.53" || build != r.UploaderBuild {
		t.Fatalf("immutable operator readback: %q %q %q %q %q", body, after, kind, version, build)
	}
	// RLS must hide every row without scope, then expose only one device even
	// when another device shares the same tenant and the report ID collides.
	if err := app.QueryRow(ctx, `SELECT count(*) FROM crash_reports`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unscoped read: %d %v", count, err)
	}
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true),set_config('litterbox.device_id',$2,true)`, tenant, device); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM crash_reports`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("scoped read=%d %v", count, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crash_reports(tenant_id,device_id,report_id,kind,app_version,uploader_build,body) VALUES($1::uuid,$2::uuid,'forged','java','x','x','x')`, tenant, sibling); err == nil {
		t.Fatal("wrong-device insert bypassed RLS")
	}
	tx.Rollback(ctx)
	tx, err = app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true),set_config('litterbox.device_id',$2,true)`, tenant, device); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crash_reports(tenant_id,device_id,report_id,kind,app_version,uploader_build,body) VALUES($1::uuid,$2::uuid,'forged-tenant','java','x','x','x')`, otherTenant, foreign); err == nil {
		t.Fatal("cross-tenant insert bypassed RLS")
	}
	tx.Rollback(ctx)
	// A device ID from a different tenant cannot pass the composite FK even
	// when a caller supplies internally inconsistent scope to the store.
	if err := (PostgresStore{DB: app}).Save(ctx, auth.Device{TenantID: tenant, DeviceID: foreign}, sample(), "java", "0.1.53"); err == nil {
		t.Fatal("cross-tenant device accepted")
	}
	bad := sample()
	bad.Body += "private mail: do not retain\n"
	if w := request(h, "POST", Path, "first", wire(bad)); w.Code != 422 {
		t.Fatalf("privacy negative control=%d", w.Code)
	}
	if _, err := admin.Exec(ctx, `UPDATE devices SET revoked_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid`, tenant, device); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "POST", Path, "first", wire(sample())); w.Code != 401 {
		t.Fatalf("revoked auth=%d", w.Code)
	}
	if w := request(h, "GET", Path, "sibling", ""); w.Code != 405 {
		t.Fatalf("public read endpoint=%d", w.Code)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM crash_reports`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("negative controls wrote data: %d %v", count, err)
	}
	for _, lockFirst := range []bool{false, true} {
		name := "revoke-after-auth-before-store"
		if lockFirst {
			name = "persistence-lock-before-revoke"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			id, token := uuid.NewString(), uuid.NewString()
			hash := sha256.Sum256([]byte(token))
			if _, err := admin.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1::uuid,$2::uuid,$3)`, tenant, id, hash[:]); err != nil {
				t.Fatal(err)
			}
			gate := &receiptGateDB{db: app, afterLock: lockFirst, entered: make(chan int, 1), release: make(chan struct{})}
			mux := http.NewServeMux()
			Register(mux, PostgresStore{DB: gate})
			a := auth.NewHandler(auth.NewPostgresStore(app), "")
			a.Register(mux)
			consumer := a.Middleware(mux)
			call := func(method, path, bearer, body string) <-chan *httptest.ResponseRecorder {
				done := make(chan *httptest.ResponseRecorder, 1)
				go func() {
					r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
					r.Header.Set("Authorization", "Bearer "+bearer)
					w := httptest.NewRecorder()
					consumer.ServeHTTP(w, r)
					done <- w
				}()
				return done
			}
			await := func(done <-chan *httptest.ResponseRecorder) *httptest.ResponseRecorder {
				t.Helper()
				select {
				case w := <-done:
					return w
				case <-ctx.Done():
					t.Fatal(ctx.Err())
					return nil
				}
			}
			upload := call("POST", Path, token, wire(sample()))
			var savePID int
			select {
			case savePID = <-gate.entered:
			case <-ctx.Done():
				t.Fatal("upload never reached persistence gate: ", ctx.Err())
			}
			// Revoke via the registered real auth route using an active sibling.
			revoke := call("DELETE", "/v1/devices/"+id, "sibling", "")
			if lockFirst {
				// Observe a database lock dependency, not a scheduling delay.
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for {
					var blocked bool
					if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_stat_activity WHERE $1::int = ANY(pg_blocking_pids(pid)))`, savePID).Scan(&blocked); err != nil {
						t.Fatal(err)
					}
					if blocked {
						break
					}
					select {
					case w := <-revoke:
						t.Fatalf("revoke completed before locked save committed: %d", w.Code)
					case <-ticker.C:
					case <-ctx.Done():
						t.Fatal("revoke did not block on persistence row lock: ", ctx.Err())
					}
				}
			} else if w := await(revoke); w.Code != 204 {
				t.Fatalf("revoke before store: %d %s", w.Code, w.Body.String())
			}
			close(gate.release)
			w := await(upload)
			want, wantRows := 503, 0
			if lockFirst {
				want, wantRows = 200, 1
				if rw := await(revoke); rw.Code != 204 {
					t.Fatalf("revoke after save: %d %s", rw.Code, rw.Body.String())
				}
			}
			if w.Code != want || (want == 503 && strings.Contains(w.Body.String(), "ack")) || (want == 200 && w.Body.String() != "{\"ack\":\"java-123\"}\n") {
				t.Fatalf("ordered upload: %d %s", w.Code, w.Body.String())
			}
			var rows int
			if err := admin.QueryRow(ctx, `SELECT count(*) FROM crash_reports WHERE tenant_id=$1::uuid AND device_id=$2::uuid`, tenant, id).Scan(&rows); err != nil || rows != wantRows {
				t.Fatalf("committed rows=%d want %d: %v", rows, wantRows, err)
			}
			if w := await(call("POST", Path, token, wire(sample()))); w.Code != 401 {
				t.Fatalf("completed revoke did not reject new request: %d", w.Code)
			}
		})
	}
}

// Test-only pgx decorators gate real transactions; production has no hooks.
// Before Begin is after middleware auth; after Scan is after the device check
// and row-lock acquisition, before INSERT/COMMIT. All SQL runs on real Postgres.
type receiptGateDB struct {
	db        *pgxpool.Pool
	afterLock bool
	entered   chan int
	release   chan struct{}
}

func (g *receiptGateDB) wait(ctx context.Context, pid int) error {
	g.entered <- pid
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *receiptGateDB) Begin(ctx context.Context) (pgx.Tx, error) {
	if !g.afterLock {
		if err := g.wait(ctx, 0); err != nil {
			return nil, err
		}
	}
	tx, err := g.db.Begin(ctx)
	if err != nil || !g.afterLock {
		return tx, err
	}
	return &receiptGateTx{Tx: tx, gate: g}, nil
}

type receiptGateTx struct {
	pgx.Tx
	gate *receiptGateDB
}

func (tx *receiptGateTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "FROM devices") {
		return receiptGateRow{Row: row, ctx: ctx, tx: tx}
	}
	return row
}

type receiptGateRow struct {
	pgx.Row
	ctx context.Context
	tx  *receiptGateTx
}

func (r receiptGateRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	var pid int
	if err := r.tx.Tx.QueryRow(r.ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		return err
	}
	return r.tx.gate.wait(r.ctx, pid)
}
