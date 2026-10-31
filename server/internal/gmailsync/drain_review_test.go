package gmailsync

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync"
 "sync/atomic"
 "testing"
 "time"

 "github.com/google/uuid"
)

func TestConcurrentOpReplayCallsGmailOnce(t *testing.T) {
 f := newSpamFixture(t)
 op := uuid.New()
 body := `{"op_id":"`+op.String()+`","card_id":"`+f.card.String()+`","type":"archive","args":{}}`
 var wg sync.WaitGroup
 codes := make(chan int, 2)
 for range 2 { wg.Add(1); go func() { defer wg.Done(); w:=httptest.NewRecorder(); f.api.ServeHTTP(w,httptest.NewRequest("POST","/v1/ops",strings.NewReader(body))); codes<-w.Code }() }
 wg.Wait(); close(codes)
 for code:=range codes { if code!=200 { t.Errorf("replay status=%d",code) } }
 f.checkCalls(t, []string{"thread-1"}, nil)
}

func TestArchiveUndoWaitsForPriorCommit(t *testing.T) {
 f:=newSpamFixture(t)
 ctx:=context.Background()
 tx,err:=f.db.Begin(ctx); if err!=nil { t.Fatal(err) }; defer tx.Rollback(ctx)
 if _,err=tx.Exec(ctx,`SELECT next_seq FROM tenants WHERE id=$1 FOR UPDATE`,f.tenant);err!=nil { t.Fatal(err) }
 post:=func(kind,args string) int { body:=`{"op_id":"`+uuid.NewString()+`","card_id":"`+f.card.String()+`","type":"`+kind+`","args":`+args+`}`; w:=httptest.NewRecorder();f.api.ServeHTTP(w,httptest.NewRequest("POST","/v1/ops",strings.NewReader(body)));return w.Code }
 done:=make(chan int,2)
 go func(){done<-post("archive",`{}`)}()
 deadline:=time.Now().Add(3*time.Second)
 for { f.mu.Lock();n:=len(f.calls);f.mu.Unlock();if n>0 {break};if time.Now().After(deadline){t.Fatal("archive never reached Gmail")};time.Sleep(time.Millisecond) }
 go func(){done<-post("gmail.label_add",`{"label":"INBOX"}`)}()
 time.Sleep(100*time.Millisecond)
 f.mu.Lock(); before:=len(f.calls); f.mu.Unlock()
 if err=tx.Commit(ctx);err!=nil {t.Fatal(err)}
 for range 2 {if code:=<-done;code!=200 {t.Errorf("status=%d",code)}}
 if before!=1 {t.Errorf("inverse reached Gmail before archive commit: calls=%d",before)}
 f.state(t,f.card,"open")
 f.mu.Lock();inbox:=f.gmail.inbox;f.mu.Unlock();if !inbox {t.Error("Gmail and DB diverged")}
}

func TestCancelledQueuedModifyMakesNoWANCall(t *testing.T) {
 var calls atomic.Int32
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls.Add(1)}));defer srv.Close()
 c:=&Client{HTTP:srv.Client(),APIBase:srv.URL,accessToken:"cached",tokenExpiry:time.Now().Add(time.Hour)}
 ctx,cancel:=context.WithCancel(context.Background())
 result:=make(chan error,1)
 go func(){result<-c.ModifyBatched(ctx,"thread",nil,[]string{"INBOX"})}()
 deadline:=time.Now().Add(time.Second)
 for { c.bmu.Lock();queued:=len(c.pending)>0;c.bmu.Unlock();if queued {break};if time.Now().After(deadline){t.Fatal("call was not queued")};time.Sleep(time.Millisecond) }
 cancel()
 if err:=<-result;!errors.Is(err,context.Canceled){t.Fatalf("error=%v",err)}
 time.Sleep(100*time.Millisecond)
 if n:=calls.Load();n!=0 {t.Fatalf("cancelled request made %d WAN calls",n)}
}

func TestQuotaPacingAnd429Backoff(t *testing.T) {
 var mu sync.Mutex
 var stamps []time.Time
 var attempts atomic.Int32
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  inner:=func(string,string,[]byte)(int,string){mu.Lock();stamps=append(stamps,time.Now());mu.Unlock();if attempts.Add(1)==1{return 429,"rate limited"};return 200,"{}"}
  if r.URL.Path=="/batch/gmail/v1" {serveFakeBatch(w,r,inner)} else {code,body:=inner("","",nil);w.WriteHeader(code);_,_=w.Write([]byte(body))}
 }));defer srv.Close()
 c:=&Client{HTTP:srv.Client(),APIBase:srv.URL,accessToken:"cached",tokenExpiry:time.Now().Add(time.Hour),RetryDelay:50*time.Millisecond}
 var wg sync.WaitGroup
 for i:=range 30 {wg.Add(1);go func(){defer wg.Done();if err:=c.ModifyBatched(context.Background(),string(rune('a'+i)),nil,[]string{"INBOX"});err!=nil{t.Error(err)}}()};wg.Wait()
 mu.Lock();defer mu.Unlock()
 for i,stamp:=range stamps {n:=0;for _,other:=range stamps[i:] {if other.Sub(stamp)<time.Second {n++}};if n>25 {t.Fatalf("%d calls in a rolling second, quota is 25",n)}}
 if attempts.Load()!=31 {t.Fatalf("attempts=%d, want 31 including retry",attempts.Load())}
 if stamps[len(stamps)-1].Sub(stamps[0])<time.Second {t.Fatal("quota burst was not paced")}
}
