package a2a

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func deviceAPI(t *testing.T) (*Store, *httptest.Server, string, string, *Events) {
	t.Helper()
	s, _, td := migrated(t)
	pool, err := pgxpool.New(context.Background(), td.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	token := "fixture-device-bearer"
	hash := sha256.Sum256([]byte(token))
	deviceID := uuid.NewString()
	if _, err := pool.Exec(context.Background(), `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,decode($3,'hex'))`, tenantA, deviceID, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	events := StartEvents(ctx, td.DSN, s)
	ah := auth.NewHandler(auth.NewPostgresStore(pool), "")
	mux := http.NewServeMux()
	ah.Register(mux)
	(&Handler{Store: s, Events: events}).Routes(mux)
	server := httptest.NewServer(ah.Middleware(mux))
	t.Cleanup(server.Close)
	return s, server, deviceID, token, events
}

func apiRequest(t *testing.T, server *httptest.Server, token, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func decodeResponse(t *testing.T, res *http.Response, status int, out any) {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status=%d want %d", res.StatusCode, status)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}

func TestDeviceAuthenticationAndRevocation(t *testing.T) {
	_, server, deviceID, token, _ := deviceAPI(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/v1/a2a/threads"},
		{"GET", "/v1/a2a/threads/" + uuid.NewString() + "/messages"},
		{"POST", "/v1/a2a/threads/" + uuid.NewString() + "/messages"},
		{"GET", "/v1/a2a/events"},
	} {
		for _, bearer := range []string{"", "invalid-device"} {
			res := apiRequest(t, server, bearer, tc.method, tc.path, "{}")
			if res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s %s: %d", tc.method, tc.path, res.StatusCode)
			}
			res.Body.Close()
		}
	}
	var empty struct{ Threads []Thread `json:"threads"` }
	decodeResponse(t, apiRequest(t, server, token, "GET", "/v1/a2a/threads", ""), 200, &empty)
	if empty.Threads == nil || len(empty.Threads) != 0 {
		t.Fatalf("empty threads=%+v", empty.Threads)
	}
	// Revoke the real database token, then check the middleware re-reads it.
	res := apiRequest(t, server, token, "DELETE", "/v1/devices/"+deviceID, "")
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke status=%d", res.StatusCode)
	}
	res.Body.Close()
	for _, path := range []string{"/v1/a2a/threads", "/v1/a2a/events"} {
		res := apiRequest(t, server, token, "GET", path, "")
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("revoked token %s status=%d", path, res.StatusCode)
		}
		res.Body.Close()
	}
}

func TestDeviceMessagesCursorAndIdempotentSend(t *testing.T) {
	s, server, _, token, _ := deviceAPI(t)
	ctx := context.Background()
	thread, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.UpsertThread(ctx, tenantB, ThreadInput{PeerID: "agent:korzhik", DisplayName: "Korzhik", Kind: "token_agent", Availability: "online"})
	if err != nil {
		t.Fatal(err)
	}
	var threads struct{ Threads []Thread `json:"threads"` }
	decodeResponse(t, apiRequest(t, server, token, "GET", "/v1/a2a/threads", ""), 200, &threads)
	if len(threads.Threads) != 1 || threads.Threads[0].ID != thread.ID {
		t.Fatalf("threads=%+v", threads.Threads)
	}
	path := "/v1/a2a/threads/" + thread.ID + "/messages"
	clientID := uuid.NewString()
	body := fmt.Sprintf(`{"client_msg_id":%q,"text":"hello"}`, clientID)
	var first, again Message
	decodeResponse(t, apiRequest(t, server, token, "POST", path, body), 202, &first)
	decodeResponse(t, apiRequest(t, server, token, "POST", path, strings.Replace(body, "hello", "different", 1)), 202, &again)
	if first.ID != again.ID || first.Seq != again.Seq || again.Text != "hello" || again.DeliveryState != "pending" || again.Sender != "agent:darafei" || again.Recipient != "agent:mau" {
		t.Fatalf("idempotence: %+v %+v", first, again)
	}
	reply, err := s.IngestMessage(ctx, tenantA, MessageInput{ThreadID: thread.ID, EventKey: "reply", Direction: "in", Sender: "agent:mau", Recipient: "agent:darafei", Text: "hi", DeliveryState: "delivered"})
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Messages []Message `json:"messages"`
		NextAfter int64 `json:"next_after"`
	}
	decodeResponse(t, apiRequest(t, server, token, "GET", path+"?limit=1", ""), 200, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != first.ID || page.NextAfter != first.Seq {
		t.Fatalf("first page=%+v", page)
	}
	decodeResponse(t, apiRequest(t, server, token, "GET", fmt.Sprintf("%s?after=%d&limit=200", path, first.Seq), ""), 200, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != reply.ID || page.NextAfter != reply.Seq {
		t.Fatalf("after page=%+v", page)
	}
	decodeResponse(t, apiRequest(t, server, token, "GET", fmt.Sprintf("%s?after=%d", path, reply.Seq), ""), 200, &page)
	if page.Messages == nil || len(page.Messages) != 0 || page.NextAfter != reply.Seq {
		t.Fatalf("empty page=%+v", page)
	}
	for _, suffix := range []string{"?limit=201", "?limit=0", "?after=-1", "?after=abc"} {
		res := apiRequest(t, server, token, "GET", path+suffix, "")
		if res.StatusCode != 400 {
			t.Fatalf("%s status=%d", suffix, res.StatusCode)
		}
		res.Body.Close()
	}
	for _, method := range []string{"GET", "POST"} {
		res := apiRequest(t, server, token, method, "/v1/a2a/threads/"+other.ID+"/messages", body)
		if res.StatusCode != 404 {
			t.Fatalf("cross-tenant %s status=%d", method, res.StatusCode)
		}
		res.Body.Close()
	}
	for _, bad := range []string{`{}`, `{"client_msg_id":"bad","text":"hello"}`, fmt.Sprintf(`{"client_msg_id":%q,"text":" "}`, uuid.NewString()), body+`{}`} {
		res := apiRequest(t, server, token, "POST", path, bad)
		if res.StatusCode != 400 {
			t.Fatalf("bad message status=%d", res.StatusCode)
		}
		res.Body.Close()
	}
}

func TestDeviceEventsNotifyAndPing(t *testing.T) {
	s, server, _, token, events := deviceAPI(t)
	events.pingEvery = 100 * time.Millisecond
	if NewEvents("", s).pingEvery != 15*time.Second {
		t.Fatal("production heartbeat must be 15 seconds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/v1/a2a/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream status=%d headers=%v", res.StatusCode, res.Header)
	}
	frames := make(chan string, 20)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		frame := ""
		for scanner.Scan() {
			if scanner.Text() == "" {
				select {
				case frames <- frame:
				case <-ctx.Done(): return
				}
				frame = ""
			} else {
				frame += scanner.Text() + "\n"
			}
		}
	}()
	select {
	case frame := <-frames:
		if frame != "event: a2a\ndata: {\"seq\":0}\n" {
			t.Fatalf("initial frame=%q", frame)
		}
	case <-ctx.Done(): t.Fatal("no initial frame")
	}
	thread, err := s.UpsertThread(ctx, tenantA, ThreadInput{PeerID: "agent:mau", DisplayName: "Mau", Kind: "token_agent", Availability: "online"})
	if err != nil { t.Fatal(err) }
	message, err := s.IngestMessage(ctx, tenantA, MessageInput{ThreadID: thread.ID, EventKey: "sse", Direction: "in", Sender: "agent:mau", Recipient: "agent:darafei", Text: "live", DeliveryState: "delivered"})
	if err != nil { t.Fatal(err) }
	deadline := time.NewTimer(2*time.Second)
	defer deadline.Stop()
	gotEvent, gotPing := false, false
	for !gotEvent || !gotPing {
		select {
		case frame := <-frames:
			gotEvent = gotEvent || frame == fmt.Sprintf("event: a2a\ndata: {\"seq\":%d}\n", message.Seq)
			gotPing = gotPing || frame == ": ping\n"
		case <-deadline.C:
			t.Fatalf("within 2s: event=%v ping=%v", gotEvent, gotPing)
		}
	}
	// Negative control: another tenant's commit must not wake this stream.
	_, err = s.UpsertThread(ctx, tenantB, ThreadInput{PeerID: "agent:x", DisplayName: "X", Kind: "token_agent", Availability: "online"})
	if err != nil { t.Fatal(err) }
	quiet := time.NewTimer(200*time.Millisecond)
	defer quiet.Stop()
	for {
		select {
		case frame := <-frames:
			if strings.HasPrefix(frame, "event: a2a") { t.Fatalf("cross-tenant event=%q", frame) }
		case <-quiet.C: return
		}
	}
}
