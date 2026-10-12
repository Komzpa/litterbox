package gmail

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "net/url"
 "strings"
 "testing"

 "github.com/google/uuid"
)

func TestCallbackStatusAndPages(t *testing.T) {
 db := webTestDB(t)
 tenant, account := uuid.New(), uuid.New()
 if _, err := db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil { t.Fatal(err) }
 if _, err := db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token,status) VALUES($1,$2,'owner@example.org','old','reauth_required')`, tenant, account); err != nil { t.Fatal(err) }
 provider := newFakeProvider(t)
 h := &WebHandler{DB:db, Config:WebConfig{Credentials:&Credentials{ClientID:"fake",ClientSecret:"fake"}, CallbackURL:"https://example.test/callback", TokenURL:provider.server.URL+"/token", ProfileURL:provider.server.URL+"/profile", StateSecret:[]byte("12345678901234567890123456789012")}}
 mux := http.NewServeMux(); h.Routes(mux)
 request := func(path string) *httptest.ResponseRecorder {
  r := httptest.NewRequest("GET",path,nil); r = r.WithContext(WithTenant(r.Context(),tenant.String()))
  w := httptest.NewRecorder(); mux.ServeHTTP(w,r); return w
 }
 list := func(want string) {
  t.Helper(); w := request(AccountsPath)
  var accounts []map[string]string
  if err := json.Unmarshal(w.Body.Bytes(),&accounts); err != nil {t.Fatal(err)}
  if w.Code != 200 || len(accounts)!=1 || accounts[0]["status"]!=want {t.Fatalf("account list: %d %s",w.Code,w.Body)}
 }
 list("reauth_required")
 state := func(expired bool) string {
  t.Helper(); s, err := h.newState(tenant); if err != nil {t.Fatal(err)}
  expiry := "now()+interval '10 minutes'"; if expired {expiry="now()-interval '1 minute'"}
  if _, err := db.Exec(`INSERT INTO gmail_oauth_states(tenant_id,state_hash,code_verifier,redirect_uri,expires_at) VALUES($1,$2,'verifier',$3,`+expiry+`)`,tenant,stateHash(s),h.Config.CallbackURL); err != nil {t.Fatal(err)}
  return url.QueryEscape(s)
 }
 check := func(w *httptest.ResponseRecorder, status int, message string) {
  t.Helper()
  for _, part := range []string{"<title>Litterbox</title>","<style>","system-ui, sans-serif",message} {
   if w.Code!=status || !strings.Contains(w.Body.String(),part) {t.Fatalf("page status=%d body=%s missing=%s",w.Code,w.Body,part)}
  }
  if w.Header().Get("Content-Type")!="text/html; charset=utf-8" {t.Fatal(w.Header())}
 }
 success := request(OAuthCallbackPath+"?state="+state(false)+"&code=fake-code")
 check(success,200,"Gmail connected: owner@example.org. You can close this tab and return to Litterbox.")
 t.Logf("success HTML:\n%s",success.Body.String())
 list("active")
 var status string
 if err := db.QueryRow(`SELECT status FROM accounts WHERE tenant_id=$1 AND id=$2`,tenant,account).Scan(&status); err != nil || status!="active" {t.Fatalf("status=%s err=%v",status,err)}
 check(request(OAuthCallbackPath+"?state="+state(true)+"&code=fake-code"),400,"expired OAuth state")
 check(request(OAuthCallbackPath+"?state="+state(false)+"&error=access_denied"),400,"authorization was denied")
 provider.tokenStatus=400; provider.tokenBody=`{"error":"invalid_grant"}`
 check(request(OAuthCallbackPath+"?state="+state(false)+"&code=fake-code"),502,"token exchange failed")
 list("active")
}
