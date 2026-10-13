package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pause the real Ollama HTTP path while observing its database connections.
// A slow or canceled embedding request must not retain a transaction or locks.
func TestAfterIngestDoesNotHoldTransactionDuringEmbedding(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	tdb := testdb.Setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, path := range migrations(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, tdb.Migration(string(b))); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	tenant, account, card := uuid.New(), uuid.New(), uuid.New()
	if _, err := conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'test@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,'thread','test@example.test','Paused embedding')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	name := "paused-embedding-" + uuid.NewString()
	config.ConnConfig.RuntimeParams["application_name"] = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[1,0]]}`))
	}))
	defer server.Close()
	defer close(release)
	t.Setenv("OLLAMA_HOST", server.URL)
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- (Handler{Pool: pool}).afterIngest(requestCtx, tenant.String()) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("hook returned before embedding: %v", err)
	case <-requestCtx.Done():
		t.Fatal("embedding was not called")
	}
	var open int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND xact_start IS NOT NULL`, name).Scan(&open); err != nil {
		t.Fatal(err)
	}
	// Cancel while paused to exercise the error path as well as the slow path.
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("canceled hook succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled hook did not return")
	}
	if open != 0 {
		t.Errorf("embedding held %d open transaction(s), want 0", open)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND xact_start IS NOT NULL`, name).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 0 {
		t.Errorf("canceled hook leaked %d transaction(s)", open)
	}
}
