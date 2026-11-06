package gmailsync

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoryReinboxWithoutNewMessage(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	factory := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: factory}
	a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
	if err := s.saveThread(ctx, a, factory(a), "thread-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil {
		t.Fatal(err)
	}
	// Exclude the one-time repair: this must exercise ordinary history polling.
	if _, err := f.db.Exec(ctx, `UPDATE accounts SET history_id='2' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.account); err != nil {
		t.Fatal(err)
	}
	f.gmail.history = "3"
	if err := s.poll(ctx, a, factory(a), "2"); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "open")
}

func TestInitialAbsenceRequiresProviderConfirmation(t *testing.T) {
	for _, inbox := range []bool{true, false} {
		t.Run(map[bool]string{true: "moving_snapshot", false: "confirmed_archive"}[inbox], func(t *testing.T) {
			f := newSpamFixture(t)
			ctx := context.Background()
			f.gmail.inbox = inbox
			// threads.list omits the thread; threads.get still returns its true labels.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gmail/v1/users/me/threads" {
					json.NewEncoder(w).Encode(map[string]any{"threads": []any{}})
					return
				}
				f.server.Config.Handler.ServeHTTP(w, r)
			}))
			defer srv.Close()
			factory := ClientFactory("id", "secret", srv.URL+"/gmail/v1/users/me", srv.URL+"/token", srv.Client())
			s := &Syncer{DB: f.db, Client: factory}
			a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
			if err := s.initial(ctx, a, factory(a), false, "", ""); err != nil {
				t.Fatal(err)
			}
			want := "archived"
			if inbox {
				want = "open"
			}
			f.state(t, f.card, want)
		})
	}
}

func TestLabelStateReconciliationAndIntentBoundary(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	factory := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: factory}
	a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
	if err := s.saveThread(ctx, a, factory(a), "thread-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `UPDATE cards SET state='archived',gmail_pending_inbox=false,gmail_intent_history=10 WHERE id=$1`, f.card); err != nil {
		t.Fatal(err)
	}
	// An old INBOX event must not reverse an acknowledged local archive.
	if err := s.pollThread(ctx, a, factory(a), "thread-1", "9", ""); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "archived")
	// The one-time repair also respects the pending archive.
	if err := s.reconcileLabelState(ctx, a, factory(a)); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "archived")
	// A later real re-inbox transition wins, without a new message.
	if err := s.pollThread(ctx, a, factory(a), "thread-1", "11", ""); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "open")
	// A legacy archive without a pending intent follows confirmed provider state.
	if _, err := f.db.Exec(ctx, `UPDATE cards SET state='archived',gmail_pending_inbox=NULL,gmail_state_checked=false WHERE id=$1`, f.card); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcileLabelState(ctx, a, factory(a)); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "open")
	var reason string
	if err := f.db.QueryRow(ctx, `SELECT gmail_state_reason FROM cards WHERE id=$1`, f.card).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason != "legacy_provider_reconciliation_inbox" {
		t.Fatal(reason)
	}
}

func TestArchiveRecordsProviderHistory(t *testing.T) {
	f := newSpamFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/modify") {
			json.NewEncoder(w).Encode(map[string]string{"id": "thread-1", "historyId": "9007199254740993"})
			return
		}
		f.server.Config.Handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	useTestOps(ClientForCard("id", "secret", srv.URL+"/gmail/v1/users/me", srv.URL+"/token", srv.Client()))
	f.post(t, uuid.New(), f.card, "archive", `{}`, 200)
	var pending bool
	var history string
	if err := f.db.QueryRow(context.Background(), `SELECT gmail_pending_inbox,gmail_intent_history::text FROM cards WHERE id=$1`, f.card).Scan(&pending, &history); err != nil {
		t.Fatal(err)
	}
	if pending || history != "9007199254740993" {
		t.Fatalf("pending=%v history=%s", pending, history)
	}
}

func TestPendingSnoozeRetainsStateOnStaleInbox(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	factory := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: factory}
	a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
	if err := s.saveThread(ctx, a, factory(a), "thread-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `UPDATE cards SET state='snoozed',gmail_pending_inbox=false WHERE id=$1`, f.card); err != nil {
		t.Fatal(err)
	}
	if err := s.saveThread(ctx, a, factory(a), "thread-1", true); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "snoozed")
	// A new message still wakes the snoozed conversation early (R20).
	f.gmail.messageCount = 2
	if err := s.saveThread(ctx, a, factory(a), "thread-1", true); err != nil {
		t.Fatal(err)
	}
	f.state(t, f.card, "open")
}
