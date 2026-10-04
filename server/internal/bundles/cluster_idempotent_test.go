package bundles

import (
	"context"
	"os"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A recluster that changes nothing must write no changes rows. The prod
// Keeper Security pair fanned out thousands of changes rows because every
// Cluster run rewrote bundle_id unconditionally, even when the assignment was
// identical to the stored one.
func TestClusterSecondRunWritesNoChangesRows(t *testing.T) {
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
	for _, path := range []string{"../../db/001_mail.sql", "../../db/002_security.sql", "../../db/003_agent_cards.sql", "../../db/008_bundles.sql", "../../db/007_offline_ops.sql"} {
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
	if _, err = conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'keeper@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	subjects := []string{"Keeper alert one", "Keeper alert two"}
	for i, id := range ids {
		if _, err = conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,$5,$6)`, tenant, id, account, id.String(), "Keeper Security <no-reply@keeper.test>", subjects[i]); err != nil {
			t.Fatal(err)
		}
	}
	vectors := fakeEmbedder{subjects[0]: {1, 0, 0}, subjects[1]: {.99, .01, 0}}
	run := func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err = Cluster(ctx, tx, tenant, vectors); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	changes := func() int {
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM changes WHERE tenant_id=$1`, tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	bundles := func() []*uuid.UUID {
		got := make([]*uuid.UUID, len(ids))
		for i, id := range ids {
			if err := conn.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&got[i]); err != nil {
				t.Fatal(err)
			}
		}
		return got
	}
	run()
	first := bundles()
	if first[0] == nil || first[1] == nil || *first[0] != *first[1] {
		t.Fatalf("similar pair not bundled after first run: %v %v", first[0], first[1])
	}
	afterFirst := changes()
	run()
	if got := changes(); got != afterFirst {
		t.Fatalf("second identical Cluster run wrote %d changes rows, want 0", got-afterFirst)
	}
	for i, b := range bundles() {
		if b == nil || *b != *first[i] {
			t.Fatalf("card %d bundle changed across identical runs", i)
		}
	}
}
