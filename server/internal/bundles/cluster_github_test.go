package bundles

// Cluster-level oracle: with a failing embedder, GitHub mail still lands in
// structured repo/actor bundles (never sender: keys) and mentions stay
// standalone; take-out exclusions survive re-clustering. Needs Postgres:
// pg_virtualenv, BUNDLE_TEST_POSTGRES=1.

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type failingEmbedder struct{}

func (failingEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	return nil, errors.New("model nomic-embed-text not found")
}

func TestClusterGitHubSplitWithoutEmbedder(t *testing.T) {
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
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	tenant := uuid.New()
	account := uuid.New()
	if _, err = conn.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO accounts(id,tenant_id,address,refresh_token) VALUES($1,$2,'komzpa@gmail.com','x')`, account, tenant); err != nil {
		t.Fatal(err)
	}
	type fixture struct {
		sender, subject, body string
	}
	fixtures := []fixture{
		{`"chatgpt-codex-connector[bot]" <notifications@github.com>`, "Re: [Komzpa/oh-my-pi] Requirements ledger (PR #39)", codexBotBody},
		{`"coderabbitai[bot]" <notifications@github.com>`, "Re: [Komzpa/oh-my-pi] Requirements ledger (PR #39)", codexBotBody},
		{"Koverstreet <notifications@github.com>", "Re: [koverstreet/ktest] test journal flush SRCU liveness", mentionBody},
		{"Maintainer <notifications@github.com>", "Re: [Soju06/codex-lb] fix(proxy) (PR #2093)", humanOnHisPRBody},
		{`"github-actions[bot]" <notifications@github.com>`, "Re: [bkbilly/lnxlink] Door lock TY0A01 (#44)", subscribedBody},
		{"GitHub <notifications@github.com>", "[konturio] A security advisory on tar affects at least one of your repositories", advisoryBody},
		{"Jane <jane@example.com>", "plain non-github mail", "hello, no footer here"},
	}
	ids := make([]uuid.UUID, len(fixtures))
	for i, f := range fixtures {
		ids[i] = uuid.New()
		if _, err = conn.Exec(ctx, `INSERT INTO cards(id,tenant_id,account_id,gmail_thread_id,source,state,subject,sender,created_at) VALUES($1,$2,$3,$4,'mail','open',$5,$6,now())`, ids[i], tenant, account, "thread-"+ids[i].String(), f.subject, f.sender); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,text,body_hash,received_at) VALUES($1,$2,$3,$4,$5,'\\x00',now())`, tenant, uuid.New(), ids[i], "msg-"+ids[i].String(), f.body); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = Cluster(ctx, tx, tenant, failingEmbedder{}); err != nil {
		t.Fatal(err)
	}

	var senderBundled int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards c JOIN bundles b ON b.id=c.bundle_id WHERE c.tenant_id=$1 AND b.bundle_key LIKE 'sender:%' AND c.sender LIKE '%github.com%'`, tenant).Scan(&senderBundled); err != nil {
		t.Fatal(err)
	}
	if senderBundled != 0 {
		t.Fatalf("%d GitHub cards bundled by sender: key with a failing embedder", senderBundled)
	}

	checks := map[string]string{
		"github-agents:Komzpa/oh-my-pi": "Komzpa/oh-my-pi · bot reviews",
		"github-fyi:bkbilly/lnxlink":    "bkbilly/lnxlink · subscribed",
		"github:konturio":               "konturio · security advisories",
	}
	for key, title := range checks {
		var got string
		if err := tx.QueryRow(ctx, `SELECT title FROM bundles WHERE tenant_id=$1 AND bundle_key=$2`, tenant, key).Scan(&got); err != nil {
			t.Fatalf("bundle %s missing: %v", key, err)
		}
		if got != title {
			t.Fatalf("bundle %s title=%q want %q", key, got, title)
		}
	}
	var agentsCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards c JOIN bundles b ON b.id=c.bundle_id WHERE c.tenant_id=$1 AND b.bundle_key='github-agents:Komzpa/oh-my-pi'`, tenant).Scan(&agentsCount); err != nil {
		t.Fatal(err)
	}
	if agentsCount != 2 {
		t.Fatalf("agents bundle has %d cards, want 2", agentsCount)
	}
	for _, idx := range []int{2, 3} {
		var bundleID *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, ids[idx]).Scan(&bundleID); err != nil {
			t.Fatal(err)
		}
		if bundleID != nil {
			t.Fatalf("card %d (must-answer) bundled: %v", idx, *bundleID)
		}
	}

	if err = TakeOut(ctx, tx, tenant, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err = Cluster(ctx, tx, tenant, failingEmbedder{}); err != nil {
		t.Fatal(err)
	}
	var bundleID *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, ids[0]).Scan(&bundleID); err != nil {
		t.Fatal(err)
	}
	if bundleID != nil {
		t.Fatalf("taken-out card re-bundled into new keys: %v", *bundleID)
	}
	var agentsCountAfter int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards c JOIN bundles b ON b.id=c.bundle_id WHERE c.tenant_id=$1 AND b.bundle_key='github-agents:Komzpa/oh-my-pi'`, tenant).Scan(&agentsCountAfter); err != nil {
		t.Fatal(err)
	}
	if agentsCountAfter != 1 {
		t.Fatalf("agents bundle has %d cards after take-out, want 1", agentsCountAfter)
	}
}
