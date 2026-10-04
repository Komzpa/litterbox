package cards

import (
	"bufio"
	"bytes"
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

	"github.com/Komzpa/litterbox/server/internal/sources/todos"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresNotePersistence(t *testing.T) {
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
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "008_bundles.sql", "009_ingest.sql", "011_card_bodies.sql", "015_card_note_updated.sql", "017_card_sender_name.sql"} {
		body, err := os.ReadFile(filepath.Join("../../db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	other := "22222222-2222-4222-8222-222222222222"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1),($2)`, tenant, other); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	rows := []todos.Card{{Source: "todo", ExternalID: "synthetic-task", Title: "Fix generated task", State: "open", Order: 0}}
	if err = todos.Upsert(context.Background(), db, tenant, rows); err != nil {
		t.Fatal(err)
	}
	if err = todos.Upsert(context.Background(), db, other, rows); err != nil {
		t.Fatal(err)
	}
	bundleID := "33333333-3333-4333-8333-333333333333"
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Synthetic bundle',ARRAY[]::real[])`, tenant, bundleID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE cards SET bundle_id=$1,pinned_rank=2 WHERE tenant_id=$2 AND source='todo'`, bundleID, tenant); err != nil {
		t.Fatal(err)
	}
	sink := filepath.Join(t.TempDir(), "feedback.md")
	handler, err := NewHandler(db, "Asia/Tbilisi", sink)
	if err != nil {
		t.Fatal(err)
	}
	events := NewEvents("")
	eventsCtx, stopEvents := context.WithCancel(context.Background())
	defer stopEvents()
	go events.Run(eventsCtx)
	handler.Events = events
	mux := http.NewServeMux()
	handler.Routes(mux)
	request := func(tenant, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req = req.WithContext(WithTenant(req.Context(), tenant))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	streamMux := http.NewServeMux()
	handler.Routes(streamMux)
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamMux.ServeHTTP(w, r.WithContext(WithTenant(r.Context(), tenant)))
	}))
	defer streamServer.Close()
	response, err := http.Get(streamServer.URL + "/v1/cards/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	response2, err := http.Get(streamServer.URL + "/v1/cards/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response2.Body.Close()
	readEvent := func(response *http.Response) {
		t.Helper()
		eventReader := bufio.NewReader(response.Body)
		deadline := time.After(2 * time.Second)
		done := make(chan error, 1)
		go func() {
			for {
				line, err := eventReader.ReadString('\n')
				if err != nil {
					done <- err
					return
				}
				if strings.TrimSpace(line) == "event: cards" {
					done <- nil
					return
				}
			}
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("timed out waiting for cards event")
		}
	}
	readEvent(response)
	readEvent(response2)
	get := func(tenant string) Sections {
		t.Helper()
		w := request(tenant, "GET", "/v1/cards?now=2026-09-28T15:00:00%2B04:00", "")
		if w.Code != 200 {
			t.Fatalf("GET: %d %s", w.Code, w.Body.String())
		}
		var data Sections
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	mine := get(tenant).Now
	foreign := get(other).Now
	if len(mine) != 1 || len(foreign) != 1 || mine[0].ID == foreign[0].ID || mine[0].At != nil || mine[0].Timed {
		t.Fatalf("card identity and time: %v / %v", mine, foreign)
	}
	if mine[0].BundleID == nil || *mine[0].BundleID != bundleID || mine[0].PinnedRank == nil || *mine[0].PinnedRank != 2 {
		t.Fatalf("card metadata bundle_id=%v pinned_rank=%v", mine[0].BundleID, mine[0].PinnedRank)
	}
	if foreign[0].BundleID != nil || foreign[0].PinnedRank != nil {
		t.Fatalf("foreign card metadata leaked: bundle_id=%v pinned_rank=%v", foreign[0].BundleID, foreign[0].PinnedRank)
	}
	wire := request(other, "GET", "/v1/cards?now=2026-09-28T15:00:00%2B04:00", "")
	var payload struct {
		Now []map[string]json.RawMessage `json:"now"`
	}
	if wire.Code != http.StatusOK || json.Unmarshal(wire.Body.Bytes(), &payload) != nil || len(payload.Now) != 1 {
		t.Fatalf("metadata wire response=%d %s", wire.Code, wire.Body.String())
	}
	for _, key := range []string{"bundle_id", "pinned_rank"} {
		if value, ok := payload.Now[0][key]; !ok || string(value) != "null" {
			t.Fatalf("unassigned %s must be explicit null, got %s (present=%t)", key, value, ok)
		}
	}
	id := mine[0].ID
	if status := request(tenant, "GET", "/v1/cards?tz=Not%2FA%2FZone", "").Code; status != 400 {
		t.Fatalf("invalid timezone status=%d", status)
	}
	if w := request(other, "POST", "/v1/cards/"+id+"/note", `{"text":"wrong tenant"}`); w.Code != 404 {
		t.Fatalf("foreign note status=%d", w.Code)
	}
	if w := request(tenant, "POST", "/v1/cards/"+id+"/note", `{"text":"not actionable"}`); w.Code != 204 {
		t.Fatalf("note status=%d body=%s", w.Code, w.Body.String())
	}
	readEvent(response)
	readEvent(response2)
	if got := get(tenant).Now[0].Note; got != "not actionable" {
		t.Fatalf("persisted note=%q", got)
	}
	noteFeed := request(tenant, "GET", "/v1/cards/notes?since=2000-01-01T00:00:00Z", "")
	if noteFeed.Code != 200 || !strings.Contains(noteFeed.Body.String(), "not actionable") || !strings.Contains(noteFeed.Body.String(), id) {
		t.Fatalf("note feed=%d %s", noteFeed.Code, noteFeed.Body.String())
	}
	foreignFeed := request(other, "GET", "/v1/cards/notes?since=2000-01-01T00:00:00Z", "")
	if foreignFeed.Code != 200 || strings.Contains(foreignFeed.Body.String(), "not actionable") {
		t.Fatalf("foreign note feed=%d %s", foreignFeed.Code, foreignFeed.Body.String())
	}

	if err = todos.Upsert(context.Background(), db, tenant, rows); err != nil {
		t.Fatal(err)
	}
	if got := get(tenant).Now[0].Note; got != "not actionable" {
		t.Fatalf("upsert lost note=%q", got)
	}
	if w := request(tenant, "POST", "/v1/cards/"+id+"/dismiss", `{"note":"do not repeat"}`); w.Code != 204 {
		t.Fatalf("dismiss status=%d body=%s", w.Code, w.Body.String())
	}
	readEvent(response)
	readEvent(response2)

	if len(get(tenant).Now) != 0 || len(get(other).Now) != 1 {
		t.Fatal("dismiss visibility or tenant isolation failed")
	}
	separate, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer separate.Close()
	separate.SetMaxOpenConns(1)
	if _, err = separate.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	if _, err = separate.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = separate.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title) VALUES ($1,$2,'todo',$3,$4)`, tenant, "33333333-3333-4333-8333-333333333333", "event-separate", "Separate insert"); err != nil {
		t.Fatal(err)
	}
	readEvent(response)
	readEvent(response2)
	if err = todos.Upsert(context.Background(), db, tenant, rows); err != nil {
		t.Fatal(err)
	}
	afterUpsert := get(tenant).Now
	if len(afterUpsert) != 1 || afterUpsert[0].ID == id {
		t.Fatal("upsert resurrected dismissed card or lost separate insert")
	}
	data, err := os.ReadFile(sink)
	if err != nil {
		t.Fatal(err)
	}
	want := "- [ ] Fix generated task — invalid: not actionable\n- [ ] Fix generated task — invalid: do not repeat\n"
	if string(data) != want {
		t.Fatalf("feedback=%q want=%q", data, want)
	}
}

func TestImportantCardSurfacesImportance(t *testing.T) {
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
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "008_bundles.sql", "009_ingest.sql", "011_card_bodies.sql", "015_card_note_updated.sql", "017_card_sender_name.sql"} {
		body, err := os.ReadFile(filepath.Join("../../db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	account := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	importantCard := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	ordinaryCard := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'user@example.com',decode('00','hex'))`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject) VALUES($1,$2,$3,'thread-important','Important thread'),($1,$4,$3,'thread-ordinary','Ordinary thread')`, tenant, importantCard, account, ordinaryCard); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,body_hash,received_at) VALUES($1,gen_random_uuid(),$2,'m1',ARRAY['IMPORTANT','CATEGORY_PERSONAL'],decode('00','hex'),now()),($1,gen_random_uuid(),$3,'m2',ARRAY['CATEGORY_PERSONAL'],decode('00','hex'),now())`, tenant, importantCard, ordinaryCard); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(db, "Asia/Tbilisi", filepath.Join(t.TempDir(), "feedback.md"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Routes(mux)
	req := httptest.NewRequest("GET", "/v1/cards?now=2026-09-28T15:00:00%2B04:00", nil)
	req = req.WithContext(WithTenant(req.Context(), tenant))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	var data Sections
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Card{}
	for _, c := range data.Now {
		byID[c.ID] = c
	}
	for _, c := range data.Later {
		byID[c.ID] = c
	}
	for _, c := range data.Missed {
		byID[c.ID] = c
	}
	if c, ok := byID[importantCard]; !ok || !c.Important {
		t.Fatalf("IMPORTANT-labeled card must surface Important=true, got %+v", c)
	}
	if c, ok := byID[ordinaryCard]; !ok || c.Important {
		t.Fatalf("ordinary card must surface Important=false, got %+v", c)
	}
}

func TestCardArrivalAndSenderAddress(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Serialize shared-schema rebuilds across concurrently-run test packages
	// sharing one pg_virtualenv database (released when db closes).
	if _, err = db.Exec(`SELECT pg_advisory_lock(7809932747080954929)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP SCHEMA public CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP ROLE IF EXISTS litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`GRANT USAGE ON SCHEMA public TO PUBLIC`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "008_bundles.sql", "009_ingest.sql", "011_card_bodies.sql", "015_card_note_updated.sql", "017_card_sender_name.sql"} {
		body, err := os.ReadFile(filepath.Join("../../db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	account := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	mailCard := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	manualCard := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'user@example.com',decode('00','hex'))`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'thread-arrival','Cerebras report','Cerebras Systems <welcome@cerebras.net>')`, tenant, mailCard, account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,subject) VALUES($1,$2,'manual','arrival-task','Manual task')`, tenant, manualCard); err != nil {
		t.Fatal(err)
	}
 	if _, err = db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,text,body_hash,received_at) VALUES($1,gen_random_uuid(),$2,'m-old',ARRAY['CATEGORY_PERSONAL'],'old body',decode('00','hex'),'2026-10-04T06:00:00Z'),($1,gen_random_uuid(),$2,'m-new',ARRAY['CATEGORY_PERSONAL'],'Latest thread preview',decode('00','hex'),'2026-10-04T07:43:00Z')`, tenant, mailCard); err != nil {
 		t.Fatal(err)
 	}
	handler, err := NewHandler(db, "Asia/Tbilisi", filepath.Join(t.TempDir(), "feedback.md"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Routes(mux)
	req := httptest.NewRequest("GET", "/v1/cards?now=2026-10-04T12:00:00%2B04:00", nil)
	req = req.WithContext(WithTenant(req.Context(), tenant))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	for _, section := range []string{"now", "later", "missed"} {
		for _, row := range data[section].([]any) {
			card := row.(map[string]any)
			rows[card["id"].(string)] = card
		}
	}
	mail, ok := rows[mailCard]
	if !ok {
		t.Fatalf("mail card missing from response: %v", rows)
	}
	if got := mail["received_at"]; got != "2026-10-04T07:43:00Z" {
		t.Fatalf("received_at=%v, want latest message arrival 2026-10-04T07:43:00Z as RFC3339 UTC", got)
	}
 	if got := mail["sender_address"]; got != "welcome@cerebras.net" {
 		t.Fatalf("sender_address=%v, want welcome@cerebras.net", got)
 	}
 	if got := mail["snippet"]; got != "Latest thread preview" {
 		t.Fatalf("snippet=%v, want newest message text", got)
 	}
	manual, ok := rows[manualCard]
	if !ok {
		t.Fatalf("manual card missing from response: %v", rows)
	}
	if got, exists := manual["received_at"]; exists {
		t.Fatalf("non-mail card must omit received_at, got %v", got)
	}
	if got, exists := manual["sender_address"]; exists {
		t.Fatalf("non-mail card must omit sender_address, got %v", got)
	}
}
