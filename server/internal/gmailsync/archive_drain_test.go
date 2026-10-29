package gmailsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// slowGmail is a Gmail provider with a fixed WAN latency per request. It counts
// OAuth token POSTs and records every archived thread, whether it arrived as a
// plain threads.modify or inside an HTTP batch. Threads named "bad-*" are
// refused with 403.
type slowGmail struct {
	latency  time.Duration
	tokens   atomic.Int32
	requests atomic.Int32
	mu       sync.Mutex
	archived map[string]int
}

var modifyLine = regexp.MustCompile(`^POST \S*/threads/([^/\s]+)/modify`)

func (g *slowGmail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(g.latency)
	switch {
	case r.URL.Path == "/token":
		g.tokens.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "drain-access", "expires_in": 3600})
	case strings.HasSuffix(r.URL.Path, "/modify"):
		g.requests.Add(1)
		thread := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/threads/"), "/modify")
		if strings.HasPrefix(thread, "bad-") {
			http.Error(w, "fixture Gmail denied "+thread, http.StatusForbidden)
			return
		}
		g.record(thread)
	case r.URL.Path == "/batch/gmail/v1":
		g.requests.Add(1)
		serveFakeBatch(w, r, func(thread, _ string, _ []byte) (int, string) {
			if strings.HasPrefix(thread, "bad-") {
				return http.StatusForbidden, "fixture Gmail denied " + thread
			}
			g.record(thread)
			return http.StatusOK, "{}"
		})
	default:
		http.NotFound(w, r)
	}
}

// serveFakeBatch answers a Gmail HTTP batch request: it parses each embedded
// threads.modify call and replies with one application/http part per call.
func serveFakeBatch(w http.ResponseWriter, r *http.Request, inner func(thread, authorization string, payload []byte) (int, string)) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var out strings.Builder
	mw := multipart.NewWriter(&out)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		raw, _ := io.ReadAll(part)
		status, body := http.StatusBadRequest, `{"error":"unparsable call"}`
		if m := modifyLine.FindStringSubmatch(string(raw)); m != nil {
			thread, _ := url.PathUnescape(m[1])
			payload := raw[strings.Index(string(raw), "\r\n\r\n")+4:]
			status, body = inner(thread, r.Header.Get("Authorization"), payload)
		}
		pw, _ := mw.CreatePart(map[string][]string{"Content-Type": {"application/http"}, "Content-Id": {"<response-" + strings.Trim(part.Header.Get("Content-ID"), "<>") + ">"}})
		fmt.Fprintf(pw, "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\n\r\n%s", status, http.StatusText(status), body)
	}
	mw.Close()
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	_, _ = io.WriteString(w, out.String())
}

func (g *slowGmail) record(thread string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.archived == nil {
		g.archived = map[string]int{}
	}
	g.archived[thread]++
}

// drain posts one archive op per card concurrently through the real ops.API and
// returns each card's HTTP status and the wall time.
func (f *spamFixture) drain(cards []uuid.UUID) (map[uuid.UUID]int, time.Duration) {
	status := make(map[uuid.UUID]int, len(cards))
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := time.Now()
	for _, card := range cards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"op_id":"` + uuid.NewString() + `","card_id":"` + card.String() + `","type":"archive","args":{}}`
			w := httptest.NewRecorder()
			f.api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
			mu.Lock()
			status[card] = w.Code
			mu.Unlock()
		}()
	}
	wg.Wait()
	return status, time.Since(start)
}

func newDrainFixture(t *testing.T, n int, bad int) (*spamFixture, *slowGmail, []uuid.UUID) {
	t.Helper()
	f := newSpamFixture(t)
	g := &slowGmail{latency: 300 * time.Millisecond}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	useTestOps(ClientForCard("id", "secret", srv.URL+"/gmail/v1/users/me", srv.URL+"/token", srv.Client()))
	ctx := context.Background()
	var cards []uuid.UUID
	for i := range n {
		card, thread := uuid.New(), fmt.Sprintf("thread-%03d", i)
		if i < bad {
			thread = fmt.Sprintf("bad-%03d", i)
		}
		if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name) VALUES($1,$2,$3,$4,'mail','Sender')`, f.tenant, card, f.account, thread); err != nil {
			t.Fatal(err)
		}
		cards = append(cards, card)
	}
	return f, g, cards
}

// 130 concurrent archives of one account against a 300 ms provider must not
// queue behind the tenant sequence lock or pay one OAuth exchange and one
// provider round trip each.
func TestArchiveBacklogDrainsInUnderASecondAndAHalf(t *testing.T) {
	f, g, cards := newDrainFixture(t, 130, 0)
	ctx := context.Background()
	var cursor, nextBefore int64
	if err := f.db.QueryRow(ctx, `SELECT COALESCE(max(seq),0) FROM changes WHERE tenant_id=$1`, f.tenant).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(ctx, `SELECT next_seq FROM tenants WHERE id=$1`, f.tenant).Scan(&nextBefore); err != nil {
		t.Fatal(err)
	}
	status, elapsed := f.drain(cards)
	t.Logf("130 archives: wall=%v token POSTs=%d provider requests=%d", elapsed, g.tokens.Load(), g.requests.Load())
	for card, code := range status {
		if code != 200 {
			t.Errorf("card %s status=%d, want 200", card, code)
		}
	}
	var archived int
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND id=ANY($2) AND state='archived'`, f.tenant, cards).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if archived != len(cards) {
		t.Errorf("archived cards=%d, want %d", archived, len(cards))
	}
	g.mu.Lock()
	if len(g.archived) != len(cards) {
		t.Errorf("provider archived %d distinct threads, want %d", len(g.archived), len(cards))
	}
	g.mu.Unlock()
	// Ordering semantics: one change per archived card, strictly increasing and
	// gap-free, and next_seq advanced by exactly that many.
	rows, err := f.db.Query(ctx, `SELECT seq FROM changes WHERE tenant_id=$1 AND seq>$2 ORDER BY seq`, f.tenant, cursor)
	if err != nil {
		t.Fatal(err)
	}
	prev, count := cursor, 0
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			t.Fatal(err)
		}
		if seq != prev+1 {
			t.Errorf("change seq %d follows %d: not strictly consecutive", seq, prev)
		}
		prev, count = seq, count+1
	}
	rows.Close()
	var nextAfter int64
	if err := f.db.QueryRow(ctx, `SELECT next_seq FROM tenants WHERE id=$1`, f.tenant).Scan(&nextAfter); err != nil {
		t.Fatal(err)
	}
	if count != len(cards) || nextAfter-nextBefore != int64(len(cards)) {
		t.Errorf("changes=%d next_seq delta=%d, want %d each", count, nextAfter-nextBefore, len(cards))
	}
	if n := g.tokens.Load(); n > 1 {
		t.Errorf("OAuth token POSTs=%d, want at most 1", n)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Errorf("wall time %v, want under 1.5s", elapsed)
	}
}

// One provider refusal fails only that op: its card stays open and leaves no
// receipt, every other op in the same coalesced batch succeeds.
func TestArchiveBatchProviderFailureIsolatesOneOp(t *testing.T) {
	f, g, cards := newDrainFixture(t, 30, 1)
	status, _ := f.drain(cards)
	ctx := context.Background()
	for i, card := range cards {
		want, wantState := 200, "archived"
		if i == 0 {
			want, wantState = 422, "open"
		}
		if status[card] != want {
			t.Errorf("card %d status=%d, want %d", i, status[card], want)
		}
		f.state(t, card, wantState)
	}
	var receipts int
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1 AND payload->>'card_id'=$2`, f.tenant, cards[0].String()).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Errorf("failed op left %d receipts", receipts)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.archived) != 29 || g.archived["bad-000"] != 0 {
		t.Errorf("provider archived %d threads (bad: %d), want 29 and 0", len(g.archived), g.archived["bad-000"])
	}
}

// A 401 drops the cached access token and retries once with a fresh one;
// later calls reuse that token.
func TestUnauthorizedDropsCachedTokenOnce(t *testing.T) {
	var tokens, modifies atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("t%d", tokens.Add(1)), "expires_in": 3600})
			return
		}
		modifies.Add(1)
		if r.Header.Get("Authorization") != "Bearer t2" {
			http.Error(w, "expired", http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, TokenURL: srv.URL + "/token", RefreshToken: "r"}
	for range 2 {
		if err := c.ModifyBatched(context.Background(), "thread", nil, []string{"INBOX"}); err != nil {
			t.Fatal(err)
		}
	}
	if tokens.Load() != 2 || modifies.Load() != 3 {
		t.Fatalf("token POSTs=%d modifies=%d, want 2 and 3", tokens.Load(), modifies.Load())
	}
}
