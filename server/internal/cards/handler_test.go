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
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresNotePersistence(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "015_card_note_updated.sql"} {
		body, err := os.ReadFile(filepath.Join("../../db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	other := "22222222-2222-4222-8222-222222222222"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1),($2)`, tenant, other); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	rows := []todos.Card{{Source: "todo", ExternalID: "synthetic-task", Title: "Fix generated task", State: "open", Order: 0}}
	if err = todos.Upsert(context.Background(), db, tenant, rows); err != nil {
		t.Fatal(err)
	}
	if err = todos.Upsert(context.Background(), db, other, rows); err != nil {
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
	separate, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer separate.Close()
	separate.SetMaxOpenConns(1)
	if _, err = separate.Exec(`SET ROLE litterbox_app`); err != nil {
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
