package mailbody

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

	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSanitizeDropsTrackersAndScripts(t *testing.T) {
	got, err := Sanitize(context.Background(), `<div><p>Keep me</p><a href="https://sendgrid.net/wf/open?upn=opaque">tracked link</a><script>alert(1)</script><img src="https://mailtrack.io/pixel.gif" width="1" height="1"><img src="https://example.org/photo.jpg" width="300" height="200"></div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Keep me") {
		t.Fatalf("content lost: %s", got)
	}
	if strings.Contains(got, "script") || strings.Contains(got, "mailtrack.io") || strings.Contains(got, "example.org") {
		t.Fatalf("unsafe/unfetched resources remain: %s", got)
	}
}

func TestGmailSourceURL(t *testing.T) {
	if got := gmailSourceURL("user@example.com", "thread-abc/123"); got != "https://mail.google.com/mail/?authuser=user%40example.com#all/thread-abc%2F123" {
		t.Fatalf("account-aware URL = %q", got)
	}
	if got := gmailSourceURL("user+tag@example.com", "abc"); got != "https://mail.google.com/mail/?authuser=user%2Btag%40example.com#all/abc" {
		t.Fatalf("plus-escaped URL = %q", got)
	}
	if got := gmailSourceURL("", "thread-abc"); got != "" {
		t.Fatalf("missing address must yield empty, got %q", got)
	}
	if got := gmailSourceURL("user@example.com", ""); got != "" {
		t.Fatalf("missing thread must yield empty, got %q", got)
	}
	if got := gmailSourceURL("", ""); got != "" {
		t.Fatalf("both missing must yield empty, got %q", got)
	}
}

func TestBodyRouteAgentAndMail(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	paths, err := filepath.Glob("../../db/0*.sql")
	if err != nil || len(paths) == 0 {
		t.Fatalf("migration discovery: %v", err)
	}
	for _, p := range paths {
		body, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(tdb.Migration(string(body))); e != nil {
			t.Fatalf("migration %s: %v", p, e)
		}
	}
	tenant := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	other := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1),($2)`, tenant, other); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	// Establish the owning tenant before writing cards/accounts/card_bodies as
	// litterbox_app, otherwise the row-level security policies reject the rows.
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant.String()); err != nil {
		t.Fatal(err)
	}
	agentCard := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")
	stored := "<pre style='white-space:pre-wrap'>full result</pre><div><a href='data:text/plain;name=a.txt;base64,aGk='>a.txt (2 bytes)</a></div>"
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,source_kind,title,summary,state) VALUES($1,$2,'agent','res-9','research_result','Report','full result','open')`, tenant, agentCard); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,$3)`, tenant, agentCard, stored); err != nil {
		t.Fatal(err)
	}
	account := uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	mailCard := uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
	if _, err = db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'user@example.com',decode('00','hex'))`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,source,account_id,gmail_thread_id,subject) VALUES($1,$2,'mail',$3,'thread-abc','Mail subject')`, tenant, mailCard, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>Hello</p><script>alert(1)</script><a href="https://sendgrid.net/wf/open?upn=opaque">tracked</a>')`, tenant, mailCard); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at,sender_name,sender_address) VALUES
		($1,'11111111-1111-4111-8111-111111111111',$2,'old-message','{}','<p>Earlier original</p>','',decode('00','hex'),'2025-01-01T00:00:00Z','',''),
		($1,'22222222-2222-4222-8222-222222222222',$2,'new-message','{}','<p>Latest original</p>','',decode('00','hex'),'2025-01-02T00:00:00Z','Latest Sender','latest@example.com')`, tenant, mailCard); err != nil {
		t.Fatal(err)
	}
	h := Handler{DB: db, Images: nil}
	mux := http.NewServeMux()
	mux.Handle("GET /v1/cards/{id}/body", h)
	var lastMessages []threadMessage
	fetch := func(tenantID, cardID string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/v1/cards/"+cardID+"/body", nil)
		req = req.WithContext(agents.WithTenant(req.Context(), tenantID))
		req.SetPathValue("id", cardID)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			return w.Code, ""
		}
		var resp struct {
			HTML      string          `json:"html"`
			ThreadID  string          `json:"threadId"`
			AccountID string          `json:"accountId"`
			SourceURL string          `json:"source_url"`
			Messages  []threadMessage `json:"messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		lastMessages = resp.Messages
		return w.Code, resp.HTML + "|" + resp.ThreadID + "|" + resp.AccountID + "|" + resp.SourceURL
	}

	// The owning tenant gets the stored server-built document unchanged.
	code, got := fetch(tenant.String(), agentCard.String())
	if code != http.StatusOK {
		t.Fatalf("agent body: got %d, want 200", code)
	}
	if want := stored + "|||"; got != want {
		t.Fatalf("agent body must serve the stored HTML verbatim with empty thread/account/url:\n got %q\nwant %q", got, want)
	}
	// Another tenant must not see it.
	if code, _ := fetch(other.String(), agentCard.String()); code != http.StatusNotFound {
		t.Fatalf("foreign agent body: got %d, want 404", code)
	}
	// A missing card stays 404.
	if code, _ := fetch(tenant.String(), uuid.New().String()); code != http.StatusNotFound {
		t.Fatalf("missing card: got %d, want 404", code)
	}
	// Mail behaviour is unchanged: Sanitize plus the account-aware Gmail URL.
	code, got = fetch(tenant.String(), mailCard.String())
	if code != http.StatusOK {
		t.Fatalf("mail body: got %d, want 200", code)
	}
	if !strings.Contains(got, "Hello") || strings.Contains(got, "script") || strings.Contains(got, "sendgrid.net") {
		t.Fatalf("mail body must keep sanitizing: %q", got)
	}
	if want := "https://mail.google.com/mail/?authuser=user%40example.com#all/thread-abc"; !strings.Contains(got, "|thread-abc|"+account.String()+"|"+want) {
		t.Fatalf("mail body must keep thread/account/gmail url: %q", got)
	}
	// Mail exposes every original message oldest-to-newest with truthful
	// per-message senders; a legacy row without From stays empty rather than
	// inheriting the card's latest sender.
	if len(lastMessages) != 2 {
		t.Fatalf("messages=%d, want 2", len(lastMessages))
	}
	if lastMessages[0].GmailMessageID != "old-message" || lastMessages[0].SenderName != "" || lastMessages[0].SenderAddress != "" {
		t.Fatalf("legacy message sender must stay empty: %+v", lastMessages[0])
	}
	if lastMessages[1].GmailMessageID != "new-message" || lastMessages[1].SenderName != "Latest Sender" || lastMessages[1].SenderAddress != "latest@example.com" {
		t.Fatalf("latest message sender: %+v", lastMessages[1])
	}
	if lastMessages[0].HTML != "<p>Earlier original</p>" || lastMessages[1].HTML != "<p>Latest original</p>" {
		t.Fatalf("message order/content wrong: %+v", lastMessages)
	}
	// Non-mail sources keep no message history.
	code, _ = fetch(tenant.String(), agentCard.String())
	if code != http.StatusOK || len(lastMessages) != 0 {
		t.Fatalf("agent messages: code=%d len=%d, want 200/0", code, len(lastMessages))
	}
}
