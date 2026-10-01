package cards

import (
 "crypto/sha256"
 "database/sql"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/Komzpa/litterbox/server/internal/ingest"
 "github.com/Komzpa/litterbox/server/internal/offline"
)

func TestPersistedReminderSemanticContract(t *testing.T) {
 if os.Getenv("CARD_TEST_POSTGRES") != "1" { t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1") }
 db, err := sql.Open("pgx", "")
 if err != nil { t.Fatal(err) }; defer db.Close(); db.SetMaxOpenConns(1)
 if _, err = db.Exec(`SELECT pg_advisory_lock(7809932747080954929); DROP SCHEMA public CASCADE; DROP ROLE IF EXISTS litterbox_app; CREATE SCHEMA public; GRANT USAGE ON SCHEMA public TO PUBLIC`); err != nil { t.Fatal(err) }
 files, err := filepath.Glob("../../db/*.sql"); if err != nil { t.Fatal(err) }
 for _, file := range files { data, err := os.ReadFile(file); if err != nil { t.Fatal(err) }; if _, err = db.Exec(string(data)); err != nil { t.Fatalf("%s: %v", file, err) } }
 tenant := "11111111-1111-4111-8111-111111111111"
 other := "22222222-2222-4222-8222-222222222222"
 hash := sha256.Sum256([]byte("semantic-secret"))
 if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1),($2)`, tenant, other); err != nil { t.Fatal(err) }
 if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,'research',$2)`, tenant, hash[:]); err != nil { t.Fatal(err) }
 if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil { t.Fatal(err) }
 producer := ingest.Handler{DB: db}
 for _, kind := range []string{"reminder", "research_result"} {
  req := httptest.NewRequest("POST", "/v1/ingest", strings.NewReader(`{"external_id":"`+kind+`","kind":"`+kind+`","title":"`+kind+`"}`))
  req.Header.Set("Authorization", "Bearer semantic-secret")
  w := httptest.NewRecorder(); producer.ServeHTTP(w, req)
  if w.Code != http.StatusNoContent { t.Fatalf("ingest %s: %d %s", kind, w.Code, w.Body.String()) }
 }
 handler := Handler{DB: db, Location: time.UTC}
 api := offline.API{DB: db, Identity: func(r *http.Request)(string,bool){ return TenantFrom(r.Context()) }}
 read := func(owner string, path string, serve http.HandlerFunc) map[string]any {
  req := httptest.NewRequest("GET", path, nil); req = req.WithContext(WithTenant(req.Context(), owner))
  w := httptest.NewRecorder(); serve(w, req)
  if w.Code != 200 { t.Fatalf("%s: %d %s", path, w.Code, w.Body.String()) }
  var payload map[string]any; if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
  if path == "/v1/cards" && owner == tenant && os.Getenv("R26_API_PROOF") != "" { if err := os.WriteFile(os.Getenv("R26_API_PROOF"), w.Body.Bytes(), 0600); err != nil { t.Fatal(err) } }
  return payload
 }
 assertKinds := func(rows []any, changes bool) {
  seen := map[string]bool{}
  for _, row := range rows { card := row.(map[string]any); if changes { card = card["value"].(map[string]any) }
   kind, _ := card["source_kind"].(string)
   if card["source"] != "research" { t.Fatalf("token identity changed: %v", card) }; seen[kind] = true
  }
  if !seen["reminder"] || !seen["research_result"] { t.Fatalf("lost persisted semantic kind: %v", rows) }
 }
 assertKinds(read(tenant, "/v1/cards", handler.List)["now"].([]any), false)
 assertKinds(read(tenant, "/v1/snapshot", api.Snapshot)["cards"].([]any), false)
 assertKinds(read(tenant, "/v1/changes?since=0", api.Changes)["changes"].([]any), true)
 for _, item := range []struct{path, key string; serve http.HandlerFunc}{{"/v1/cards","now",handler.List},{"/v1/snapshot","cards",api.Snapshot},{"/v1/changes?since=0","changes",api.Changes}} {
  if rows := read(other,item.path,item.serve)[item.key].([]any); len(rows) != 0 { t.Fatalf("foreign payload leaked: %v", rows) }
 }
}
