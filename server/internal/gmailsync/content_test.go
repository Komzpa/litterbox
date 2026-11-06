package gmailsync

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/bodyversion"
	"github.com/Komzpa/litterbox/server/internal/mailbody"
	"github.com/Komzpa/litterbox/server/internal/mailhtml"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/google/uuid"
)

func TestMailContentPersistenceDownloadAndRepair(t *testing.T) {
	db, tdb := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1);`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	raw := []byte("From: Sender <sender@example.test>\r\nTo: One <one@example.test>, two@example.test\r\nCc: Three <three@example.test>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nvisible body\r\n--x\r\nContent-Type: application/octet-stream; name=report.bin\r\nContent-Disposition: attachment; filename=report.bin\r\nContent-Transfer-Encoding: base64\r\n\r\nAAEC/w==\r\n--x--\r\n")
	var requests atomic.Int32
	fail := false
	gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "x", "expires_in": 3600})
		case strings.Contains(r.URL.Path, "/threads/"):
			json.NewEncoder(w).Encode(map[string]any{"id": "thread", "messages": []any{map[string]any{"id": "m1", "internalDate": "1780000000000", "labelIds": []string{"INBOX"}}}})
		case strings.Contains(r.URL.Path, "/messages/"):
			requests.Add(1)
			if fail {
				http.Error(w, "temporary refusal", 400)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString(raw)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gmail.Close()
	c := &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "x"}
	s := &Syncer{DB: db}
	a := Account{TenantID: tenant, ID: account}
	if err := s.saveThread(ctx, a, c, "thread", true); err != nil {
		t.Fatal(err)
	}
	var card, message string
	var stored []byte
	var version int
	if err := db.QueryRow(ctx, `SELECT card_id::text,id::text,raw_mime,content_version FROM messages WHERE tenant_id=$1`, tenant).Scan(&card, &message, &stored, &version); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, raw) || version != mailContentVersion {
		t.Fatalf("raw/version not retained: version=%d raw=%q", version, stored)
	}
	httpDB, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer httpDB.Close()
	httpDB.SetMaxOpenConns(1)
	if _, err = httpDB.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	metadata := func() mailbody.ContentMetadata {
		t.Helper()
		tx, err := httpDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`SELECT set_config('litterbox.tenant_id',$1,true)`, tenant.String()); err != nil {
			t.Fatal(err)
		}
		m, err := mailbody.MessageContent(ctx, tx, tenant.String(), card, message)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	meta := metadata()
	if len(meta.To) != 2 || meta.To[0].Name != "One" || len(meta.Cc) != 1 || len(meta.Attachments) != 2 {
		t.Fatalf("metadata=%+v", meta)
	}
	h := mailbody.Handler{DB: httpDB}
	bodyRequest := httptest.NewRequest("GET", "/v1/cards/"+card+"/body", nil)
	bodyRequest.SetPathValue("id", card)
	bodyRequest = bodyRequest.WithContext(agents.WithTenant(bodyRequest.Context(), tenant.String()))
	bodyResponse := httptest.NewRecorder()
	h.ServeHTTP(bodyResponse, bodyRequest)
	var bodyPayload struct {
		Messages []struct{ mailbody.ContentMetadata } `json:"messages"`
	}
	if err := json.Unmarshal(bodyResponse.Body.Bytes(), &bodyPayload); err != nil || bodyResponse.Code != 200 || len(bodyPayload.Messages) != 1 {
		t.Fatalf("body API metadata status=%d body=%s error=%v", bodyResponse.Code, bodyResponse.Body.String(), err)
	}
	if got := bodyPayload.Messages[0]; len(got.To) != 2 || len(got.Cc) != 1 || len(got.Attachments) != 2 || got.Attachments[0].DownloadURL == "" {
		t.Fatalf("body API omitted content: %+v", got)
	}
	download := func(who string, file string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", card)
		r.SetPathValue("fileID", file)
		if who != "" {
			r = r.WithContext(agents.WithTenant(r.Context(), who))
		}
		w := httptest.NewRecorder()
		h.DownloadFile(w, r)
		return w
	}
	file := meta.Attachments[0]
	if got := download(tenant.String(), file.ID); got.Code != 200 || !bytes.Equal(got.Body.Bytes(), []byte{0, 1, 2, 255}) || !strings.Contains(got.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("attachment response=%d %q", got.Code, got.Body.String())
	}
	if got := download(tenant.String(), message); got.Code != 200 || !bytes.Equal(got.Body.Bytes(), raw) {
		t.Fatalf("raw response=%d", got.Code)
	}
	if got := download("", file.ID); got.Code != 401 {
		t.Fatalf("anonymous=%d", got.Code)
	}
	if got := download(uuid.NewString(), file.ID); got.Code != 404 {
		t.Fatalf("other tenant=%d", got.Code)
	}
	bodyVersion := func() string {
		t.Helper()
		var v string
		if err := db.QueryRow(ctx, `SELECT `+bodyversion.SQL+` FROM cards c LEFT JOIN accounts a ON a.tenant_id=c.tenant_id AND a.id=c.account_id WHERE c.id=$1`, card).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	before := bodyVersion()
	if err := s.saveThread(ctx, a, c, "thread", true); err != nil {
		t.Fatal(err)
	}
	if bodyVersion() != before {
		t.Fatal("unchanged content changed body identity")
	}
	if metadata().Attachments[0].ID != file.ID {
		t.Fatal("attachment ID changed on replay")
	}
	// Legacy body repairs must not reopen, unpin, unsnooze, relabel or overwrite notes.
	if _, err := db.Exec(ctx, `UPDATE cards SET state='snoozed',note='owner note',pinned_rank=7,snooze_until='2030-01-01',mail_content_checked_at=NULL WHERE id=$1;`, card); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE messages SET content_version=0,html='',text='old',raw_mime='' WHERE id=$1`, message); err != nil {
		t.Fatal(err)
	}
	requests.Store(0)
	fail = true
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatal("missing failed repair attempt")
	}
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatal("failed repair immediately retried")
	}
	fail = false
	if _, err := db.Exec(ctx, `UPDATE messages SET content_repair_attempted_at=now()-interval '2 hours' WHERE id=$1`, message); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE cards SET mail_content_checked_at=now()-interval '2 hours' WHERE id=$1`, card); err != nil {
		t.Fatal(err)
	}
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	var state, note, text string
	var pinned int
	var labels []string
	if err := db.QueryRow(ctx, `SELECT c.state,c.note,c.pinned_rank,m.text,m.labels,m.raw_mime,m.content_version FROM cards c JOIN messages m ON m.card_id=c.id WHERE c.id=$1`, card).Scan(&state, &note, &pinned, &text, &labels, &stored, &version); err != nil {
		t.Fatal(err)
	}
	if state != "snoozed" || note != "owner note" || pinned != 7 || !strings.Contains(text, "visible body") || fmt.Sprint(labels) != "[INBOX]" || !bytes.Equal(stored, raw) || version != mailContentVersion {
		t.Fatalf("repair lost content/state: %s %s %d %q %v %d", state, note, pinned, text, labels, version)
	}
	requests.Store(0)
	if err := s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("complete message was refetched")
	}

	// v1 retained MIME must be reprocessed locally by the ordinary repair worker.
	calendarRaw, err := os.ReadFile("../mailhtml/testdata/calendar/google.eml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE messages SET content_repair_attempted_at=NULL,raw_mime=$1,content_version=1,decode_status='Unsupported message part (text/calendar)',html='<p>Unsupported message part (text/calendar)</p>',text='' WHERE id=$2`, calendarRaw, message); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE cards SET mail_content_checked_at=NULL WHERE id=$1`, card); err != nil {
		t.Fatal(err)
	}
	oldVersion := bodyVersion()
	if err = s.backfillContent(ctx, a, c); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || bodyVersion() == oldVersion {
		t.Fatal("repair must use retained MIME and advance body revision")
	}
	meta = metadata()
	if meta.DecodeStatus != "" || len(meta.Invitations) != 1 || meta.Invitations[0].Summary != "Meet with Example" || len(meta.Attachments) != 2 {
		t.Fatalf("calendar metadata=%+v", meta)
	}
	if got := download(tenant.String(), meta.Attachments[0].ID); got.Code != 200 || !bytes.Contains(got.Body.Bytes(), []byte("BEGIN:VCALENDAR")) {
		t.Fatal("ICS download lost")
	}
	bodyResponse = httptest.NewRecorder()
	h.ServeHTTP(bodyResponse, bodyRequest)
	if bodyResponse.Code != 200 || strings.Contains(bodyResponse.Body.String(), "Unsupported") || !strings.Contains(bodyResponse.Body.String(), `"invitations":[{`) {
		t.Fatal(bodyResponse.Body.String())
	}
	if err = db.QueryRow(ctx, `SELECT c.state,c.note,c.pinned_rank,m.labels,m.raw_mime,m.content_version FROM cards c JOIN messages m ON m.card_id=c.id WHERE c.id=$1`, card).Scan(&state, &note, &pinned, &labels, &stored, &version); err != nil {
		t.Fatal(err)
	}
	if state != "snoozed" || note != "owner note" || pinned != 7 || fmt.Sprint(labels) != "[INBOX]" || !bytes.Equal(stored, calendarRaw) || version != 2 {
		t.Fatal("calendar repair changed owner state or failed revision")
	}
}

func TestReprocessingPreservesFetchedImages(t *testing.T) {
	raw := []byte("Content-Type: text/html\r\n\r\n<p>body</p><img src=\"https://images.example.test/p.png\" width=\"300\" height=\"200\">")
	cached := `<p>body</p><img src="data:image/png;base64,aW1hZ2U=" data-original-src="https://images.example.test/p.png" data-fetch-state="available" width="300" height="200">`
	// The reserved domain cannot supply the image. Already persisted bytes win.
	s := &Syncer{}
	got, err := s.prepareContent(context.Background(), raw, cached)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.HTML, `src="data:image/png;base64,aW1hZ2U="`) || strings.Contains(got.HTML, `data-fetch-state="unavailable"`) {
		t.Fatal("cached image erased", got.HTML)
	}
	// An unrelated cached resource cannot replace an image's source.
	got, err = s.prepareContent(context.Background(), []byte("Content-Type: text/html\r\n\r\n<p>other body</p>"), cached)
	if err != nil || strings.Contains(got.HTML, "data:image/") {
		t.Fatal("unrelated image injected", got, err)
	}
}

func TestContentRepairWorkerBoundedAndDoesNotStarve(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token,history_id) VALUES($1,$2,'owner@example.test','x','1')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		card := uuid.New()
		if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,sender_name,sort_at) VALUES($1,$2,$3,$4,'archived','',now()+$5*interval '1 second')`, tenant, card, account, fmt.Sprint(i), i); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,body_hash,received_at) VALUES($1,$2,$3,$4,''::bytea,now())`, tenant, uuid.New(), card, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	var requests, labelChecks atomic.Int32
	gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "x", "expires_in": 3600})
		case strings.HasSuffix(r.URL.Path, "/history"):
			json.NewEncoder(w).Encode(map[string]any{"historyId": "1", "history": []any{}})
		case strings.Contains(r.URL.Path, "/threads/"):
			labelChecks.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"messages": []any{map[string]any{"labelIds": []string{}}}})
		case strings.Contains(r.URL.Path, "/messages/"):
			requests.Add(1)
			if strings.HasSuffix(r.URL.Path, "/11") {
				http.Error(w, "gone", 404)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte("Content-Type: text/plain\r\n\r\nrestored"))})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gmail.Close()
	c := &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "x"}
	s := &Syncer{DB: db, Client: func(Account) *Client { return c }}
	a := Account{TenantID: tenant, ID: account}
	if err := s.SyncAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 10 {
		t.Fatalf("first repair requests=%d, want cap 10", requests.Load())
	}
	if err := s.SyncAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 12 {
		t.Fatalf("unchecked cards starved or failed immediately retried: %d", requests.Load())
	}
	if labelChecks.Load() != 12 {
		t.Fatalf("label reconciliation checks=%d, want 12 exactly once alongside content repair", labelChecks.Load())
	}
	var repaired, changed int
	if err := db.QueryRow(ctx, `SELECT count(*) FILTER(WHERE content_version=$2) FROM messages WHERE tenant_id=$1`, tenant, mailContentVersion).Scan(&repaired); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND state<>'archived'`, tenant).Scan(&changed); err != nil {
		t.Fatal(err)
	}
	if repaired != 11 || changed != 0 {
		t.Fatalf("repaired=%d state changes=%d", repaired, changed)
	}
}

type contentImageResolver struct{}

func (contentImageResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
}

type contentImageDialer struct{ address string }

func (d contentImageDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, d.address)
}

func TestContentRepairRetriesUnavailableImageFromStoredRaw(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account, card, message := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,sender_name) VALUES($1,$2,$3,'image-thread','archived','')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	raw := []byte("Content-Type: text/html\r\n\r\n<p>original prose</p><img src=\"http://images.example.test/p.png\" width=\"300\" height=\"200\">")
	if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,body_hash,received_at,content_version,raw_mime,html) VALUES($1,$2,$3,'image-message',''::bytea,now(),1,$4,'<p>original prose</p><img data-original-src="http://images.example.test/p.png" data-fetch-state="unavailable">')`, tenant, message, card, raw); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("recovered image"))
	}))
	defer image.Close()
	s := &Syncer{DB: db, Images: mailhtml.NewImageFetcher(contentImageResolver{}, contentImageDialer{strings.TrimPrefix(image.URL, "http://")})}
	// No Gmail client exists: the retained raw original is the retry input.
	if err := s.backfillContent(ctx, Account{TenantID: tenant, ID: account}, nil); err != nil {
		t.Fatal(err)
	}
	var body, state string
	if err := db.QueryRow(ctx, `SELECT m.html,c.state FROM messages m JOIN cards c ON c.id=m.card_id WHERE m.id=$1`, message).Scan(&body, &state); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || !strings.Contains(body, `src="data:image/png;base64,`) || strings.Contains(body, `data-fetch-state="unavailable"`) || state != "archived" {
		t.Fatalf("retry requests=%d body=%s state=%s", requests.Load(), body, state)
	}
	if err := s.backfillContent(ctx, Account{TenantID: tenant, ID: account}, nil); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatal("successful image fetched twice")
	}
}

func TestCIDImageReferencesResolveWholeIdentifier(t *testing.T) {
	source := `<p>cid:photo stays text</p><img src="cid:photo"><img src="CID:photo-large"><img src="cid:photo%40mail">`
	got := resolveCIDImages(source, map[string]mailhtml.Part{
		"photo":       {ContentType: "image/png", Data: []byte("first")},
		"photo-large": {ContentType: "image/png", Data: []byte("second")},
		"photo@mail":  {ContentType: "image/png", Data: []byte("third")},
	})
	for _, want := range []string{"cid:photo stays text", "Zmlyc3Q=", "c2Vjb25k", "dGhpcmQ="} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s: %s", want, got)
		}
	}
	if strings.Contains(got, `src="cid:`) || strings.Contains(got, `src="CID:`) {
		t.Fatal(got)
	}
}
