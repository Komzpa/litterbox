package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestIngestUpsertCloseAndSourceTokenScope(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" { t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1") }
	db,err:=sql.Open("pgx",""); if err!=nil {t.Fatal(err)}; defer db.Close(); db.SetMaxOpenConns(1)
	for _,p:=range migrations(t,db) { body,e:=os.ReadFile(p); if e!=nil {t.Fatal(e)}; if _,e=db.Exec(string(body));e!=nil {t.Fatalf("migration %s: %v",p,e)} }
	tenant:="11111111-1111-4111-8111-111111111111"; other:="22222222-2222-4222-8222-222222222222"
	if _,err=db.Exec(`INSERT INTO tenants(id) VALUES($1),($2)`,tenant,other);err!=nil {t.Fatal(err)}
	for _,x:=range []struct{tenant,source,token string}{{tenant,"todo","todo-secret"},{other,"todo","other-secret"}} { if _,err=db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,$2,$3)`,x.tenant,x.source,hashToken(x.token));err!=nil {t.Fatal(err)} }
	if _,err=db.Exec(`SET ROLE litterbox_app`);err!=nil {t.Fatal(err)}
	h:=Handler{DB:db}; send:=func(token,body string) int { r:=httptest.NewRequest(http.MethodPost,"/v1/ingest",strings.NewReader(body)); r.Header.Set("Authorization","Bearer "+token); w:=httptest.NewRecorder(); h.ServeHTTP(w,r); return w.Code }
	if got:=send("wrong-secret",`{"external_id":"task-7","kind":"todo","title":"bad"}`);got!=http.StatusUnauthorized {t.Fatalf("bad token: %d",got)}
 if got:=send("todo-secret",`{"external_id":"task-7","source":"ha","kind":"todo","title":"bad"}`);got!=http.StatusBadRequest {t.Fatalf("source spoof: %d",got)}
 payload:=`{"external_id":"task-7","kind":"todo","title":"First","summary":"one","at":"2026-09-28T12:00:00Z"}`
	if got:=send("todo-secret",payload);got!=http.StatusNoContent {t.Fatalf("upsert: %d",got)}
	if got:=send("todo-secret",`{"external_id":"task-7","kind":"todo","title":"Updated","summary":"two"}`);got!=http.StatusNoContent {t.Fatalf("upsert update: %d",got)}
	var id,title,summary string
	if _,err=db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`,tenant);err!=nil {t.Fatal(err)}
	if err=db.QueryRow(`SELECT id::text,title,summary FROM cards WHERE tenant_id=$1 AND source='todo' AND external_id='task-7'`,tenant).Scan(&id,&title,&summary);err!=nil {t.Fatal(err)}
	if title!="Updated"||summary!="two" {t.Fatalf("upsert data: %q %q",title,summary)}
	if got:=send("other-secret",`{"operation":"close","external_id":"task-7"}`);got!=http.StatusNoContent {t.Fatalf("foreign close should be invisible/idempotent, got %d",got)}
	if got:=send("todo-secret",`{"operation":"close","external_id":"task-7"}`);got!=http.StatusNoContent {t.Fatalf("close: %d",got)}
	var state string; if err=db.QueryRow(`SELECT state FROM cards WHERE id=$1`,id).Scan(&state);err!=nil||state!="done" {t.Fatalf("state=%s err=%v",state,err)}
	if got:=send("todo-secret",`{"external_id":"task-7","kind":"todo","title":"Reopened"}`);got!=http.StatusNoContent {t.Fatalf("reopen: %d",got)}
	if err=db.QueryRow(`SELECT state FROM cards WHERE id=$1`,id).Scan(&state);err!=nil||state!="done" {t.Fatalf("re-upsert should preserve dismissal state=%s err=%v",state,err)}
	if _,err=db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`,other);err!=nil {t.Fatal(err)}
	if got:=send("other-secret",`{"external_id":"task-7","kind":"todo","title":"Foreign"}`);got!=http.StatusNoContent {t.Fatalf("other source token ingest: %d",got)}
	var foreign int; if err=db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1 AND external_id='task-7'`,other).Scan(&foreign);err!=nil||foreign!=1 {t.Fatalf("source token tenant scope count=%d err=%v",foreign,err)}
	if _,err=db.Exec(`RESET ROLE`);err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`UPDATE source_tokens SET revoked_at=now() WHERE token_hash=$1`,hashToken("todo-secret"));err!=nil {t.Fatal(err)}
 if got:=send("todo-secret",payload);got!=http.StatusUnauthorized {t.Fatalf("revoked token: %d",got)}
}

func TestDoneCallbackRetries(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" { t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1") }
	db,err:=sql.Open("pgx",""); if err!=nil {t.Fatal(err)}; defer db.Close(); db.SetMaxOpenConns(1)
	for _,p:=range migrations(t,db) { body,e:=os.ReadFile(p); if e!=nil {t.Fatal(e)}; if _,e=db.Exec(string(body));e!=nil {t.Fatalf("migration %s: %v",p,e)} }
	ctx:=context.Background(); tenant:=uuid.MustParse("33333333-3333-4333-8333-333333333333"); card:=uuid.MustParse("44444444-4444-4444-8444-444444444444")
	if _,err=db.Exec(`INSERT INTO tenants(id) VALUES($1)`,tenant);err!=nil {t.Fatal(err)}
	if _,err=db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash,callback_url) VALUES($1,'ha',$2,'http://callback.invalid/action')`,tenant,hashToken("ha-token"));err!=nil {t.Fatal(err)}
	if _,err=db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title,state) VALUES($1,$2,'ha','notification-9','Low battery','done')`,tenant,card);err!=nil {t.Fatal(err)}
	agentCard:=uuid.MustParse("66666666-6666-4666-8666-666666666666")
 if _,err=db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash,callback_url) VALUES($1,'agent',$2,'http://callback.invalid/action')`,tenant,hashToken("agent-token"));err!=nil {t.Fatal(err)}
 if _,err=db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title,state) VALUES($1,$2,'agent','result-9','Research','done')`,tenant,agentCard);err!=nil {t.Fatal(err)}
 conn,err:=pgx.Connect(ctx,"");if err!=nil {t.Fatal(err)};defer conn.Close(ctx)
	tx,err:=conn.Begin(ctx);if err!=nil {t.Fatal(err)}
	if _,err=tx.Exec(ctx,`SELECT set_config('litterbox.tenant_id',$1,true)`,tenant.String());err!=nil {t.Fatal(err)}
	if err=OnDone(ctx,tx,tenant,agentCard);err!=nil {t.Fatal(err)}
 if err=OnDone(ctx,tx,tenant,card);err!=nil {t.Fatal(err)}; if err=tx.Commit(ctx);err!=nil {t.Fatal(err)}
	var queued int
 if err=db.QueryRow(`SELECT count(*) FROM source_action_callbacks WHERE tenant_id=$1`,tenant).Scan(&queued);err!=nil||queued!=1 {t.Fatalf("only HA should callback: count=%d err=%v",queued,err)}
 calls:=0; server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ calls++; var action map[string]any; if json.NewDecoder(r.Body).Decode(&action)!=nil||action["type"]!="done" {t.Errorf("unexpected action: %#v",action)}; if calls==1 {w.WriteHeader(http.StatusServiceUnavailable)} else {w.WriteHeader(http.StatusNoContent)} }));defer server.Close()
	if _,err=db.Exec(`UPDATE source_action_callbacks SET callback_url=$1,next_attempt_at=now() WHERE tenant_id=$2`,server.URL,tenant);err!=nil {t.Fatal(err)}
	if processed,err:=DispatchOne(ctx,db,server.Client());err!=nil||!processed {t.Fatalf("first dispatch processed=%v err=%v",processed,err)}
	if calls!=1 {t.Fatalf("first call count=%d",calls)}
	if _,err=db.Exec(`UPDATE source_action_callbacks SET next_attempt_at=now() WHERE tenant_id=$1`,tenant);err!=nil {t.Fatal(err)}
	if processed,err:=DispatchOne(ctx,db,server.Client());err!=nil||!processed {t.Fatalf("retry processed=%v err=%v",processed,err)}
	var delivered time.Time; if err=db.QueryRow(`SELECT delivered_at FROM source_action_callbacks WHERE tenant_id=$1`,tenant).Scan(&delivered);err!=nil||delivered.IsZero()||calls!=2 {t.Fatalf("delivered=%v calls=%d err=%v",delivered,calls,err)}
}

func migrations(t *testing.T, db *sql.DB) []string { t.Helper(); var exists bool; if err:=db.QueryRow(`SELECT to_regclass('cards') IS NOT NULL`).Scan(&exists);err!=nil {t.Fatal(err)}; if exists {return nil}; paths,err:=filepath.Glob("../../db/00*.sql");if err!=nil||len(paths)==0 {t.Fatalf("migration discovery: %v",err)};return paths }
