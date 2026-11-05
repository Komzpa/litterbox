package gmailsync

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/bodyversion"
	"github.com/Komzpa/litterbox/server/internal/mailbody"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/google/uuid"
)

func TestThreadSyncReconcilesMessages(t *testing.T) {
	db, tdb := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	message := func(id string, date int, labels ...string) map[string]any {
		return map[string]any{"id": id, "internalDate": fmt.Sprint(1780000000000 + date), "labelIds": labels, "payload": map[string]any{"headers": []map[string]string{{"name": "From", "value": id + " <" + id + "@example.test>"}}}}
	}
	first, middle, latest := message("first", 1, "INBOX"), message("middle", 2), message("latest", 3, "INBOX", "UNREAD")
	draft := message("draft", 4, "DRAFT")
	msgs := []map[string]any{first, latest, draft}
	gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "expires_in": 3600})
		case strings.HasSuffix(r.URL.Path, "/threads/thread-1"):
			json.NewEncoder(w).Encode(map[string]any{"id": "thread-1", "messages": msgs})
		case strings.Contains(r.URL.Path, "/messages/"):
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			raw := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + id + " body"
			json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gmail.Close()
	client := &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "x"}
	s := &Syncer{DB: db}
	save := func() {
		t.Helper()
		if err := s.saveThread(ctx, Account{TenantID: tenant, ID: account}, client, "thread-1", true); err != nil {
			t.Fatal(err)
		}
	}
	save()
	readVersion := func() string {
		t.Helper()
		var version string
		if err := db.QueryRow(ctx, `SELECT `+bodyversion.SQL+` FROM cards c LEFT JOIN accounts a ON a.tenant_id=c.tenant_id AND a.id=c.account_id WHERE c.tenant_id=$1 AND c.gmail_thread_id='thread-1'`, tenant).Scan(&version); err != nil {
			t.Fatal(err)
		}
		return version
	}
	unchanged := readVersion()
	save()
	if got := readVersion(); got != unchanged {
		t.Fatalf("unchanged sync changed body version: %s != %s", got, unchanged)
	}
	// The older archived message arrives after the newest; then the first is removed.
	msgs = []map[string]any{first, latest, draft, middle}
	save()
	msgs = []map[string]any{latest, draft, middle}
	save()
	rows, err := db.Query(ctx, `SELECT gmail_message_id FROM messages WHERE tenant_id=$1 ORDER BY received_at`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var combined string
	if err := db.QueryRow(ctx, `SELECT b.html FROM card_bodies b JOIN cards c ON c.tenant_id=b.tenant_id AND c.id=b.card_id WHERE c.tenant_id=$1 AND c.gmail_thread_id='thread-1'`, tenant).Scan(&combined); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(combined, "draft body") {
		t.Fatalf("draft leaked into legacy body: %s", combined)
	}
	if fmt.Sprint(ids) != "[middle latest draft]" {
		t.Fatalf("messages after reconciliation: %v", ids)
	}
	var cardID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM cards WHERE tenant_id=$1 AND gmail_thread_id='thread-1'`, tenant).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	httpDB, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer httpDB.Close()
	req := httptest.NewRequest("GET", "/v1/cards/"+cardID+"/body", nil)
	req.SetPathValue("id", cardID)
	req = req.WithContext(agents.WithTenant(req.Context(), tenant.String()))
	rec := httptest.NewRecorder()
	(mailbody.Handler{DB: httpDB}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("body: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Messages []struct {
			GmailID string `json:"gmail_message_id"`
			HTML    string `json:"html"`
			Unread  bool   `json:"unread"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Messages) != 2 || response.Messages[0].GmailID != "middle" || response.Messages[0].HTML != "<pre>middle body</pre>" || response.Messages[0].Unread || response.Messages[1].GmailID != "latest" || !response.Messages[1].Unread {
		t.Fatalf("thread body after sync: %+v", response.Messages)
	}

}
