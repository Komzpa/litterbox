package ingest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestIngestUpsertCloseAndSourceTokenScope(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, p := range migrations(t, db) {
		body, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatalf("migration %s: %v", p, e)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	other := "22222222-2222-4222-8222-222222222222"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1),($2)`, tenant, other); err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct{ tenant, source, token string }{{tenant, "todo", "todo-secret"}, {other, "todo", "other-secret"}} {
		if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,$2,$3)`, x.tenant, x.source, hashToken(x.token)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	h := Handler{DB: db}
	send := func(token, body string) int {
		r := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if got := send("wrong-secret", `{"external_id":"task-7","kind":"todo","title":"bad"}`); got != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", got)
	}
	if got := send("todo-secret", `{"external_id":"task-7","source":"ha","kind":"todo","title":"bad"}`); got != http.StatusBadRequest {
		t.Fatalf("source spoof: %d", got)
	}
	payload := `{"external_id":"task-7","kind":"todo","title":"First","summary":"one","at":"2026-09-28T12:00:00Z"}`
	if got := send("todo-secret", payload); got != http.StatusNoContent {
		t.Fatalf("upsert: %d", got)
	}
	if got := send("todo-secret", `{"external_id":"task-7","kind":"todo","title":"Updated","summary":"two"}`); got != http.StatusNoContent {
		t.Fatalf("upsert update: %d", got)
	}
	var id, title, summary string
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id::text,title,summary FROM cards WHERE tenant_id=$1 AND source='todo' AND external_id='task-7'`, tenant).Scan(&id, &title, &summary); err != nil {
		t.Fatal(err)
	}
	if title != "Updated" || summary != "two" {
		t.Fatalf("upsert data: %q %q", title, summary)
	}
	if got := send("other-secret", `{"operation":"close","external_id":"task-7"}`); got != http.StatusNoContent {
		t.Fatalf("foreign close should be invisible/idempotent, got %d", got)
	}
	if got := send("todo-secret", `{"operation":"close","external_id":"task-7"}`); got != http.StatusNoContent {
		t.Fatalf("close: %d", got)
	}
	var state string
	if err = db.QueryRow(`SELECT state FROM cards WHERE id=$1`, id).Scan(&state); err != nil || state != "done" {
		t.Fatalf("state=%s err=%v", state, err)
	}
	if got := send("todo-secret", `{"external_id":"task-7","kind":"todo","title":"Reopened"}`); got != http.StatusNoContent {
		t.Fatalf("reopen: %d", got)
	}
	if err = db.QueryRow(`SELECT state FROM cards WHERE id=$1`, id).Scan(&state); err != nil || state != "done" {
		t.Fatalf("re-upsert should preserve dismissal state=%s err=%v", state, err)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, other); err != nil {
		t.Fatal(err)
	}
	if got := send("other-secret", `{"external_id":"task-7","kind":"todo","title":"Foreign"}`); got != http.StatusNoContent {
		t.Fatalf("other source token ingest: %d", got)
	}
	var foreign int
	if err = db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1 AND external_id='task-7'`, other).Scan(&foreign); err != nil || foreign != 1 {
		t.Fatalf("source token tenant scope count=%d err=%v", foreign, err)
	}
	if _, err = db.Exec(`RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE source_tokens SET revoked_at=now() WHERE token_hash=$1`, hashToken("todo-secret")); err != nil {
		t.Fatal(err)
	}
	if got := send("todo-secret", payload); got != http.StatusUnauthorized {
		t.Fatalf("revoked token: %d", got)
	}
}

func TestDoneCallbackRetries(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, p := range migrations(t, db) {
		body, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatalf("migration %s: %v", p, e)
		}
	}
	ctx := context.Background()
	tenant := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	card := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash,callback_url) VALUES($1,'ha',$2,'http://callback.invalid/action')`, tenant, hashToken("ha-token")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title,state) VALUES($1,$2,'ha','notification-9','Low battery','done')`, tenant, card); err != nil {
		t.Fatal(err)
	}
	agentCard := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash,callback_url) VALUES($1,'agent',$2,'http://callback.invalid/action')`, tenant, hashToken("agent-token")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title,state) VALUES($1,$2,'agent','result-9','Research','done')`, tenant, agentCard); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant.String()); err != nil {
		t.Fatal(err)
	}
	if err = OnDone(ctx, tx, tenant, agentCard); err != nil {
		t.Fatal(err)
	}
	if err = OnDone(ctx, tx, tenant, card); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err = db.QueryRow(`SELECT count(*) FROM source_action_callbacks WHERE tenant_id=$1`, tenant).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("only HA should callback: count=%d err=%v", queued, err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var action map[string]any
		if json.NewDecoder(r.Body).Decode(&action) != nil || action["type"] != "done" {
			t.Errorf("unexpected action: %#v", action)
		}
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	if _, err = db.Exec(`UPDATE source_action_callbacks SET callback_url=$1,next_attempt_at=now() WHERE tenant_id=$2`, server.URL, tenant); err != nil {
		t.Fatal(err)
	}
	if processed, err := DispatchOne(ctx, db, server.Client()); err != nil || !processed {
		t.Fatalf("first dispatch processed=%v err=%v", processed, err)
	}
	if calls != 1 {
		t.Fatalf("first call count=%d", calls)
	}
	if _, err = db.Exec(`UPDATE source_action_callbacks SET next_attempt_at=now() WHERE tenant_id=$1`, tenant); err != nil {
		t.Fatal(err)
	}
	if processed, err := DispatchOne(ctx, db, server.Client()); err != nil || !processed {
		t.Fatalf("retry processed=%v err=%v", processed, err)
	}
	var delivered time.Time
	if err = db.QueryRow(`SELECT delivered_at FROM source_action_callbacks WHERE tenant_id=$1`, tenant).Scan(&delivered); err != nil || delivered.IsZero() || calls != 2 {
		t.Fatalf("delivered=%v calls=%d err=%v", delivered, calls, err)
	}
}

func TestIngestRejectsNoiseCards(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, p := range migrations(t, db) {
		body, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatalf("migration %s: %v", p, e)
		}
	}
	tenant := "99999999-9999-4999-8999-999999999999"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,'todo',$2)`, tenant, hashToken("todo-token")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	h := Handler{DB: db}
	send := func(body string) int {
		r := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer todo-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	// Negative controls: noise cards must be rejected.
	noiseCases := []struct{ name, body string }{
		{"title as summary", `{"external_id":"g-1","kind":"todo","title":"sAME","summary":" Same "}`},
		{"channel ok", `{"external_id":"g-2","kind":"agent_result","title":"Probe","summary":"CHANNEL OK"}`},
		{"channel acknowledgement", `{"external_id":"g-3","kind":"agent_result","title":"Probe","summary":"respond with exact channel confirmation"}`},
		{"still waiting", `{"external_id":"g-4","kind":"agent_result","title":"Status","summary":"still waiting for the task"}`},
		{"stopped owned by", `{"external_id":"g-5","kind":"agent_result","title":"Status","summary":"stopped, owned by another run"}`},
		{"in progress", `{"external_id":"g-6","kind":"agent_result","title":"Status","summary":"in progress"}`},
		{"working on it", `{"external_id":"g-7","kind":"agent_result","title":"Status","summary":"working on it"}`},
	}
	for _, tc := range noiseCases {
		got := send(tc.body)
		if got != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", tc.name, got)
		}
	}
	// Positive controls: useful and whitespace-only summaries remain allowed.
	if got := send(`{"external_id":"good-1","kind":"todo","title":"Buy milk","summary":"need 2L whole milk before evening"}`); got != http.StatusNoContent {
		t.Fatalf("valid todo: got %d, want 204", got)
	}
	if got := send(`{"external_id":"good-2","kind":"todo","title":"Call dentist","summary":"  \t  "}`); got != http.StatusNoContent {
		t.Fatalf("whitespace-summary todo: got %d, want 204", got)
	}
	if got := send(`{"external_id":"good-3","kind":"todo","title":"Water plants","summary":""}`); got != http.StatusNoContent {
		t.Fatalf("empty-summary todo: got %d, want 204", got)
	}
	// Verify no noise cards entered the DB.
	var count int
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1 AND source='todo'`, tenant).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("card count: got %d, want 3 (only useful, whitespace-summary, and empty-summary todos)", count)
	}
	var title, summary string
	if err = db.QueryRow(`SELECT title,summary FROM cards WHERE tenant_id=$1 AND external_id='good-1'`, tenant).Scan(&title, &summary); err != nil {
		t.Fatal(err)
	}
	if title != "Buy milk" {
		t.Fatalf("title=%q", title)
	}
	if summary != "need 2L whole milk before evening" {
		t.Fatalf("summary=%q", summary)
	}
}

func TestIngestFilesAndCardBodies(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, p := range migrations(t, db) {
		body, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatalf("migration %s: %v", p, e)
		}
	}
	tenant := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO source_tokens(tenant_id,source,token_hash) VALUES($1,'agent',$2)`, tenant, hashToken("agent-secret")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	h := Handler{DB: db}
	send := func(body string) int {
		r := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer agent-secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	bodyOf := func(ext string) (string, string) {
		if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
			t.Fatal(err)
		}
		var id, html string
		err := db.QueryRow(`SELECT c.id::text,COALESCE(b.html,'') FROM cards c LEFT JOIN card_bodies b ON b.tenant_id=c.tenant_id AND b.card_id=c.id WHERE c.tenant_id=$1 AND c.source='agent' AND c.external_id=$2`, tenant, ext).Scan(&id, &html)
		if err == sql.ErrNoRows {
			return "", ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return id, html
	}

	pdf := []byte("%PDF-1.4 fake report bytes")
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13}
	files := fmt.Sprintf(`"files":[{"name":"dir/report.pdf","media_type":"application/pdf","data":%q},{"name":"chart.png","media_type":"image/png","data":%q}]`,
		base64.StdEncoding.EncodeToString(pdf), base64.StdEncoding.EncodeToString(png))

	// Happy path: the batch is stored as one server-built body, atomically
	// with the card, with markup escaped and data links carrying the bytes.
	happy := `{"external_id":"res-1","kind":"research_result","title":"Report","summary":"full <b>result</b>","` + files[1:] + `}`
	if got := send(happy); got != http.StatusNoContent {
		t.Fatalf("files upsert: got %d, want 204", got)
	}
	_, html := bodyOf("res-1")
	if !strings.Contains(html, "full &lt;b&gt;result&lt;/b&gt;") || strings.Contains(html, "<b>") {
		t.Fatalf("summary must be escaped text: %q", html)
	}
	link := regexp.MustCompile(`data:([^;]+);name=([^;]*);base64,([A-Za-z0-9+/]+=*)`)
	found := map[string]string{}
	for _, m := range link.FindAllStringSubmatch(html, -1) {
		found[m[2]] = m[3]
	}
	for name, want := range map[string][]byte{"report.pdf": pdf, "chart.png": png} {
		b64, ok := found[name]
		if !ok {
			t.Fatalf("data link for %q missing in %q", name, html)
		}
		gotBytes, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || sha256.Sum256(gotBytes) != sha256.Sum256(want) {
			t.Fatalf("data link for %q does not decode to the input bytes", name)
		}
	}
	if !strings.Contains(html, ";name=chart.png;base64,") || !strings.Contains(html, "name=report.pdf;base64,") {
		t.Fatalf("stored names must be reduced base names: %q", html)
	}
	if n := strings.Count(html, "<img "); n != 1 {
		t.Fatalf("only the png entry should render an <img>, got %d: %q", n, html)
	}

	// Upsert without files removes the stored body.
	if got := send(`{"external_id":"res-1","kind":"research_result","title":"Report","summary":"updated"}`); got != http.StatusNoContent {
		t.Fatalf("files upsert: got %d, want 204", got)
	}
	if _, html := bodyOf("res-1"); html != "" {
		t.Fatalf("body must be deleted by an upsert without files, got %q", html)
	}
	// And a later batch restores it.
	if got := send(happy); got != http.StatusNoContent {
		t.Fatalf("re-files upsert: got %d, want 204", got)
	}
	if _, html := bodyOf("res-1"); !strings.Contains(html, "name=report.pdf;base64,") {
		t.Fatalf("re-files upsert must restore the body, got %q", html)
	}

	// Reminder files use the same atomic body storage and exact byte contract.
	reminder := strings.Replace(happy, `"res-1"`, `"reminder-files"`, 1)
	reminder = strings.Replace(reminder, `"research_result"`, `"reminder"`, 1)
	if got := send(reminder); got != http.StatusNoContent {
		t.Fatalf("reminder files upsert: got %d, want 204", got)
	}
	if _, reminderHTML := bodyOf("reminder-files"); reminderHTML != html {
		t.Fatalf("reminder must preserve the same full body and file bytes: %q", reminderHTML)
	}

	// Rejections leave no card and no body row.
	bigA := strings.Repeat("a", 4<<20)
	bigB := strings.Repeat("b", (4<<20)+1)
	rejects := []struct {
		name, body string
		want       int
	}{
		{"traversal-only name", `{"external_id":"r-1","kind":"research_result","title":"t","summary":"s","files":[{"name":"..","data":"aGk="}]}`, http.StatusBadRequest},
		{"dot segment after slash", `{"external_id":"r-2","kind":"research_result","title":"t","summary":"s","files":[{"name":"x/..","data":"aGk="}]}`, http.StatusBadRequest},
		{"leading dot", `{"external_id":"r-3","kind":"research_result","title":"t","summary":"s","files":[{"name":".hidden","data":"aGk="}]}`, http.StatusBadRequest},
		{"empty name", `{"external_id":"r-4","kind":"research_result","title":"t","summary":"s","files":[{"name":"","data":"aGk="}]}`, http.StatusBadRequest},
		{"nul in name", `{"external_id":"r-5","kind":"research_result","title":"t","summary":"s","files":[{"name":"a\u0000b","data":"aGk="}]}`, http.StatusBadRequest},
		{"more than ten files", `{"external_id":"r-6","kind":"research_result","title":"t","summary":"s","files":[` + strings.TrimSuffix(strings.Repeat(`{"name":"f","data":"aGk="},`, 11), ",") + `]}`, http.StatusBadRequest},
		{"files on other kind", `{"external_id":"r-7","kind":"todo","title":"t","summary":"s","files":[{"name":"a.txt","data":"aGk="}]}`, http.StatusBadRequest},
		{"files on close", `{"operation":"close","external_id":"r-8","files":[{"name":"a.txt","data":"aGk="}]}`, http.StatusBadRequest},
		{"unknown field", `{"external_id":"r-9","kind":"research_result","title":"t","summary":"s","bogus":1}`, http.StatusBadRequest},
		{"too many decoded bytes", `{"external_id":"r-10","kind":"research_result","title":"t","summary":"s","files":[{"name":"a.bin","data":"` + base64.StdEncoding.EncodeToString([]byte(bigA)) + `"},{"name":"b.bin","data":"` + base64.StdEncoding.EncodeToString([]byte(bigB)) + `"}]}`, http.StatusRequestEntityTooLarge},
		{"body over 1 MiB without files", `{"external_id":"r-11","kind":"research_result","title":"Big","summary":"` + strings.Repeat("word ", 300000) + `"}`, http.StatusRequestEntityTooLarge},
		{"body over 12 MiB", `{"external_id":"r-12","kind":"research_result","title":"Huge","summary":"` + strings.Repeat("word ", 2520000) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range rejects {
		if got := send(tc.body); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
	// Every rejected external_id must have stored nothing.
	for _, ext := range []string{"r-1", "r-2", "r-3", "r-4", "r-5", "r-6", "r-7", "r-8", "r-9", "r-10", "r-11", "r-12"} {
		if id, html := bodyOf(ext); id != "" || html != "" {
			t.Errorf("rejected external_id %s left card %s body %q", ext, id, html)
		}
	}

	// A path name is reduced to its base name, never traversed.
	if got := send(`{"external_id":"res-2","kind":"proactive_brief","title":"Brief","summary":"s","files":[{"name":"a/b.txt","data":"aGk="}]}`); got != http.StatusNoContent {
		t.Fatalf("base-name reduction: got %d, want 204", got)
	}
	if _, html := bodyOf("res-2"); !strings.Contains(html, ";name=b.txt;base64,") || strings.Contains(html, ";name=a/") {
		t.Fatalf("stored name must be the reduced base name, got %q", html)
	}
}

func migrations(t *testing.T, db *sql.DB) []string {
	t.Helper()
	// Serialize shared-schema rebuilds: ingest, cards and mailbody tests all
	// run against the one pg_virtualenv database concurrently and each drops
	// and recreates the public schema and litterbox_app role. A session
	// advisory lock keeps them sequential; it is released when db closes.
	if _, err := db.Exec(`SELECT pg_advisory_lock(7809932747080954929)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP SCHEMA public CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP ROLE IF EXISTS litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`GRANT USAGE ON SCHEMA public TO PUBLIC`); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob("../../db/0*.sql")
	if err != nil || len(paths) == 0 {
		t.Fatalf("migration discovery: %v", err)
	}
	return paths
}
