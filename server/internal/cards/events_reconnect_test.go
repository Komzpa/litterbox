package cards

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestEventsReconnectWakesAllSubscribers(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("requires pg_virtualenv")
	}
	db := testdb.Setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsn := db.DSN + " application_name=cards_reconnect_test"
	admin, err := pgx.Connect(ctx, db.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	e := NewEvents(dsn)
	go e.Run(ctx)
	select {
	case <-e.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("LISTEN not ready")
	}
	await := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("subscriber was not woken")
		}
	}
	probe, doneProbe := e.subscribe("probe")
	defer doneProbe()
	// Two notifications establish that the listener has entered its wait loop,
	// even if the initial broadcast raced with the probe subscription.
	for i := 0; i < 2; i++ {
		if _, err = admin.Exec(ctx, `SELECT pg_notify('cards', 'probe')`); err != nil {
			t.Fatal(err)
		}
		await(probe)
	}
	a, doneA := e.subscribe("tenant-a")
	defer doneA()
	b, doneB := e.subscribe("tenant-b")
	defer doneB()
	var pid int
	err = admin.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE application_name='cards_reconnect_test' AND datname=current_database()`).Scan(&pid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `SELECT pg_terminate_backend($1)`, pid); err != nil {
		t.Fatal(err)
	}
	await(a)
	await(b)
}
