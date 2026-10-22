package cards

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// A legacy card has an empty cards.sender_address (added by migration 022
// with default ”). Its sender photo must still be flagged from the latest
// message sender, the same sender the sender-photo endpoint serves.
func TestSenderPhotoFlagUsesLatestMessageSenderForLegacyCards(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	account := "44444444-4444-4444-8444-444444444444"
	card := "55555555-5555-4555-8555-555555555555"
	message := "66666666-6666-4666-8666-666666666666"
	ctx := context.Background()
	if _, err = db.ExecContext(ctx, `INSERT INTO tenants(id) VALUES ($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES ($1,$2,'legacy@example.test',$3)`, tenant, account, []byte("refresh")); err != nil {
		t.Fatal(err)
	}
	// Legacy row: sender_address keeps the migration default ''.
	if _, err = db.ExecContext(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES ($1,$2,$3,'legacy-thread','Old mail','Maumap <hi@maumap.com>')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at,sender_name,sender_address) VALUES ($1,$2,$3,'legacy-msg','{}','','','\x00',now(),'Maumap','hi@maumap.com')`, tenant, message, card); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO sender_photos(tenant_id,account_id,sender_address,photo,content_type) VALUES ($1,$2,'hi@maumap.com',$3,'image/jpeg')`, tenant, account, []byte{0xFF, 0xD8, 0xFF, 0xE0}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		t.Fatal(err)
	}
	all := []Card{{ID: card}}
	markSenderPhotos(ctx, tx, tenant, all)
	if !all[0].SenderHasPhoto {
		t.Fatalf("legacy card with empty sender_address: SenderHasPhoto=false, want true (latest message sender has a stored photo)")
	}
}
