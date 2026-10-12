package gmailsync

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/Komzpa/litterbox/server/internal/ops"
 "github.com/google/uuid"
 "github.com/jackc/pgx/v5"
)

func TestArchiveGrantError(t *testing.T) {
 for _, kind := range []string{"archive", "bundle_archive"} {
  for _, grant := range []bool{true, false} {
   name := kind + "/non_grant"
   if grant { name = kind + "/invalid_grant" }
   t.Run(name, func(t *testing.T) {
    db, _ := testPool(t)
    ctx := context.Background()
    tenant, account, bundle, first, second, op := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
    device := uuid.New()
    for _, query := range []struct {sql string; args []any}{
     {`INSERT INTO tenants(id) VALUES($1)`, []any{tenant}},
     {`INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, []any{tenant,device,[]byte("fixture-device")}},
     {`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'expired@example.test',$3)`, []any{tenant,account,[]byte("fixture")}},
     {`INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Fixture','{0}')`, []any{tenant,bundle}},
     {`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,bundle_id) VALUES($1,$2,$3,'first','mail',$4),($1,$5,$3,'second','mail',$4)`, []any{tenant,first,account,bundle,second}},
    } { if _, err := db.Exec(ctx, query.sql, query.args...); err != nil { t.Fatal(err) } }
    calls := 0
    token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
     calls++
     if r.URL.Path != "/token" { t.Errorf("Gmail API must not run after token failure: %s",r.URL); http.NotFound(w,r); return }
     code := "invalid_client"; if grant { code = "invalid_grant" }
     w.WriteHeader(400)
     json.NewEncoder(w).Encode(map[string]string{"error":code})
    }))
    defer token.Close()
    useTestOps(func(ctx context.Context,tx pgx.Tx,tenant,card uuid.UUID)(*Client,string,error) {
     var thread string
     err := tx.QueryRow(ctx,`SELECT gmail_thread_id FROM cards WHERE tenant_id=$1 AND id=$2`,tenant,card).Scan(&thread)
     return &Client{HTTP:token.Client(),TokenURL:token.URL+"/token",APIBase:token.URL,RefreshToken:"fixture"},thread,err
    })
    api := ops.API{DB:db,Identity:func(context.Context)(string,string,bool){return tenant.String(),device.String(),true}}
    payload,_ := json.Marshal(map[string]any{"op_id":op,"card_id":first,"type":kind,"args":map[string]any{"bundle_id":bundle}})
    w := httptest.NewRecorder()
    api.ServeHTTP(w,httptest.NewRequest("POST","/v1/ops",strings.NewReader(string(payload))))
    if w.Code != 422 { t.Fatalf("status=%d body=%s",w.Code,w.Body) }
    var response map[string]string
    if err := json.Unmarshal(w.Body.Bytes(),&response);err != nil {t.Fatal(err)}
    if grant {
     if response["code"] != "gmail_reconnect_required" || response["account_address"] != "expired@example.test" {t.Fatalf("response=%v",response)}
    } else if response["code"] != "" || !strings.Contains(response["error"],"invalid_client") {t.Fatalf("generic response changed: %v",response)}
    var open, recorded int
    var status string
    if err := db.QueryRow(ctx,`SELECT count(*) FROM cards WHERE tenant_id=$1 AND state='open'`,tenant).Scan(&open);err != nil {t.Fatal(err)}
    if err := db.QueryRow(ctx,`SELECT count(*) FROM ops WHERE tenant_id=$1`,tenant).Scan(&recorded);err != nil {t.Fatal(err)}
    if err := db.QueryRow(ctx,`SELECT status FROM accounts WHERE tenant_id=$1 AND id=$2`,tenant,account).Scan(&status);err != nil {t.Fatal(err)}
    wantStatus := "active"; if grant {wantStatus = "reconnect_required"}
    if open != 2 || recorded != 0 || status != wantStatus || calls != 1 {t.Fatalf("open=%d ops=%d health=%s token calls=%d",open,recorded,status,calls)}
   })
  }
 }
}
