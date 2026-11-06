package gmailsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repairTransport struct {
	http.RoundTripper
	budget *time.Duration
}

func (r repairTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/messages/") {
		if deadline, ok := req.Context().Deadline(); ok {
			*r.budget = time.Until(deadline)
		}
	}
	return r.RoundTripper.RoundTrip(req)
}

func TestRepairProgressAndMessageBudget(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account, card := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []string{`INSERT INTO tenants(id) VALUES($1)`, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,note,pinned_rank) VALUES($1,$3,$2,'large','snoozed','owner note',7)`} {
		args := []any{tenant}
		if strings.Contains(q, "accounts") {
			args = append(args, account)
		}
		if strings.Contains(q, "cards") {
			args = append(args, account, card)
		}
		if _, err := db.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 21; i++ {
		if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,body_hash,received_at,text) VALUES($1,$2,$3,$4,''::bytea,now()+$5*interval '1 second','old')`, tenant, uuid.New(), card, fmt.Sprintf("m%02d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if strings.HasSuffix(r.URL.Path, "/m00") {
			http.Error(w, "bad message", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte("Content-Type: text/plain\r\n\r\nrestored"))})
	}))
	defer srv.Close()
	var budget time.Duration
	client := srv.Client()
	client.Transport = repairTransport{client.Transport, &budget}
	c := &Client{HTTP: client, APIBase: srv.URL, accessToken: "x", tokenExpiry: time.Now().Add(time.Hour)}
	s := &Syncer{DB: db}
	a := Account{TenantID: tenant, ID: account}
	for i := 0; i < 25; i++ {
		// Restarting the worker cannot lose progress or cooldowns.
		s = &Syncer{DB: db}
		if err := s.backfillContent(ctx, a, c); err != nil {
			t.Fatal(err)
		}
	}
	var repaired int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM messages WHERE tenant_id=$1 AND content_version=2`, tenant).Scan(&repaired); err != nil {
		t.Fatal(err)
	}
	if repaired != 20 {
		t.Errorf("one broken message starves remaining messages: repaired=%d want 20", repaired)
	}
	if requests.Load() != 21 {
		t.Errorf("requests=%d want 21 (failure cools down, successes resume)", requests.Load())
	}
	if budget < 2*time.Minute || budget > 5*time.Minute {
		t.Errorf("per-message budget=%s want 2..5 minutes", budget)
	}
	var state, note string
	var pin int
	if err := db.QueryRow(ctx, `SELECT state,note,pinned_rank FROM cards WHERE id=$1`, card).Scan(&state, &note, &pin); err != nil {
		t.Fatal(err)
	}
	if state != "snoozed" || note != "owner note" || pin != 7 {
		t.Fatal("owner state changed")
	}
}

func TestRepairProviderDeletedIsTerminalAndRetainsContent(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account, card, message := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id) VALUES($1,$2,$3,'gone')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,body_hash,received_at,text) VALUES($1,$2,$3,'gone',''::bytea,now(),'retained owner-visible content')`, tenant, message, card); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.NotFound(w, r) }))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, accessToken: "x", tokenExpiry: time.Now().Add(time.Hour)}
	s := &Syncer{DB: db}
	a := Account{TenantID: tenant, ID: account}
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	// Simulate a later poll, beyond the old card cooldown.
	if _, err := db.Exec(ctx, `UPDATE cards SET mail_content_checked_at=now()-interval '2 hours' WHERE id=$1`, card); err != nil {
		t.Fatal(err)
	}
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	var text, status string
	if err := db.QueryRow(ctx, `SELECT text,decode_status FROM messages WHERE id=$1`, message).Scan(&text, &status); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || text != "retained owner-visible content" || !strings.Contains(status, "Gmail 404") {
		t.Fatalf("requests=%d text=%q status=%q", requests.Load(), text, status)
	}
}
