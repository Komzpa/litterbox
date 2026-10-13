package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func bridgeFixture(t *testing.T) (*Store, http.Handler) {
	t.Helper()
	s, db, td := migrated(t)
	if _, err := db.Exec(`RESET ROLE`); err != nil { t.Fatal(err) }
	for _, entry := range []struct{ tenant, source, token string }{{tenantA,"a2a","bridge-a"},{tenantB,"a2a","bridge-b"},{tenantA,"todos","other-source"}} {
		h := sha256.Sum256([]byte(entry.token))
		if _, err := db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,$2,$3)`, entry.tenant,entry.source,h[:]); err != nil { t.Fatal(err) }
	}
	h := sha256.Sum256([]byte("device-a"))
	if _, err := db.Exec(`INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`,tenantA,uuid.NewString(),h[:]); err != nil { t.Fatal(err) }
	if _, err := db.Exec(`SET ROLE `+td.Role); err != nil { t.Fatal(err) }
	config, err := pgxpool.ParseConfig(td.DSN)
	if err != nil { t.Fatal(err) }
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE `+td.Role)
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil { t.Fatal(err) }
	t.Cleanup(pool.Close)
	bridge := NewBridgeHandler(pool).(*BridgeHandler)
	t.Cleanup(func() { bridge.Store.DB.Close() })
	return s, auth.NewHandler(auth.NewPostgresStore(pool), "").Middleware(bridge)
}

func bridgeCall(t *testing.T, h http.Handler, method, path, token string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil { t.Fatal(err) }
	r := httptest.NewRequest(method,path,strings.NewReader(string(b)))
	if token != "" { r.Header.Set("Authorization","Bearer "+token) }
	w := httptest.NewRecorder()
	h.ServeHTTP(w,r)
	if w.Code != want { t.Fatalf("%s %s: status=%d body=%s want=%d",method,path,w.Code,w.Body.String(),want) }
	return w
}

func ingestEvents(t *testing.T, h http.Handler, events ...JournalEvent) {
	t.Helper()
	bridgeCall(t,h,"POST","/v1/a2a/ingest","bridge-a",map[string]any{"events":events},204)
}

func TestBridgeSourceAuthentication(t *testing.T) {
	_, h := bridgeFixture(t)
	for _, token := range []string{"","device-a","other-source","invalid"} {
		for _, path := range []string{"/v1/a2a/ingest","/v1/a2a/outbox","/v1/a2a/outbox/"+uuid.NewString()+"/result"} {
			bridgeCall(t,h,"POST",path,token,map[string]any{},401)
		}
	}
	bridgeCall(t,h,"GET","/v1/a2a/outbox","bridge-a",nil,200)
	bridgeCall(t,h,"POST","/v1/a2a/ingest","bridge-a",map[string]any{"events":[]JournalEvent{}},204)
	// Neighboring routes still require a device token; exemption is bounded.
	bridgeCall(t,h,"GET","/v1/a2a/threads","bridge-a",nil,401)
	bridgeCall(t,h,"GET","/v1/cards","bridge-a",nil,401)
}

func TestBridgeIngestObservedAndAvailability(t *testing.T) {
	s,h := bridgeFixture(t)
	text := "Mau asks Korzhik"
	e := JournalEvent{EventKey:"sent-observed",Kind:"sent",Sender:"agent:mau",Recipient:"agent:korzhik",RequestID:"observed-1",Text:&text}
	body := map[string]any{"events":[]JournalEvent{e},"recipients":[]BridgeRecipient{{"agent:mau","Mau","token_agent","online"},{"agent:korzhik","Korzhik","token_agent","busy"}}}
	bridgeCall(t,h,"POST","/v1/a2a/ingest","bridge-a",body,204)
	bridgeCall(t,h,"POST","/v1/a2a/ingest","bridge-a",body,204)
	threads,err := s.ListThreads(context.Background(),tenantA)
	if err != nil || len(threads)!=2 { t.Fatalf("threads=%v err=%v",threads,err) }
	for _, th := range threads {
		ms,err := s.ListMessages(context.Background(),tenantA,th.ID,0,200)
		if err != nil || len(ms)!=1 || ms[0].Direction!="observed" || ms[0].Text!=text { t.Fatalf("messages=%v err=%v",ms,err) }
		if th.PeerID=="agent:korzhik" && th.Availability!="busy" { t.Fatalf("availability=%s",th.Availability) }
	}
}

func TestBridgeOutboxBusyReplyAndTenantIsolation(t *testing.T) {
	s,h := bridgeFixture(t)
	ctx := context.Background()
	th,err := s.UpsertThread(ctx,tenantA,ThreadInput{"omp:busy","Busy","omp_session","busy"})
	if err != nil { t.Fatal(err) }
	client := uuid.NewString()
	out,err := s.CreateOutbound(ctx,tenantA,th.ID,humanPeer,th.PeerID,"hello",client)
	if err != nil { t.Fatal(err) }
	w := bridgeCall(t,h,"GET","/v1/a2a/outbox","bridge-a",nil,200)
	var box struct { Messages []Message `json:"messages"` }
	if err := json.Unmarshal(w.Body.Bytes(),&box); err!=nil || len(box.Messages)!=1 || box.Messages[0].ID!=out.ID { t.Fatalf("outbox=%s err=%v",w.Body.String(),err) }
	bridgeCall(t,h,"POST","/v1/a2a/outbox/"+out.ID+"/result","bridge-b",map[string]any{"status":"failed","reason_code":"recipient_busy"},404)
	bridgeCall(t,h,"POST","/v1/a2a/outbox/"+out.ID+"/result","bridge-a",map[string]any{"status":"failed","reason_code":"recipient_busy"},200)
	ms,err := s.ListMessages(ctx,tenantA,th.ID,0,200)
	if err!=nil || len(ms)!=1 || ms[0].DeliveryState!="not_delivered" || ms[0].ReasonCode==nil || *ms[0].ReasonCode!="recipient_busy" { t.Fatalf("busy=%v err=%v",ms,err) }
	// An old sent event replay cannot undo the failed receipt or duplicate it.
	text := "hello"
	ingestEvents(t,h,JournalEvent{EventKey:"busy-sent",Kind:"sent",Sender:humanPeer,Recipient:th.PeerID,RequestID:"litterbox-"+client,Text:&text})
	ms,err = s.ListMessages(ctx,tenantA,th.ID,0,200)
	if err!=nil || len(ms)!=1 || ms[0].DeliveryState!="not_delivered" { t.Fatalf("replay=%v err=%v",ms,err) }

	mau,err := s.UpsertThread(ctx,tenantA,ThreadInput{"agent:mau","Mau","token_agent","online"})
	if err != nil { t.Fatal(err) }
	client2 := uuid.NewString()
	out2,err := s.CreateOutbound(ctx,tenantA,mau.ID,humanPeer,mau.PeerID,"hello",client2)
	if err != nil { t.Fatal(err) }
	req := "litterbox-"+client2
	sent := JournalEvent{EventKey:"mau-sent",Kind:"sent",Sender:humanPeer,Recipient:mau.PeerID,RequestID:req,Text:&text}
	replyText := "hi Darafei"
	reply := JournalEvent{EventKey:"mau-reply",Kind:"reply",Sender:mau.PeerID,Recipient:humanPeer,RequestID:req,Text:&replyText,Status:"completed"}
	ingestEvents(t,h,sent,reply)
	ingestEvents(t,h,sent,reply)
	ms,err = s.ListMessages(ctx,tenantA,mau.ID,0,200)
	if err!=nil || len(ms)!=2 || ms[0].ID!=out2.ID || ms[0].DeliveryState!="replied" || ms[1].Direction!="in" || ms[1].Text!=replyText { t.Fatalf("reply=%v err=%v",ms,err) }
	bridgeCall(t,h,"GET","/v1/a2a/outbox?limit=201","bridge-a",nil,400)
	bridgeCall(t,h,"GET","/v1/a2a/outbox","bridge-b",nil,200)
}

func TestBridgeJournalDeliveryCorrelatesSent(t *testing.T) {
	s,h := bridgeFixture(t)
	text := "work"
	sent := JournalEvent{EventKey:"journal-sent",Kind:"sent",Sender:humanPeer,Recipient:"omp:worker",RequestID:"non-composer-request",Text:&text}
	delivery := JournalEvent{EventKey:"journal-failed",Kind:"delivery",Sender:humanPeer,Recipient:"omp:worker",RequestID:sent.RequestID,Status:"failed",ReasonCode:busy()}
	ingestEvents(t,h,sent,delivery)
	ingestEvents(t,h,sent,delivery)
	ths,err := s.ListThreads(context.Background(),tenantA)
	if err!=nil || len(ths)!=1 { t.Fatalf("threads=%v err=%v",ths,err) }
	ms,err := s.ListMessages(context.Background(),tenantA,ths[0].ID,0,200)
	if err!=nil || len(ms)!=1 || ms[0].DeliveryState!="not_delivered" || ms[0].ReasonCode==nil || *ms[0].ReasonCode!="recipient_busy" { t.Fatalf("delivery=%v err=%v",ms,err) }
}

func TestBridgeJournalSubmittedDeliveryCorrelatesSent(t *testing.T) {
	s,h := bridgeFixture(t)
	ctx := context.Background()
	text := "work"
	sent := JournalEvent{Seq:4,EventKey:"journal-submitted",Kind:"sent",Sender:"agent:mau",Recipient:"omp:worker",RequestID:"submitted-request",Text:&text,Status:"submitted"}
	delivery := JournalEvent{Seq:5,EventKey:"journal-delivered",Kind:"delivery",Sender:sent.Sender,Recipient:sent.Recipient,RequestID:sent.RequestID,Status:"delivered"}
	invalid := sent
	invalid.Status = "unknown"
	bridgeCall(t,h,"POST","/v1/a2a/ingest","bridge-a",map[string]any{"events":[]JournalEvent{invalid}},400)
	// The old mapping rejects this canonical submitted event with HTTP 400.
	ingestEvents(t,h,sent)
	ths,err := s.ListThreads(ctx,tenantA)
	if err!=nil || len(ths)!=2 { t.Fatalf("threads=%v err=%v",ths,err) }
	ids := make(map[string]string)
	for _, th := range ths {
		ms,err := s.ListMessages(ctx,tenantA,th.ID,0,200)
		if err!=nil || len(ms)!=1 || ms[0].Direction!="observed" || ms[0].Text!=text || ms[0].DeliveryState!="sent" { t.Fatalf("submitted=%v err=%v",ms,err) }
		ids[th.ID] = ms[0].ID
	}
	ingestEvents(t,h,sent,delivery)
	ingestEvents(t,h,sent,delivery)
	for _, th := range ths {
		ms,err := s.ListMessages(ctx,tenantA,th.ID,0,200)
		if err!=nil || len(ms)!=1 || ms[0].ID!=ids[th.ID] || ms[0].Direction!="observed" || ms[0].Text!=text || ms[0].DeliveryState!="delivered" { t.Fatalf("delivery replay=%v err=%v",ms,err) }
	}
}
