package gmailsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type spamCall struct {
	Thread string
	Token string
	Add []string `json:"addLabelIds"`
	Remove []string `json:"removeLabelIds"`
}

type spamFixture struct {
	db *pgxpool.Pool
	tenant, account, card uuid.UUID
	api ops.API
	server *httptest.Server
	mu sync.Mutex
	calls []spamCall
	fail bool
	gmail fakeGmail
}

func newSpamFixture(t *testing.T) *spamFixture {
	t.Helper()
	db, tdb := testPool(t)
	f := &spamFixture{db: db, tenant: uuid.New(), account: uuid.New(), card: uuid.New(), gmail: fakeGmail{inbox: true, messageCount: 1, history: "1"}}
	ctx := context.Background()
	for _, name := range []string{"005_card_notify.sql", "007_offline_ops.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, tdb.Migration(string(migration))); err != nil {
			t.Fatal(err)
		}
	}
	device := uuid.New()
	for _, stmt := range []struct { sql string; args []any }{
		{`INSERT INTO tenants(id) VALUES($1)`, []any{f.tenant}},
		{`INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, []any{f.tenant, device, []byte("fixture-device")}},
		{`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'origin@example.test',$3),($1,$4,'other@example.test',$5)`, []any{f.tenant, f.account, []byte("origin-refresh"), uuid.New(), []byte("wrong-refresh")}},
		{`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name) VALUES($1,$2,$3,'thread-1','mail','Sender')`, []any{f.tenant, f.card, f.account}},
	} {
		if _, err := db.Exec(ctx, stmt.sql, stmt.args...); err != nil { t.Fatal(err) }
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.URL.Path == "/token" {
			if err := r.ParseForm(); err != nil { t.Error(err) }
			if got := r.Form.Get("refresh_token"); got != "origin-refresh" { t.Errorf("wrong account refresh token: %q", got) }
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/modify") {
			var call spamCall
			if err := json.NewDecoder(r.Body).Decode(&call); err != nil { t.Error(err) }
			call.Thread = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/threads/"), "/modify")
			call.Token = r.Header.Get("Authorization")
			f.calls = append(f.calls, call)
			if f.fail { http.Error(w, "fixture Gmail denied", http.StatusForbidden); return }
			f.gmail.inbox = false
			f.gmail.history = "2"
			w.WriteHeader(http.StatusOK)
			return
		}
		f.gmail.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	useTestOps(ClientForCard("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client()))
	f.api = ops.API{DB: db, Identity: func(context.Context) (string, string, bool) { return f.tenant.String(), device.String(), true }}
	return f
}

func (f *spamFixture) post(t *testing.T, opID, card uuid.UUID, kind, args string, want int) string {
	t.Helper()
	body := `{"op_id":"`+opID.String()+`","card_id":"`+card.String()+`","type":"`+kind+`","args":`+args+`}`
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
	if w.Code != want { t.Fatalf("%s status=%d body=%s, want %d", kind, w.Code, w.Body.String(), want) }
	return w.Body.String()
}

func (f *spamFixture) state(t *testing.T, card uuid.UUID, want string) {
	t.Helper()
	var got string
	if err := f.db.QueryRow(context.Background(), `SELECT state FROM cards WHERE tenant_id=$1 AND id=$2`, f.tenant, card).Scan(&got); err != nil { t.Fatal(err) }
	if got != want { t.Fatalf("card state=%q, want %q", got, want) }
}

func (f *spamFixture) checkCalls(t *testing.T, threads []string, add []string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != len(threads) { t.Fatalf("Gmail calls=%v, want threads %v", f.calls, threads) }
	seen := map[string]bool{}
	for _, call := range f.calls {
		if !reflect.DeepEqual(call.Add, add) || !reflect.DeepEqual(call.Remove, []string{"INBOX"}) || call.Token != "Bearer test-access" {
			t.Fatalf("wrong Gmail labels/account: %+v", call)
		}
		seen[call.Thread] = true
	}
	for _, thread := range threads { if !seen[thread] { t.Fatalf("missing thread %s: %+v", thread, f.calls) } }
}

func TestSpamSuccessDuplicateAndResync(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	var cursor int64
	if err := f.db.QueryRow(ctx, `SELECT COALESCE(max(seq),0) FROM changes WHERE tenant_id=$1`, f.tenant).Scan(&cursor); err != nil { t.Fatal(err) }
	op := uuid.New()
	if got := f.post(t, op, f.card, "spam", `{}`, 200); got != `{"ok":true}` { t.Fatalf("response=%s", got) }
	f.state(t, f.card, "archived")
	f.post(t, op, f.card, "spam", `{}`, 200)
	f.checkCalls(t, []string{"thread-1"}, []string{"SPAM"})
	var count int
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM changes WHERE tenant_id=$1 AND seq>$2 AND entity='card' AND id=$3 AND value->>'state'='archived'`, f.tenant, cursor, f.card).Scan(&count); err != nil { t.Fatal(err) }
	if count == 0 { t.Fatal("other devices received no durable archived card change") }
	// A different op ID is a harmless repeat of Gmail's idempotent label modify.
	f.post(t, uuid.New(), f.card, "spam", `{}`, 200)
	f.state(t, f.card, "archived")
	f.checkCalls(t, []string{"thread-1", "thread-1"}, []string{"SPAM"})
	f.post(t, op, f.card, "spam", `{"label":"different"}`, 409)
	client := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: client}
	a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
	// Incremental history, then an expired-cursor/full reconciliation.
	if _, err := f.db.Exec(ctx, `UPDATE accounts SET history_id='1' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.account); err != nil { t.Fatal(err) }
	if err := s.SyncAccount(ctx, a); err != nil { t.Fatal(err) }
	f.state(t, f.card, "archived")
	if _, err := f.db.Exec(ctx, `UPDATE accounts SET history_id=NULL WHERE tenant_id=$1 AND id=$2`, f.tenant, f.account); err != nil { t.Fatal(err) }
	if err := s.SyncAccount(ctx, a); err != nil { t.Fatal(err) }
	f.state(t, f.card, "archived")
	f.checkCalls(t, []string{"thread-1", "thread-1"}, []string{"SPAM"})
}

func TestSpamFailureRollsBackLikeArchive(t *testing.T) {
	for _, kind := range []string{"spam", "archive"} {
		t.Run(kind, func(t *testing.T) {
			f := newSpamFixture(t)
			f.fail = true
			op := uuid.New()
			body := f.post(t, op, f.card, kind, `{}`, 422)
			var response map[string]string
			if err := json.Unmarshal([]byte(body), &response); err != nil { t.Fatal(err) }
			if !strings.Contains(response["error"], "403 Forbidden") || !strings.Contains(response["error"], "fixture Gmail denied") { t.Fatalf("error=%s", body) }
			f.state(t, f.card, "open")
			var count int
			if err := f.db.QueryRow(context.Background(), `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, f.tenant, op).Scan(&count); err != nil { t.Fatal(err) }
			if count != 0 { t.Fatal("failed op receipt persisted") }
			f.fail = false
			f.post(t, op, f.card, kind, `{}`, 200)
			f.state(t, f.card, "archived")
		})
	}
}

func TestBundleSpamTargetsUnpinnedMembersInOriginatingAccounts(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	bundle, second, pinned := uuid.New(), uuid.New(), uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Fixture','{0}')`, f.tenant, bundle); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card, bundle); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,bundle_id,pinned_rank) VALUES($1,$2,$3,'thread-2',$4,NULL),($1,$5,$3,'thread-pinned',$4,1)`, f.tenant, second, f.account, bundle, pinned); err != nil { t.Fatal(err) }
	f.post(t, uuid.New(), f.card, "bundle_spam", `{"bundle_id":"`+bundle.String()+`"}`, 200)
	f.checkCalls(t, []string{"thread-1", "thread-2"}, []string{"SPAM"})
	f.state(t, f.card, "archived")
	f.state(t, second, "archived")
	f.state(t, pinned, "open")
}

func TestSpamRejectsNonMailAndForeignCardsWithoutGmailCall(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	manual, foreign, otherTenant := uuid.New(), uuid.New(), uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id) VALUES($1,$2,'manual','manual-fixture')`, f.tenant, manual); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, otherTenant); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id) VALUES($1,$2,'manual','foreign-fixture')`, otherTenant, foreign); err != nil { t.Fatal(err) }
	f.post(t, uuid.New(), manual, "spam", `{}`, 422)
	f.post(t, uuid.New(), foreign, "spam", `{}`, 422)
	f.state(t, manual, "open")
	f.state(t, f.card, "open")
	f.checkCalls(t, nil, nil)
}

// Pre-flush Undo lives in Qt: it removes the local outbox item so /v1/ops
// receives nothing. Account synchronization must not manufacture such writes.
func TestSpamNotSubmittedDoesNotWriteGmailDuringSync(t *testing.T) {
	f := newSpamFixture(t)
	client := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: client}
	if err := s.SyncAccount(context.Background(), Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}); err != nil { t.Fatal(err) }
	f.state(t, f.card, "open")
	f.checkCalls(t, nil, nil)
}
