package gmailsync

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// An owner archive must survive a Gmail sync that still sees the thread in the
// inbox (the incident: the sync upsert re-opened the card), while a genuinely
// new message must still resurface it. The archive itself must reach Gmail.
func TestArchiveSurvivesSyncUntilNewMessage(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	client := ClientFactory("id", "secret", f.server.URL+"/gmail/v1/users/me", f.server.URL+"/token", f.server.Client())
	s := &Syncer{DB: f.db, Client: client}
	a := Account{TenantID: f.tenant, ID: f.account, RefreshToken: []byte("origin-refresh")}
	resync := func(inbox bool, messages int) {
		t.Helper()
		f.mu.Lock()
		f.gmail.inbox, f.gmail.messageCount = inbox, messages
		f.mu.Unlock()
		if _, err := f.db.Exec(ctx, `UPDATE accounts SET history_id=NULL WHERE tenant_id=$1 AND id=$2`, f.tenant, f.account); err != nil {
			t.Fatal(err)
		}
		if err := s.SyncAccount(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	resync(true, 1)
	f.state(t, f.card, "open")

	f.post(t, uuid.New(), f.card, "archive", `{}`, 200)
	f.state(t, f.card, "archived")
	f.checkCalls(t, []string{"thread-1"}, nil)

	// Gmail still reports the thread in the inbox with the same single message.
	resync(true, 1)
	f.state(t, f.card, "archived")
	resync(true, 1)
	f.state(t, f.card, "archived")

	// A new message in the thread re-opens it.
	resync(true, 2)
	f.state(t, f.card, "open")
}

// The archive op reaches Gmail through the registered connector, removing INBOX
// from the card's thread on the originating account.
func TestArchiveOpCallsGmailArchiveWhenConnectorRegistered(t *testing.T) {
	f := newSpamFixture(t)
	f.post(t, uuid.New(), f.card, "archive", `{}`, 200)
	f.state(t, f.card, "archived")
	f.checkCalls(t, []string{"thread-1"}, nil)
}
