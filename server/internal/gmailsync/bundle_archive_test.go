package gmailsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The Gmail ops handlers register once per test binary; each test installs the
// client resolver it needs before dispatching registered ops.
var (
	testOpsClientFor func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error)
	testOpsOnce      sync.Once
)

func useTestOps(clientFor func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error)) {
	testOpsClientFor = clientFor
	testOpsOnce.Do(func() {
		RegisterOps(func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID) (*Client, string, error) {
			return testOpsClientFor(ctx, tx, tenant, card)
		})
	})
}

// The legacy bundle_archive op must archive every unpinned member through the
// per-card archive op, i.e. remove INBOX in Gmail for each member's thread and
// never touch pinned members.
func TestBundleArchiveRemovesInboxForEveryUnpinnedMember(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account, bundle := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'bundle@example.test',$3)`, tenant, account, []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Bundle','{0}')`, tenant, bundle); err != nil {
		t.Fatal(err)
	}
	type member struct {
		thread string
		card   uuid.UUID
		rank   any
	}
	members := []member{
		{"thread-a", uuid.New(), nil},
		{"thread-b", uuid.New(), nil},
		{"thread-pinned", uuid.New(), int64(1)},
	}
	for _, m := range members {
		if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,bundle_id,pinned_rank) VALUES($1,$2,$3,$4,'mail',$5,$6)`, tenant, m.card, account, m.thread, bundle, m.rank); err != nil {
			t.Fatal(err)
		}
	}

	type modifyCall struct {
		thread string
		remove []string
	}
	var mu sync.Mutex
	var calls []modifyCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/modify") {
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		thread := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/threads/"), "/modify")
		var body struct {
			Remove []string `json:"removeLabelIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		mu.Lock()
		calls = append(calls, modifyCall{thread: thread, remove: body.Remove})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	useTestOps(func(ctx context.Context, tx pgx.Tx, tenantID, card uuid.UUID) (*Client, string, error) {
		var thread string
		if err := tx.QueryRow(ctx, `SELECT gmail_thread_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenantID, card).Scan(&thread); err != nil {
			return nil, "", err
		}
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh", ClientID: "id", ClientSecret: "secret"}, thread, nil
	})

	h, ok := ops.Lookup("bundle_archive")
	if !ok {
		t.Fatal("bundle_archive operation not registered")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(`{"bundle_id":"` + bundle.String() + `"}`)
	if err = h(ctx, tx, tenant, uuid.Nil, args); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	got := map[string][]string{}
	for _, c := range calls {
		got[c.thread] = c.remove
	}
	mu.Unlock()
	for _, thread := range []string{"thread-a", "thread-b"} {
		remove := got[thread]
		if len(remove) != 1 || remove[0] != "INBOX" {
			t.Fatalf("member %s modify remove labels = %v, want [INBOX] (no Gmail call before the fix)", thread, remove)
		}
	}
	if remove, pinned := got["thread-pinned"]; pinned {
		t.Fatalf("pinned member got a Gmail modify call: remove=%v", remove)
	}
	for _, m := range members {
		var state string
		if err := db.QueryRow(ctx, `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, m.card).Scan(&state); err != nil {
			t.Fatal(err)
		}
		want := "archived"
		if m.thread == "thread-pinned" {
			want = "open"
		}
		if state != want {
			t.Fatalf("card %s state = %q, want %q", m.thread, state, want)
		}
	}
}


func TestStaleBundleArchiveReturnsErrorWithoutRecordingOperation(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, device, opID, cardID, bundleID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, tenant, device, []byte("token")); err != nil {
		t.Fatal(err)
	}
	api := ops.API{DB: db, Identity: func(context.Context) (string, string, bool) { return tenant.String(), device.String(), true }}
	body := `{"op_id":"` + opID.String() + `","card_id":"` + cardID.String() + `","type":"bundle_archive","args":{"bundle_id":"` + bundleID.String() + `"}}`
	w := httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stale bundle status=%d, want 422", w.Code)
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, tenant, opID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale bundle left %d ops rows, want 0", count)
	}
}
