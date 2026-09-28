package gmailsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		threads := []any{}
		if f.inbox {
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
			msgs = append(msgs, map[string]any{"id": "m" + string(rune('0'+i)), "threadId": "thread-1", "internalDate": "1780000000000", "labelIds": labels, "payload": map[string]any{"headers": []any{map[string]string{"name": "Subject", "value": "Hello"}, map[string]string{"name": "From", "value": "sender@example.test"}}}})
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "thread-1", "historyId": f.history, "messages": msgs})
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
	for _, name := range []string{"001_mail.sql", "003_agent_cards.sql", "006_mail_sync.sql"} {
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
	var state string
	if e := db.QueryRow(ctx, "SELECT id,state FROM cards WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id='thread-1'", tenant, account).Scan(&cardID, &state); e != nil {
		t.Fatal(e)
	}
	if state != "open" {
		t.Fatalf("initial card state=%s", state)
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
	RegisterOps(func(ctx context.Context, tx pgx.Tx, tenantID, card uuid.UUID) (*Client, string, error) {
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
