package bundles

import (
	"context"
	"os"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// R66: open agent cards without a source collect in one Agent digest bundle;
// a lone agent card stays standalone and mail clustering is unaffected.
func TestClusterCollectsAgentCardsIntoDigest(t *testing.T) {
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
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = conn.Exec(ctx, tdb.Migration(string(b))); e != nil {
			t.Fatalf("migration %s: %v", path, e)
		}
	}
	tenant, account := uuid.New(), uuid.New()
	if _, err = conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'a@example.com','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	mail := uuid.New()
	if _, err = conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,'x@one.test','lone mail')`, tenant, mail, account, mail.String()); err != nil {
		t.Fatal(err)
	}
	agent := func(external string) uuid.UUID {
		id := uuid.New()
		if _, err := conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,title,summary) VALUES($1,$2,'agent',$3,'OMP · result','done')`, tenant, id, external); err != nil {
			t.Fatal(err)
		}
		return id
	}
	bundleOf := func(id uuid.UUID) *uuid.UUID {
		var b *uuid.UUID
		if err := conn.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return b
	}
	embed := fakeEmbedder{"lone mail": {1, 0, 0}}
	first := agent("omp:1")
	if err = Cluster(ctx, conn, tenant, embed); err != nil {
		t.Fatal(err)
	}
	if b := bundleOf(first); b != nil {
		t.Fatalf("a lone agent card was bundled: %v", b)
	}
	second := agent("codex:2")
	if err = Cluster(ctx, conn, tenant, embed); err != nil {
		t.Fatal(err)
	}
	b1, b2 := bundleOf(first), bundleOf(second)
	if b1 == nil || b2 == nil || *b1 != *b2 {
		t.Fatalf("agent cards not collected into one digest: %v %v", b1, b2)
	}
	var title, key string
	if err = conn.QueryRow(ctx, `SELECT title,bundle_key FROM bundles WHERE tenant_id=$1 AND id=$2`, tenant, *b1).Scan(&title, &key); err != nil {
		t.Fatal(err)
	}
	if title != AgentDigestTitle || key != AgentDigestKey {
		t.Fatalf("digest title/key = %q/%q", title, key)
	}
	if b := bundleOf(mail); b != nil {
		t.Fatalf("mail joined the agent digest: %v", b)
	}
	// Archiving one leaves a single agent card: it becomes standalone again.
	if _, err = conn.Exec(ctx, `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND id=$2`, tenant, second); err != nil {
		t.Fatal(err)
	}
	if err = Cluster(ctx, conn, tenant, embed); err != nil {
		t.Fatal(err)
	}
	if b := bundleOf(first); b != nil {
		t.Fatalf("lone remaining agent card stayed in the digest: %v", b)
	}
}
