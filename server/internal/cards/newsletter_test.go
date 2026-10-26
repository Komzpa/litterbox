package cards

import (
 "crypto/sha256"
 "database/sql"
 "encoding/json"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/Komzpa/litterbox/server/internal/testdb"
)

// Version 1 synthetic persisted newsletter inputs exercise the real List consumer.
// No owner mailbox export, token, message id or private repair file is required.
func TestNewsletterPersistedList(t *testing.T) {
 if os.Getenv("CARD_TEST_POSTGRES") != "1" { t.Skip("requires pg_virtualenv") }
 const bundleID = "10000000-0000-4000-8000-000000000001"
 const band = `<table role="presentation"><tr><td><p>Catalog label</p></td><td><p>Service label</p></td></tr></table>`
 input := []struct { ID, Subject, Sender, Text, HTML, MessageID string; ReceivedAt time.Time; Want string }{}
 for i, prose := range []string{
  "Compare module prices before choosing the next evaluation board.",
  "Measure sustained throughput with the antenna mounted in its enclosure.",
  "Standalone networks use a dedicated core rather than the legacy anchor.",
 } {
  input = append(input, struct { ID, Subject, Sender, Text, HTML, MessageID string; ReceivedAt time.Time; Want string }{
   ID: []string{"20000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000002", "20000000-0000-4000-8000-000000000003"}[i],
   Subject: "Module field notes", Sender: "newsletter@example.test", Text: "Catalog label\nService label",
   HTML: band + `<table role="presentation"><tr><td><h1>Article heading</h1><p>` + prose + `</p></td></tr></table>`,
   MessageID: []string{"newsletter-v1-1", "newsletter-v1-2", "newsletter-v1-3"}[i],
   ReceivedAt: time.Date(2020, 3, 3-i, 12, 0, 0, 0, time.UTC), Want: prose,
  })
 }
 admin,err:=sql.Open("pgx",os.Getenv("DATABASE_URL"));if err!=nil {t.Fatal(err)};defer admin.Close()
 var database string;if err=admin.QueryRow(`SELECT current_database()`).Scan(&database);err!=nil {t.Fatal(err)}
 if !strings.HasPrefix(database,"litterbox_test_") {t.Fatalf("unsafe database %q",database)}
 td := testdb.Setup(t)
 db, err := sql.Open("pgx", td.DSN); if err != nil { t.Fatal(err) }; defer db.Close()
 db.SetMaxOpenConns(1)
 migrations, err := filepath.Glob("../../db/*.sql"); if err != nil { t.Fatal(err) }
 for _, name := range migrations { body, err := os.ReadFile(name); if err != nil {t.Fatal(err)}; if _,err=db.Exec(td.Migration(string(body))); err!=nil {t.Fatalf("%s: %v",name,err)} }
 const tenant = "30000000-0000-4000-8000-000000000001"
 const account = "40000000-0000-4000-8000-000000000001"
 if _,err=db.Exec(`INSERT INTO tenants(id) VALUES($1)`,tenant);err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'fixture@example.test','fixture')`,tenant,account);err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`INSERT INTO bundles(tenant_id,id,title,centroid,bundle_key,title_origin) VALUES($1,$2,'Module field notes','{}','sender:newsletter@example.test','seed')`,tenant,bundleID);err!=nil {t.Fatal(err)}
 for _, row := range input {
  if _,err=db.Exec(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,subject,sender,sender_name,bundle_id,created_at) VALUES($1,$2,$3,$4,'mail',$5,$6,'Module newsletter',$7,$8)`,tenant,row.ID,account,row.MessageID,row.Subject,row.Sender,bundleID,row.ReceivedAt);err!=nil {t.Fatal(err)}
  if _,err=db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,received_at,text,html,body_hash,labels) VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,'fixture',ARRAY[]::text[])`,tenant,row.ID,row.MessageID,row.ReceivedAt,row.Text,row.HTML);err!=nil {t.Fatal(err)}
 }
 // Guarded provenance repair/rollback stays covered on disposable synthetic data.
 repair,err:=db.Exec(`UPDATE bundles SET title_origin='explicit' WHERE tenant_id=$1 AND id=$2 AND title='Module field notes' AND title_origin='seed'`,tenant,bundleID);if err!=nil {t.Fatal(err)}
 if n,err:=repair.RowsAffected();err!=nil || n!=1 {t.Fatalf("guarded repair: rows=%d err=%v",n,err)}
 control,err:=db.Exec(`UPDATE bundles SET title_origin='topic' WHERE tenant_id=$1 AND id=$2 AND title='Wrong expected title' AND title_origin='explicit'`,tenant,bundleID);if err!=nil {t.Fatal(err)}
 if n,err:=control.RowsAffected();err!=nil || n!=0 {t.Fatalf("mismatched guard changed metadata: rows=%d err=%v",n,err)}
 h := Handler{DB:db,Location:time.UTC}
 req:=httptest.NewRequest("GET","/v1/cards",nil); req=req.WithContext(WithTenant(req.Context(),tenant)); w:=httptest.NewRecorder(); h.List(w,req)
 if w.Code!=200 {t.Fatalf("List: %d %s",w.Code,w.Body.String())}
 var sections Sections; if err=json.Unmarshal(w.Body.Bytes(),&sections);err!=nil {t.Fatal(err)}
 if len(sections.Now)!=3 {t.Fatalf("cards: %+v",sections)}
 expected := map[string]string{}
 for _, row := range input { expected[row.ID] = row.Want }
 for _, c := range sections.Now {
	  t.Logf("card=%s preview_sha256=%x bundle_title=%q",c.ID,sha256.Sum256([]byte(c.Snippet)),c.BundleTitle)
	  if c.Snippet=="" || strings.Contains(c.Snippet,"marketingusercontent") || strings.Contains(c.Snippet,"logo in orange") || strings.HasPrefix(c.Snippet,"[") {t.Errorf("decorative preview on %s",c.ID)}
  if want,ok:=expected[c.ID];!ok || c.Snippet!=want {t.Errorf("article preview on %s: got %q want %q",c.ID,c.Snippet,want)}
  t.Logf("readable_preview=%q",c.Snippet)
  if c.BundleTitle!="Module field notes" {t.Errorf("explicit bundle title: %q",c.BundleTitle)}
 }
 for _, row:=range input {
  var text, body string; if err=db.QueryRow(`SELECT text,html FROM messages WHERE tenant_id=$1 AND card_id=$2`,tenant,row.ID).Scan(&text,&body);err!=nil {t.Fatal(err)}
  if sha256.Sum256([]byte(text))!=sha256.Sum256([]byte(row.Text)) || sha256.Sum256([]byte(body))!=sha256.Sum256([]byte(row.HTML)) {t.Fatalf("full body mutated: %s",row.ID)}
  t.Logf("card=%s text_sha256=%x html_sha256=%x",row.ID,sha256.Sum256([]byte(text)),sha256.Sum256([]byte(body)))
 }
 if _,err=db.Exec(`UPDATE bundles SET title_origin='seed' WHERE tenant_id=$1 AND id=$2 AND title_origin='explicit'`,tenant,bundleID);err!=nil {t.Fatal(err)}
 var restoredTitle,restoredOrigin string
 if err=db.QueryRow(`SELECT title,title_origin FROM bundles WHERE tenant_id=$1 AND id=$2`,tenant,bundleID).Scan(&restoredTitle,&restoredOrigin);err!=nil {t.Fatal(err)}
 if restoredTitle!="Module field notes" || restoredOrigin!="seed" {t.Fatal("rollback changed original metadata")}
 t.Log("three synthetic article previews; full text/HTML hashes unchanged; guarded metadata rollback restored")
}
