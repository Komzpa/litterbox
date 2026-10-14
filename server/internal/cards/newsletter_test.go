package cards

import (
 "crypto/sha256"
 "database/sql"
 "encoding/json"
 "fmt"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/Komzpa/litterbox/server/internal/testdb"
)

// The private immutable input is exported with tenant-scoped read-only SQL.
// Only disposable copies become open; production archived states are untouched.
func TestNewsletterPersistedList(t *testing.T) {
 path := os.Getenv("TECHSHIP_SOURCE_SNAPSHOT")
 if path == "" || os.Getenv("CARD_TEST_POSTGRES") != "1" { t.Skip("requires private source snapshot and pg_virtualenv") }
 raw, err := os.ReadFile(path); if err != nil { t.Fatal(err) }
 var input []struct { ID, Subject, Sender, Text, HTML, State string; SenderName string `json:"sender_name"`; BundleID string `json:"bundle_id"`; BundleTitle string `json:"bundle_title"`; MessageID string `json:"message_id"`; ReceivedAt time.Time `json:"received_at"` }
 if err := json.Unmarshal(raw, &input); err != nil { t.Fatal(err) }
 if len(input) != 3 { t.Fatalf("expected three immutable cards, got %d",len(input)) }
 admin,err:=sql.Open("pgx",os.Getenv("DATABASE_URL"));if err!=nil {t.Fatal(err)};defer admin.Close()
 var database string;if err=admin.QueryRow(`SELECT current_database()`).Scan(&database);err!=nil {t.Fatal(err)}
 if !strings.HasPrefix(database,"litterbox_test_") {t.Fatalf("unsafe database %q",database)}
 td := testdb.Setup(t)
 db, err := sql.Open("pgx", td.DSN); if err != nil { t.Fatal(err) }; defer db.Close()
 db.SetMaxOpenConns(1)
 migrations, err := filepath.Glob("../../db/*.sql"); if err != nil { t.Fatal(err) }
 for _, name := range migrations { body, err := os.ReadFile(name); if err != nil {t.Fatal(err)}; if _,err=db.Exec(td.Migration(string(body))); err!=nil {t.Fatalf("%s: %v",name,err)} }
 const tenant = "3b09aac5-be4b-43d8-abca-f7b1db66f8bc"
 const account = "e09758a2-90f1-4add-88f1-78036c8fefa6"
 if _,err=db.Exec(`INSERT INTO tenants(id) VALUES($1)`,tenant);err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'fixture@example.test','fixture')`,tenant,account);err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`INSERT INTO bundles(tenant_id,id,title,centroid,bundle_key) VALUES($1,$2,$3,'{}','semantic:30123be7-9327-4733-89a6-df69dc4569d4')`,tenant,input[0].BundleID,input[0].BundleTitle);err!=nil {t.Fatal(err)}
 for _, row := range input {
  if row.State != "archived" { t.Fatalf("snapshot state changed: %s", row.ID) }
  if _,err=db.Exec(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,subject,sender,sender_name,bundle_id,created_at) VALUES($1,$2,$3,$4,'mail',$5,$6,$7,$8,$9)`,tenant,row.ID,account,row.MessageID,row.Subject,row.Sender,row.SenderName,row.BundleID,row.ReceivedAt);err!=nil {t.Fatal(err)}
  if _,err=db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,received_at,text,html,body_hash,labels) VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,'fixture',ARRAY[]::text[])`,tenant,row.ID,row.MessageID,row.ReceivedAt,row.Text,row.HTML);err!=nil {t.Fatal(err)}
 }
 // Rehearse the separately authorized legacy metadata repair, not a heuristic
 // backfill. The immutable body/state snapshot remains unchanged.
 provenancePath := os.Getenv("TECHSHIP_TITLE_PROVENANCE")
 provenance, err := os.ReadFile(provenancePath); if err != nil { t.Fatal(err) }
 var origin struct { BundleID string `json:"bundle_id"`; BundleKey string `json:"bundle_key"`; OldTitle string `json:"old_title"`; OldOrigin string `json:"old_origin"`; NewOrigin string `json:"new_origin"`; SnapshotSHA string `json:"snapshot_sha256"` }
 if err=json.Unmarshal(provenance,&origin);err!=nil {t.Fatal(err)}
 if origin.SnapshotSHA != fmt.Sprintf("%x",sha256.Sum256(raw)) {t.Fatal("provenance snapshot mismatch")}
 t.Logf("snapshot_sha256=%x provenance_sha256=%x",sha256.Sum256(raw),sha256.Sum256(provenance))
 repair,err:=os.ReadFile(os.Getenv("TECHSHIP_TITLE_REPAIR_SQL"));if err!=nil {t.Fatal(err)}
 rollback,err:=os.ReadFile(os.Getenv("TECHSHIP_TITLE_ROLLBACK_SQL"));if err!=nil {t.Fatal(err)}
 if _,err=db.Exec(string(repair));err!=nil {t.Fatal(err)}
 t.Logf("repair_sha256=%x rollback_sha256=%x",sha256.Sum256(repair),sha256.Sum256(rollback))
 h := Handler{DB:db,Location:time.UTC}
 req:=httptest.NewRequest("GET","/v1/cards",nil); req=req.WithContext(WithTenant(req.Context(),tenant)); w:=httptest.NewRecorder(); h.List(w,req)
 if w.Code!=200 {t.Fatalf("List: %d %s",w.Code,w.Body.String())}
 var sections Sections; if err=json.Unmarshal(w.Body.Bytes(),&sections);err!=nil {t.Fatal(err)}
 if len(sections.Now)!=3 {t.Fatalf("cards: %+v",sections)}
 for _, c := range sections.Now {
	  t.Logf("card=%s preview_sha256=%x bundle_title=%q",c.ID,sha256.Sum256([]byte(c.Snippet)),c.BundleTitle)
	  if c.Snippet=="" || strings.Contains(c.Snippet,"marketingusercontent") || strings.Contains(c.Snippet,"logo in orange") || strings.HasPrefix(c.Snippet,"[") {t.Errorf("decorative preview on %s",c.ID)}
	  if !strings.Contains(c.Snippet,"cellular") && !strings.Contains(c.Snippet,"5G") {t.Errorf("newsletter topic missing on %s",c.ID)}
	  if !strings.Contains(c.Snippet,"marketingusercontent") {t.Logf("readable_preview=%.180s",c.Snippet)}
  if c.BundleTitle=="" || strings.Contains(c.BundleTitle,"September 23") || c.BundleTitle==c.Title {t.Errorf("seeded bundle title: %q",c.BundleTitle)}
 }
 for _, row:=range input {
  var text, body string; if err=db.QueryRow(`SELECT text,html FROM messages WHERE tenant_id=$1 AND card_id=$2`,tenant,row.ID).Scan(&text,&body);err!=nil {t.Fatal(err)}
  if sha256.Sum256([]byte(text))!=sha256.Sum256([]byte(row.Text)) || sha256.Sum256([]byte(body))!=sha256.Sum256([]byte(row.HTML)) {t.Fatalf("full body mutated: %s",row.ID)}
 }
 if _,err=db.Exec(string(rollback));err!=nil {t.Fatal(err)}
 var restoredTitle,restoredOrigin string
 if err=db.QueryRow(`SELECT title,title_origin FROM bundles WHERE tenant_id=$1 AND id=$2`,tenant,origin.BundleID).Scan(&restoredTitle,&restoredOrigin);err!=nil {t.Fatal(err)}
 if restoredTitle!=origin.OldTitle || restoredOrigin!=origin.OldOrigin {t.Fatal("rollback changed original metadata")}
 t.Log("all three full text/HTML hashes unchanged; exact private metadata rollback restored original title/origin")
}
