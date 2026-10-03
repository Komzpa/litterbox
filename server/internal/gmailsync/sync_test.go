package gmailsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeGmail struct {
	inbox           bool
	messageCount    int
	history         string
	refreshes       atomic.Int32
	modifiedAccount string
	resumePageToken string
	requestedPages  []string
}

func (f *fakeGmail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		f.refreshes.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
		return
	}
	if r.Header.Get("Authorization") != "Bearer test-access" {
		http.Error(w, "unauthorized", 401)
		return
	}
	switch {
	case r.URL.Path == "/gmail/v1/users/me/threads":
		page := r.URL.Query().Get("pageToken")
		f.requestedPages = append(f.requestedPages, page)
		threads := []any{}
		if f.resumePageToken != "" && page == f.resumePageToken {
			threads = append(threads, map[string]string{"id": "thread-1"})
		} else if f.inbox {
			threads = append(threads, map[string]string{"id": "thread-1"})
		}
		json.NewEncoder(w).Encode(map[string]any{"threads": threads})
	case r.URL.Path == "/gmail/v1/users/me/profile":
		json.NewEncoder(w).Encode(map[string]string{"historyId": f.history})
	case r.URL.Path == "/gmail/v1/users/me/history":
		events := []any{}
		if r.URL.Query().Get("startHistoryId") != "1" && f.history != "1" {
			events = append(events, map[string]any{"id": f.history, "labelsAdded": []any{map[string]any{"message": map[string]string{"id": "m-new", "threadId": "thread-1"}, "labelIds": []string{"INBOX"}}}})
		} else if !f.inbox {
			events = append(events, map[string]any{"id": "2", "labelsRemoved": []any{map[string]any{"message": map[string]string{"id": "m1", "threadId": "thread-1"}, "labelIds": []string{"INBOX"}}}})
		}
		json.NewEncoder(w).Encode(map[string]any{"history": events, "historyId": f.history})
	case r.URL.Path == "/gmail/v1/users/me/threads/thread-1":
		msgs := []any{}
		for i := 1; i <= f.messageCount; i++ {
			labels := []string{"INBOX"}
			if !f.inbox && i == 1 {
				labels = nil
			}
			if i > 1 {
				labels = []string{"INBOX"}
			}
 			msgs = append(msgs, map[string]any{"id": "m" + string(rune('0'+i)), "threadId": "thread-1", "internalDate": "1780000000000", "labelIds": labels, "payload": map[string]any{"headers": []any{map[string]string{"name": "Subject", "value": "Hello"}, map[string]string{"name": "From", "value": "LinkedIn <messages-noreply@linkedin.com>"}}}})
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "thread-1", "historyId": f.history, "messages": msgs})
	case strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
		id := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/messages/")
		body := "<p>First message</p><script>unsafe()</script>"
		if id == "m2" {
			body = "<p>Second message</p>"
		}
		raw := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body
		json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
	case strings.HasSuffix(r.URL.Path, "/threads/thread-1/modify"):
		var b struct {
			Remove []string `json:"removeLabelIds"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if len(b.Remove) > 0 && b.Remove[0] == "INBOX" {
			f.modifiedAccount = "account-b"
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("CARD_TEST_POSTGRES=1 required")
	}
	ctx := context.Background()
	cfg, e := pgxpool.ParseConfig("")
	if e != nil {
		t.Fatal(e)
	}
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Close)
	if _, e := db.Exec(ctx, `DROP SCHEMA public CASCADE`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, `DROP ROLE IF EXISTS litterbox_app`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, `CREATE SCHEMA public`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, `GRANT USAGE ON SCHEMA public TO PUBLIC`); e != nil {
		t.Fatal(e)
	}
 	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "006_mail_sync.sql", "008_bundles.sql", "009_ingest.sql", "011_card_bodies.sql", "016_gmail_initial_sync.sql", "017_card_sender_name.sql"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "db", name))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(b)); e != nil {
			t.Fatalf("migration %s: %v", name, e)
		}
	}
	return db
}
func TestInitialSyncExternalArchiveReopenAndAccountArchive(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, e := db.Exec(ctx, "INSERT INTO tenants(id) VALUES($1)", tenant); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, "INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,$3,$4)", tenant, account, "b@example.test", []byte("refresh-for-account-b")); e != nil {
		t.Fatal(e)
	}
	roleTx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = roleTx.Exec(ctx, "SET LOCAL ROLE litterbox_app"); e != nil {
		t.Fatal(e)
	}
	var workerAccount uuid.UUID
	var workerToken []byte
	if e = roleTx.QueryRow(ctx, "SELECT id,refresh_token FROM litterbox_gmail_sync_accounts() WHERE tenant_id=$1", tenant).Scan(&workerAccount, &workerToken); e != nil {
		t.Fatal(e)
	}
	if workerAccount != account || string(workerToken) != "refresh-for-account-b" {
		t.Fatal("worker account discovery lost tenant token")
	}
	if e = roleTx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	fake := &fakeGmail{inbox: true, messageCount: 1, history: "1"}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client := func(Account) *Client {
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh-for-account-b", ClientID: "id", ClientSecret: "secret"}
	}
	s := &Syncer{DB: db, Client: client}
	a := Account{TenantID: tenant, ID: account}
	if e := s.SyncAccount(ctx, a); e != nil {
		t.Fatal(e)
	}
	var cardID uuid.UUID
 	var state, senderName string
 	if e := db.QueryRow(ctx, "SELECT id,state,sender_name FROM cards WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id='thread-1'", tenant, account).Scan(&cardID, &state, &senderName); e != nil {
		t.Fatal(e)
	}
	if state != "open" {
		t.Fatalf("initial card state=%s", state)
	}
 	if senderName != "LinkedIn" {
 		t.Fatalf("sender_name=%q, want LinkedIn", senderName)
 	}
	fake.inbox = false
	fake.history = "2"
	if e := s.SyncAccount(ctx, a); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(ctx, "SELECT state FROM cards WHERE id=$1", cardID).Scan(&state); e != nil {
		t.Fatal(e)
	}
	if state != "archived" {
		t.Fatalf("external archive state=%s", state)
	}
	fake.inbox = true
	fake.messageCount = 2
	fake.history = "3"
	if e := s.SyncAccount(ctx, a); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := db.QueryRow(ctx, "SELECT count(*) FROM cards WHERE id=$1 AND state='open'", cardID).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("new mail did not reopen archived card")
	}
	if e := db.QueryRow(ctx, "SELECT count(*) FROM messages WHERE card_id=$1", cardID).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 2 {
		t.Fatalf("message count=%d", count)
	}
	var messageHTML, cardHTML string
	if e := db.QueryRow(ctx, "SELECT html FROM messages WHERE card_id=$1 AND gmail_message_id='m1'", cardID).Scan(&messageHTML); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(ctx, "SELECT html FROM card_bodies WHERE card_id=$1", cardID).Scan(&cardHTML); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(messageHTML, "First message") || strings.Contains(messageHTML, "script") {
		t.Fatalf("message MIME body not safely persisted: %s", messageHTML)
	}
	if !strings.Contains(cardHTML, "First message") || !strings.Contains(cardHTML, "Second message") || strings.Contains(cardHTML, "script") {
		t.Fatalf("aggregate offline card body incorrect: %s", cardHTML)
	}
	useTestOps(func(ctx context.Context, tx pgx.Tx, tenantID, card uuid.UUID) (*Client, string, error) {
		var stored uuid.UUID
		var thread string
		if e := tx.QueryRow(ctx, "SELECT account_id,gmail_thread_id FROM cards WHERE tenant_id=$1 AND id=$2", tenantID, card).Scan(&stored, &thread); e != nil {
			return nil, "", e
		}
		if stored != account {
			return nil, "", pgx.ErrNoRows
		}
		return client(a), thread, nil
	})
	h, ok := ops.Lookup("archive")
	if !ok {
		t.Fatal("archive operation not registered")
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = h(ctx, tx, tenant, cardID, nil); e != nil {
		tx.Rollback(ctx)
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if fake.modifiedAccount != "account-b" {
		t.Fatalf("archive did not use originating account: %q", fake.modifiedAccount)
	}
	if fake.refreshes.Load() == 0 {
		t.Fatal("OAuth refresh token was not exchanged")
	}
	_ = time.Second
}
func TestInitialSyncResumesFromPersistedPage(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO tenants(id) VALUES($1)", tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO accounts(tenant_id,id,address,refresh_token,gmail_initial_sync,gmail_initial_page_token,gmail_initial_history_id) VALUES($1,$2,$3,$4,true,'page-2','1')", tenant, account, "resume@example.test", []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	fake := &fakeGmail{history: "1", resumePageToken: "page-2", inbox: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s := &Syncer{DB: db, Client: func(Account) *Client {
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh", ClientID: "id", ClientSecret: "secret"}
	}}
	if err := s.SyncAccount(ctx, Account{TenantID: tenant, ID: account}); err != nil {
		t.Fatal(err)
	}
	if len(fake.requestedPages) != 1 || fake.requestedPages[0] != "page-2" {
		t.Fatalf("initial sync pages=%q, want only saved page-2", fake.requestedPages)
	}
	var cursor string
	var started bool
	if err := db.QueryRow(ctx, "SELECT COALESCE(history_id,''),gmail_initial_sync FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, account).Scan(&cursor, &started); err != nil {
		t.Fatal(err)
	}
	if cursor != "1" || started {
		t.Fatalf("initial sync state cursor=%q started=%t", cursor, started)
	}
}
func TestClientFactoryUsesBoundedDefaultHTTPClient(t *testing.T) {
	client := ClientFactory("id", "secret", "", "", nil)(Account{})
	if client.HTTP == nil || client.HTTP.Timeout <= 0 {
		t.Fatalf("default Gmail client timeout=%v", client.HTTP)
	}
}

func TestSenderDisplayName(t *testing.T) {
	for _, tc := range []struct{ header, want string }{
		{"LinkedIn <messages-noreply@linkedin.com>", "LinkedIn"},
 		{"=?UTF-8?B?SmFuZSBEw7Y=?= <jane@example.com>", "Jane Dö"},
		{"jane@example.com", ""},
	} {
		if got := senderDisplayName(tc.header); got != tc.want {
			t.Errorf("senderDisplayName(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestBackfillSenderNameFromGmailMetadata(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account, card := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil { t.Fatal(err) }
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'legacy@example.test',$3)`, tenant, account, []byte("refresh")); err != nil { t.Fatal(err) }
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'legacy-thread','Old mail','legacy@example.test')`, tenant, card, account); err != nil { t.Fatal(err) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" { json.NewEncoder(w).Encode(map[string]any{"access_token":"test-access","expires_in":3600}); return }
		if r.URL.Path != "/gmail/v1/users/me/threads/legacy-thread" || r.URL.Query().Get("format") != "metadata" || r.URL.Query().Get("metadataHeaders") != "From" {
			t.Errorf("unexpected metadata request: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": []any{map[string]any{"payload":map[string]any{"headers":[]any{map[string]string{"name":"From","value":"LinkedIn <messages-noreply@linkedin.com>"}}}}}})
	}))
	defer srv.Close()
	client := &Client{HTTP:srv.Client(), APIBase:srv.URL+"/gmail/v1/users/me", TokenURL:srv.URL+"/token", RefreshToken:"refresh", ClientID:"id", ClientSecret:"secret"}
	syncer := &Syncer{DB:db}
	if err := syncer.backfillSenderNames(ctx, Account{TenantID:tenant, ID:account}, client); err != nil { t.Fatal(err) }
	var got string
	if err := db.QueryRow(ctx, `SELECT sender_name FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&got); err != nil { t.Fatal(err) }
	if got != "LinkedIn" { t.Fatalf("backfilled sender_name=%q, want LinkedIn", got) }
}

func TestSyncSurvivesDeletedThreadDuringSenderBackfill(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account, gone, flaky := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	// history_id='1' sends SyncAccount down the poll path once backfill returns.
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token,history_id) VALUES($1,$2,'legacy@example.test',$3,'1')`, tenant, account, []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'gone-thread','Old mail','x@example.test')`, tenant, gone, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'flaky-thread','Old mail','y@example.test')`, tenant, flaky, account); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	fetches := map[string]int{}
	historyCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/threads/gone-thread"):
			fetches["gone-thread"]++
			http.NotFound(w, r) // deleted in Gmail
		case strings.HasSuffix(r.URL.Path, "/threads/flaky-thread"):
			fetches["flaky-thread"]++
			http.Error(w, "boom", http.StatusInternalServerError) // transient
		case strings.HasSuffix(r.URL.Path, "/history"):
			historyCalls++
			json.NewEncoder(w).Encode(map[string]any{"history": []any{}, "historyId": "1"})
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := &Syncer{DB: db, Client: func(Account) *Client {
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh", ClientID: "id", ClientSecret: "secret", RetryDelay: time.Millisecond}
	}}
	a := Account{TenantID: tenant, ID: account}
	if err := s.SyncAccount(ctx, a); err != nil {
		t.Fatalf("sync aborted on deleted thread: %v", err)
	}
	mu.Lock()
	if historyCalls == 0 {
		mu.Unlock()
		t.Fatal("sync did not poll: sender backfill aborted the account before poll")
	}
	goneFetches := fetches["gone-thread"]
	mu.Unlock()
	if goneFetches != 1 {
		t.Fatalf("deleted thread fetched %d times in one sync, want 1", goneFetches)
	}
	var goneName string
	if err := db.QueryRow(ctx, `SELECT COALESCE(sender_name,'<null>') FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, gone).Scan(&goneName); err != nil {
		t.Fatal(err)
	}
	if goneName != "" {
		t.Fatalf("deleted thread sender_name = %q, want empty mark so it is not retried", goneName)
	}
	var flakyPending bool
	if err := db.QueryRow(ctx, `SELECT sender_name IS NULL FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, flaky).Scan(&flakyPending); err != nil {
		t.Fatal(err)
	}
	if !flakyPending {
		t.Fatal("transiently failing thread must stay pending for a later retry")
	}
	if err := s.SyncAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fetches["gone-thread"] != 1 {
		t.Fatalf("deleted thread fetched %d times across two syncs, want 1 (marked, never retried)", fetches["gone-thread"])
	}
}

func TestBackfillCapsBatchPerSync(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'cap@example.test',$3)`, tenant, account, []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	total := senderBackfillBatch + 1
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) SELECT $1, md5(g::text||'cap')::uuid, $2, 'live-'||g, 'S', 'x@example.test' FROM generate_series(1,$3) g`, tenant, account, total); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	fetches := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
			return
		}
		mu.Lock()
		fetches[strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/threads/")]++
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"messages": []any{map[string]any{"payload": map[string]any{"headers": []any{map[string]string{"name": "From", "value": "Name <a@b.test>"}}}}}})
	}))
	defer srv.Close()
	client := &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh", ClientID: "id", ClientSecret: "secret"}
	s := &Syncer{DB: db}
	a := Account{TenantID: tenant, ID: account}
	if err := s.backfillSenderNames(ctx, a, client); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	first := len(fetches)
	mu.Unlock()
	if first != senderBackfillBatch {
		t.Fatalf("first backfill fetched %d threads, want batch cap %d", first, senderBackfillBatch)
	}
	if err := s.backfillSenderNames(ctx, a, client); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(fetches) != total {
		t.Fatalf("after two backfills fetched %d threads, want %d (cap per sync, backlog drains)", len(fetches), total)
	}
}

func TestSyncErrorLogsCarryBuildField(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	tenant, account, card := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'logs@example.test',$3)`, tenant, account, []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'flaky-thread','S','x@example.test')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "expires_in": 3600})
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	s := &Syncer{DB: db, Build: "testbuild", Client: func(Account) *Client {
		return &Client{HTTP: srv.Client(), APIBase: srv.URL + "/gmail/v1/users/me", TokenURL: srv.URL + "/token", RefreshToken: "refresh", ClientID: "id", ClientSecret: "secret", RetryDelay: time.Millisecond}
	}}
	if err := s.backfillSenderNames(ctx, Account{TenantID: tenant, ID: account}, s.Client(Account{})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "build=testbuild") {
		t.Fatalf("sync error log line missing build field: %q", buf.String())
	}
}
