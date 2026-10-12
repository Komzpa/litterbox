package gmailsync

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/google/uuid"
)

func TestSyncGrantStatus(t *testing.T) {
 for _, code := range []string{"invalid_grant","invalid_client"} {
  t.Run(code,func(t *testing.T) {
   db,_ := testPool(t); ctx := context.Background(); tenant,account := uuid.New(),uuid.New()
   if _,err := db.Exec(ctx,`INSERT INTO tenants(id) VALUES($1)`,tenant); err != nil {t.Fatal(err)}
   if _,err := db.Exec(ctx,`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'sync@example.test','old')`,tenant,account); err != nil {t.Fatal(err)}
   provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {w.WriteHeader(400); w.Write([]byte(`{"error":"`+code+`"}`))})); defer provider.Close()
   s := &Syncer{DB:db,Client:ClientFactory("fake","fake",provider.URL,provider.URL,provider.Client())}
   err := s.SyncAccount(ctx,Account{TenantID:tenant,ID:account,RefreshToken:[]byte("old")})
   if err==nil || errors.Is(err,ErrInvalidGrant)!=(code=="invalid_grant") {t.Fatalf("error=%v",err)}
   var status string
   if err := db.QueryRow(ctx,`SELECT status FROM accounts WHERE tenant_id=$1 AND id=$2`,tenant,account).Scan(&status); err != nil {t.Fatal(err)}
   want := "active"; if code=="invalid_grant" {want="reauth_required"}
   if status!=want {t.Fatalf("status=%s want=%s",status,want)}
  })
 }
}
