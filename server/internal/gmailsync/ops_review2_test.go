package gmailsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A database outage during the pre-commit reads is transient: the client keeps
// the queued operation only for non-422 answers, so it must be a 503.
func TestDatabaseOutageBeforeCommitIs503NotRejection(t *testing.T) {
	f := newSpamFixture(t)
	cfg, err := pgxpool.ParseConfig(f.dsn)
	if err != nil {
		t.Fatal(err)
	}
	down, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	down.Close() // every Begin now fails, like an unavailable Postgres
	api := f.api
	api.DB = down
	for _, kind := range []string{"archive", "spam", "bundle_archive"} {
		args := `{}`
		if kind == "bundle_archive" {
			args = `{"bundle_id":"` + uuid.NewString() + `"}`
		}
		body := `{"op_id":"` + uuid.NewString() + `","card_id":"` + f.card.String() + `","type":"` + kind + `","args":` + args + `}`
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s with database down: status=%d body=%s, want 503", kind, w.Code, w.Body.String())
		}
	}
	f.state(t, f.card, "open")
	f.checkCalls(t, nil, nil)
}

// The bundle's target set is resolved once. Members that join after the
// provider phase began are neither sent to Gmail nor archived locally, and no
// Gmail request is ever made while the tenant sequence row lock is held.
func TestBundleTargetsFixedAndNoGmailUnderSeqLock(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	bundle := uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Bundle','{0}')`, f.tenant, bundle); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card, bundle); err != nil {
		t.Fatal(err)
	}
	second := uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,bundle_id) VALUES($1,$2,$3,'thread-2','mail',$4)`, f.tenant, second, f.account, bundle); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var locked []string
	var late []uuid.UUID
	injected := false
	inner := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/batch/gmail/v1" || strings.HasSuffix(r.URL.Path, "/modify") {
			mu.Lock()
			first := !injected
			injected = true
			mu.Unlock()
			if first {
				// A member joins the bundle while the provider phase runs.
				for i := range 3 {
					id := uuid.New()
					if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,bundle_id) VALUES($1,$2,$3,$4,'mail',$5)`, f.tenant, id, f.account, fmt.Sprintf("late-%d", i), bundle); err != nil {
						t.Error(err)
					}
					mu.Lock()
					late = append(late, id)
					mu.Unlock()
				}
			}
			probe, err := f.db.Begin(ctx)
			if err != nil {
				t.Error(err)
			} else {
				_, err = probe.Exec(ctx, `SELECT 1 FROM tenants WHERE id=$1 FOR UPDATE NOWAIT`, f.tenant)
				var pg *pgconn.PgError
				if errors.As(err, &pg) && pg.Code == "55P03" {
					mu.Lock()
					locked = append(locked, r.URL.Path)
					mu.Unlock()
				} else if err != nil {
					t.Error(err)
				}
				_ = probe.Rollback(ctx)
			}
		}
		inner.ServeHTTP(w, r)
	})

	f.post(t, uuid.New(), f.card, "bundle_archive", `{"bundle_id":"`+bundle.String()+`"}`, 200)

	mu.Lock()
	defer mu.Unlock()
	if len(locked) > 0 {
		t.Errorf("Gmail requests made while the tenant sequence lock was held: %v", locked)
	}
	f.state(t, f.card, "archived")
	f.state(t, second, "archived")
	if len(late) != 3 {
		t.Fatalf("late members injected: %d", len(late))
	}
	for _, id := range late {
		f.state(t, id, "open")
	}
	for _, call := range f.calls {
		if strings.HasPrefix(call.Thread, "late-") {
			t.Errorf("Gmail archived %s though the card stays open locally", call.Thread)
		}
	}
	if len(f.calls) != 2 {
		t.Errorf("gmail calls=%v, want the two prepared members", f.calls)
	}
}
