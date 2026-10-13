package ingest

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	go func() { done <- (&Handler{Pool: pool}).afterIngest(requestCtx, tenant.String()) }()
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

// An authenticated upsert must finish while Ollama is paused, without canceling
// the clustering pass when the producer disconnects.
func TestIngestReturnsWhileEmbeddingPaused(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, path := range migrations(t) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'test@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	for _, thread := range []string{"one", "two"} {
		if _, err := db.Exec(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,'test@example.test','Paused embedding')`, tenant, uuid.New(), account, thread); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,'todo',$2)`, tenant, hashToken("paused-test-token")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	entered := make(chan struct{}, 2)
	release := make(chan struct{}, 2)
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[1,0]]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLAMA_HOST", ollama.URL)
	h := &Handler{DB: db, Pool: pool}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(`{"external_id":"paused-upsert","kind":"todo","title":"Committed while paused"}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer paused-test-token")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	start := time.Now()
	go func() {
		h.ServeHTTP(w, r)
		close(done)
	}()
	// Always release the server and drain the handler, including on baseline failure.
	defer func() {
		close(release)
		<-done
		shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := h.Shutdown(shutdownCtx); err != nil {
			t.Errorf("drain clustering at cleanup: %v", err)
		}
	}()
	select {
	case <-done:
		if w.Code != http.StatusNoContent {
			t.Fatalf("ingest status=%d, want 204", w.Code)
		}
		t.Logf("ingest returned 204 in %s while Ollama paused", time.Since(start))
	case <-time.After(time.Second):
		t.Fatal("ingest did not return within 1s while Ollama paused")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("embedding was not called")
	}
	cancel() // HTTP completion/disconnect must not cancel the background job.
	var stored int
	if err := db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1 AND external_id='paused-upsert'`, tenant).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("committed card count=%d err=%v", stored, err)
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if err := h.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown while embedding paused: %v, want deadline exceeded", err)
	}
	// Allow both embeddings to resume without closing the channel twice at cleanup.
	release <- struct{}{}
	release <- struct{}{}
	drainCtx, stopDrain := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopDrain()
	if err := h.Shutdown(drainCtx); err != nil {
		t.Fatalf("shutdown after Ollama resumed: %v", err)
	}
	var members, groups int
	if err := db.QueryRow(`SELECT count(bundle_id),count(DISTINCT bundle_id) FROM cards WHERE tenant_id=$1 AND source='mail'`, tenant).Scan(&members, &groups); err != nil {
		t.Fatal(err)
	}
	if members != 2 || groups != 1 {
		t.Fatalf("cards not clustered after Ollama resumed: members=%d groups=%d", members, groups)
	}
	t.Log("mail cards clustered after Ollama resumed despite request cancellation; shutdown drained")
}
