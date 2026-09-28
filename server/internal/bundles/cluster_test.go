package bundles

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeEmbedder map[string][]float64

func (f fakeEmbedder) Embed(_ context.Context, text string) ([]float64, error) { return f[text], nil }

func TestClusterFakeEmbedderMergesSimilarKeepsTakeOutSeparateAndSplitsTopics(t *testing.T) {
	if os.Getenv("BUNDLE_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with BUNDLE_TEST_POSTGRES=1")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `DROP SCHEMA public CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `DROP ROLE IF EXISTS litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO PUBLIC`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../db/001_mail.sql", "../../db/002_security.sql", "../../db/003_agent_cards.sql", "../../db/008_bundles.sql"} {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = conn.Exec(ctx, string(b)); e != nil {
			t.Fatalf("migration %s: %v", path, e)
		}
	}
	tenant := uuid.New()
	a1, a2, a3 := uuid.New(), uuid.New(), uuid.New()
	if _, err = conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	for _, a := range []uuid.UUID{a1, a2, a3} {
		if _, err = conn.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,$3,'x')`, tenant, a, a.String()+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	subjects := []string{"same topic one", "same topic two", "same topic three", "different topic"}
	accounts := []uuid.UUID{a1, a2, a3, a1}
	senders := []string{"a@one.test", "b@two.test", "a@one.test", "c@three.test"}
	for i, id := range ids {
		if _, err = conn.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,subject) VALUES($1,$2,$3,$4,$5,$6)`, tenant, id, idAccount(accounts[i]), id.String(), senders[i], subjects[i]); err != nil {
			t.Fatal(err)
		}
	}
	vectors := fakeEmbedder{subjects[0]: {1, 0, 0}, subjects[1]: {.99, .01, 0}, subjects[2]: {.98, .02, 0}, subjects[3]: {0, 0, 1}}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = Cluster(ctx, tx, tenant, vectors); err != nil {
		t.Fatal(err)
	}
	var b0, b1, b3 *uuid.UUID
	for i, id := range ids {
		var b *uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&b); err != nil {
			t.Fatal(err)
		}
		switch i {
		case 0:
			b0 = b
		case 1:
			b1 = b
		case 2:
			_ = b
		case 3:
			b3 = b
		}
	}
	if b0 == nil || b1 == nil || *b0 != *b1 {
		t.Fatalf("similar cards not merged: %v %v", b0, b1)
	}
	if b3 != nil {
		t.Fatalf("different topic unexpectedly bundled: %v", b3)
	}
	if err = TakeOut(ctx, tx, tenant, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err = Cluster(ctx, tx, tenant, vectors); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids[:3] {
		var got *uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if i == 0 && got != nil {
			t.Fatal("taken-out card rejoined its bundle")
		}
		if i == 1 && got == nil {
			t.Fatal("similar card from another sender was not kept in a bundle")
		}
		if i == 2 && got != nil {
			t.Fatal("same-sender later card ignored take-out exclusion")
		}
	}
}
func idAccount(id uuid.UUID) uuid.UUID { return id }
