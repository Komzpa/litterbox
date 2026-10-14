package bundles

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type countingEmbedder struct {
	model string
	calls int
}

func (e *countingEmbedder) ModelName() string { return e.model }

func (e *countingEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	e.calls++
	return []float64{1, 0, 0}, nil
}

func TestClusterEmbedsOnlyNewOrChangedCards(t *testing.T) {
	if os.Getenv("BUNDLE_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with BUNDLE_TEST_POSTGRES=1")
	}
	tdb := testdb.Setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, path := range []string{"../../db/001_mail.sql", "../../db/002_security.sql", "../../db/003_agent_cards.sql", "../../db/008_bundles.sql", "../../db/019_card_embeddings.sql", "../../db/020_bundle_title_origin.sql"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, tdb.Migration(string(b))); err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	tenant, account := uuid.New(), uuid.New()
	if _, err := conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'cache@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		if _, err := conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,'cache@example.test','Cache topic')`, tenant, id, account, id.String()); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(id.String()))
		if _, err := conn.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,body_hash,text,received_at) VALUES($1,$2,$3,$4,$5,'original body',now())`, tenant, uuid.New(), id, id.String(), hash[:]); err != nil {
			t.Fatal(err)
		}
	}
	embedder := &countingEmbedder{model: "test-model"}
	run := func(label string, want int) {
		t.Helper()
		embedder.calls = 0
		if err := Cluster(ctx, conn, tenant, embedder); err != nil {
			t.Fatal(err)
		}
		if embedder.calls != want {
			t.Fatalf("%s: embed calls=%d, want %d", label, embedder.calls, want)
		}
		t.Logf("%s: embed calls=%d", label, embedder.calls)
	}
	run("first Cluster", len(ids))
	run("unchanged Cluster", 0)
	if _, err := conn.Exec(ctx, `UPDATE messages SET text='changed body' WHERE tenant_id=$1 AND card_id=$2`, tenant, ids[0]); err != nil {
		t.Fatal(err)
	}
	run("one changed body", 1)
	// Use a fresh instance: reuse must come from PostgreSQL, not embedder memory.
	embedder = &countingEmbedder{model: "test-model"}
	run("fresh embedder unchanged Cluster", 0)
	embedder.model = "other-model"
	run("different model", len(ids))
}
