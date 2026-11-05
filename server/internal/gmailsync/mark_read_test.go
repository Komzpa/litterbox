package gmailsync

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestMarkReadFailureAndRetry(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	if _, err := f.db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,body_hash,received_at) VALUES($1,$2,$3,'m1',ARRAY['INBOX','UNREAD'],decode('00','hex'),now())`, f.tenant, uuid.New(), f.card); err != nil {
		t.Fatal(err)
	}
	op := uuid.New()
	f.mu.Lock()
	f.fail = true
	f.mu.Unlock()
	f.post(t, op, f.card, "gmail.mark_read", `{}`, 422)
	var unread bool
	check := func(want bool) {
		t.Helper()
		if err := f.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE tenant_id=$1 AND card_id=$2 AND 'UNREAD'=ANY(labels))`, f.tenant, f.card).Scan(&unread); err != nil {
			t.Fatal(err)
		}
		if unread != want {
			t.Fatalf("unread=%v want %v", unread, want)
		}
	}
	check(true)
	f.mu.Lock()
	f.fail = false
	f.mu.Unlock()
	f.post(t, op, f.card, "gmail.mark_read", `{}`, 200)
	check(false)
	f.post(t, op, f.card, "gmail.mark_read", `{}`, 200)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 2 || !reflect.DeepEqual(f.calls[1].Remove, []string{"UNREAD"}) {
		t.Fatalf("calls=%+v", f.calls)
	}
}
