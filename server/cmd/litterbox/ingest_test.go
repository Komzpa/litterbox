package main

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"
 "time"
 "github.com/Komzpa/litterbox/server/internal/ingest"
)

func TestCollectorsUseSharedIngest(t *testing.T) {
 var requests []ingest.Request
 server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if r.URL.Path != "/v1/ingest" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer source-secret" { t.Errorf("unexpected request %s %s",r.Method,r.URL.Path) }
  var card ingest.Request
  if err := json.NewDecoder(r.Body).Decode(&card); err != nil { t.Error(err) }
  requests = append(requests,card)
  w.WriteHeader(http.StatusNoContent)
 }))
 defer server.Close()
 notes := t.TempDir()
 today := time.Now().Format("2006-01-02")
 if err := os.WriteFile(filepath.Join(notes,today+".md"),[]byte("- [ ] 13:42 Timed task\n- [ ] Untimed task\n"),0600); err != nil { t.Fatal(err) }
 var out bytes.Buffer
 if code := RunIngestTodos([]string{"-notes-dir",notes,"-ingest-url",server.URL,"-source-token","source-secret"},&out); code != 0 { t.Fatalf("todo exit=%d %s",code,&out) }
 if len(requests)!=2 || requests[0].Title!="Timed task" || requests[0].At==nil || !requests[0].Timed || requests[1].At!=nil || requests[1].Order!=1 { t.Fatalf("todo projection: %#v",requests) }
 requests=nil
 if code := RunIngestAgents([]string{"-omp-root","../../internal/sources/agents/testdata","-ingest-url",server.URL,"-source-token","source-secret"},&out); code != 0 { t.Fatalf("agent exit=%d %s",code,&out) }
 if len(requests)!=1 || requests[0].Kind!="agent_result" || requests[0].Summary=="" { t.Fatalf("finished results: %#v",requests) }
}
