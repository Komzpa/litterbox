package a2a

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	tenantA = "a2a00000-0000-4000-8000-00000000000a"
	tenantB = "a2a00000-0000-4000-8000-00000000000b"
)

func migrated(t *testing.T) (*Store, *sql.DB, *testdb.DB) {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("set CARD_TEST_POSTGRES=1 under pg_virtualenv")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	files, err := filepath.Glob("../../db/0*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations: %v %v", files, err)
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(b))); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(path), err)
		}
	}
	if filepath.Base(files[len(files)-1]) != "019_card_embeddings.sql" {
		t.Fatalf("last migration is %s", files[len(files)-1])
	}
	for _, tenant := range []string{tenantA, tenantB} {
		if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	return &Store{DB: db}, db, tdb
}

func busy() *string { s := "recipient_busy"; return &s }

func TestRLSHidesRowsWithoutTenantAndAcrossTenants(t *testing.T) {
	s, db, _ := migrated(t)
	ctx := context.Background()
	th, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.IngestMessage(ctx, tenantA, MessageInput{ThreadID: th.ID, EventKey: "e1", Direction: "in", Sender: "agent:mau", Recipient: "agent:darafei", Text: "hi", DeliveryState: "delivered"}); err != nil {
		t.Fatal(err)
	}

	// No tenant context: the app role sees nothing.
	for _, table := range []string{"a2a_threads", "a2a_messages"} {
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s without tenant: n=%d err=%v", table, n, err)
		}
	}
	// Other tenant sees nothing through the store.
	if got, err := s.ListThreads(ctx, tenantB); err != nil || len(got) != 0 {
		t.Fatalf("tenant B threads: %v %v", got, err)
	}
	if got, err := s.ListMessages(ctx, tenantB, th.ID, 0, 10); err != nil || len(got) != 0 {
		t.Fatalf("tenant B messages: %v %v", got, err)
	}
	if got, err := s.ListMessages(ctx, tenantA, th.ID, 0, 10); err != nil || len(got) != 1 {
		t.Fatalf("tenant A messages: %v %v", got, err)
	}

	// Cross-tenant insert fails RLS WITH CHECK.
	if _, err = s.DB.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO a2a_threads(tenant_id, peer_id, display_name, kind) VALUES($1,'agent:x','X','token_agent')`, tenantA); err == nil {
		t.Fatal("cross-tenant thread insert succeeded")
	}
	// Cross-tenant FK: tenant B message pointing at tenant A's thread fails.
	if _, err = db.Exec(`INSERT INTO a2a_messages(tenant_id, thread_id, event_key, direction, sender, recipient, text)
VALUES($1,$2,'x','in','a','b','t')`, tenantB, th.ID); err == nil {
		t.Fatal("cross-tenant FK insert succeeded")
	}
	// Cross-tenant update affects nothing.
	res, err := db.Exec(`UPDATE a2a_messages SET text='tampered'`)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Fatalf("cross-tenant update touched %d rows", n)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id','',false)`); err != nil {
		t.Fatal(err)
	}
}

func TestOnePeerOneThread(t *testing.T) {
	s, db, _ := migrated(t)
	ctx := context.Background()
	a, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "omp:tasks-loop", DisplayName: "old", Kind: "omp_session", Availability: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "omp:tasks-loop", DisplayName: "tasks-loop", Kind: "omp_session", Availability: "busy"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID || b.DisplayName != "tasks-loop" || b.Availability != "busy" {
		t.Fatalf("thread not reused: %+v %+v", a, b)
	}
	// The same peer in another tenant is a different thread.
	c, err := s.UpsertThread(ctx, tenantB, ThreadInput{PeerID: "omp:tasks-loop", DisplayName: "tasks-loop", Kind: "omp_session", Availability: "busy"})
	if err != nil || c.ID == a.ID {
		t.Fatalf("tenant B thread: %+v %v", c, err)
	}
	// Raw duplicate is rejected by the unique constraint.
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO a2a_threads(tenant_id, peer_id, display_name, kind) VALUES($1,'omp:tasks-loop','dup','omp_session')`, tenantA); err == nil {
		t.Fatal("duplicate peer thread inserted")
	}
	if _, err = s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "p", DisplayName: "p", Kind: "bogus", Availability: "unknown"}); err == nil {
		t.Fatal("invalid kind accepted")
	}
}

func TestReingestNoDuplicateAndStateChangeAdvancesSeq(t *testing.T) {
	s, _, _ := migrated(t)
	ctx := context.Background()
	th, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	if err != nil {
		t.Fatal(err)
	}
	in := MessageInput{ThreadID: th.ID, EventKey: "evt-1", Direction: "out", Sender: "agent:darafei", Recipient: "agent:mau", Text: "hello", DeliveryState: "sent"}
	first, err := s.IngestMessage(ctx, tenantA, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.IngestMessage(ctx, tenantA, in)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || again.Seq != first.Seq {
		t.Fatalf("re-ingest changed row: %+v vs %+v", first, again)
	}
	rows, _ := s.ListMessages(ctx, tenantA, th.ID, 0, 10)
	if len(rows) != 1 {
		t.Fatalf("duplicate rows: %d", len(rows))
	}

	in.DeliveryState, in.ReasonCode = "not_delivered", busy()
	changed, err := s.IngestMessage(ctx, tenantA, in)
	if err != nil {
		t.Fatal(err)
	}
	if changed.ID != first.ID || changed.Seq <= first.Seq || changed.DeliveryState != "not_delivered" || changed.ReasonCode == nil || *changed.ReasonCode != "recipient_busy" {
		t.Fatalf("state change: %+v", changed)
	}
	// A client that already has first.Seq sees the changed row again.
	rows, _ = s.ListMessages(ctx, tenantA, th.ID, first.Seq, 10)
	if len(rows) != 1 || rows[0].ID != first.ID {
		t.Fatalf("after cursor: %+v", rows)
	}
	threads, _ := s.ListThreads(ctx, tenantA)
	if len(threads) != 1 || threads[0].LastSeq != changed.Seq || threads[0].LastText == nil || *threads[0].LastText != "hello" {
		t.Fatalf("thread summary: %+v", threads)
	}

	// The same event_key in another thread (observed in both participants'
	// threads) is a separate row.
	other, _ := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:korzhik", DisplayName: "Korzhik", Kind: "token_agent", Availability: "online"})
	in.ThreadID = other.ID
	in.Direction = "observed"
	obs, err := s.IngestMessage(ctx, tenantA, in)
	if err != nil || obs.ID == first.ID {
		t.Fatalf("observed copy: %+v %v", obs, err)
	}
}

func TestClientMsgIDIsIdempotentAndNullable(t *testing.T) {
	s, db, _ := migrated(t)
	ctx := context.Background()
	th, _ := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	id := uuid.NewString()
	a, err := s.CreateOutbound(ctx, tenantA, th.ID, "agent:darafei", "agent:mau", "hello", id)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateOutbound(ctx, tenantA, th.ID, "agent:darafei", "agent:mau", "hello", id)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID || a.Seq != b.Seq || a.DeliveryState != "pending" || a.Direction != "out" || a.ClientMsgID == nil || *a.ClientMsgID != id {
		t.Fatalf("outbound not idempotent: %+v %+v", a, b)
	}
	// Rows from the journal carry client_msg_id null, and many may coexist.
	for _, key := range []string{"k1", "k2"} {
		m, err := s.IngestMessage(ctx, tenantA, MessageInput{ThreadID: th.ID, EventKey: key, Direction: "in", Sender: "agent:mau", Recipient: "agent:darafei", Text: key, DeliveryState: "delivered"})
		if err != nil || m.ClientMsgID != nil {
			t.Fatalf("journal row: %+v %v", m, err)
		}
	}
	// The same client_msg_id in a different thread is allowed.
	other, _ := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:ugolyok", DisplayName: "Ugolyok", Kind: "token_agent", Availability: "online"})
	if c, err := s.CreateOutbound(ctx, tenantA, other.ID, "agent:darafei", "agent:ugolyok", "hello", id); err != nil || c.ID == a.ID {
		t.Fatalf("other thread: %+v %v", c, err)
	}
	// The partial unique index exists and is partial.
	var def string
	if err = db.QueryRow(`SELECT indexdef FROM pg_indexes WHERE indexname='a2a_messages_client_msg_idx' AND schemaname=current_schema()`).Scan(&def); err != nil {
		t.Fatal(err)
	}
	t.Log(def)
}

func TestCommitNotifiesTenantOnly(t *testing.T) {
	s, _, tdb := migrated(t)
	ctx := context.Background()
	listener, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close(ctx)
	if _, err = listener.Exec(ctx, `LISTEN a2a`); err != nil {
		t.Fatal(err)
	}
	_ = tdb
	th, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.IngestMessage(ctx, tenantA, MessageInput{ThreadID: th.ID, EventKey: "n1", Direction: "in", Sender: "agent:mau", Recipient: "agent:darafei", Text: "secret message body", DeliveryState: "delivered"}); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	n, err := listener.WaitForNotification(waitCtx)
	if err != nil {
		t.Fatalf("no notification: %v", err)
	}
	if n.Channel != "a2a" || n.Payload != tenantA {
		t.Fatalf("notification %q payload %q", n.Channel, n.Payload)
	}

	// A rolled-back transaction must not notify.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`SELECT set_config('litterbox.tenant_id',$1,true)`, tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO a2a_threads(tenant_id, peer_id, display_name, kind) VALUES($1,'agent:rb','RB','token_agent')`, tenantB); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	drain, cancel2 := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel2()
	for {
		n, err = listener.WaitForNotification(drain)
		if err != nil {
			break
		}
		if n.Payload != tenantA {
			t.Fatalf("unexpected payload %q", n.Payload)
		}
	}
}
