package journal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"database/sql"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMCPPostgresScopesRevocationAndTools(t *testing.T) {
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
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "012_journal.sql"} {
		b, e := os.ReadFile(filepath.Join("../../db", name))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(tdb.Migration(string(b))); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := Store{DB: pool, Cards: SQLCardIngest{DB: pool}}
	mux := http.NewServeMux()
	s.Register(mux)
	id, token, err := s.CreateToken(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, params string) *httptest.ResponseRecorder {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + `}`
		r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := call("tools/list", `{}`); w.Code != 200 {
		t.Fatalf("tools/list status %d", w.Code)
	} else {
		var v map[string]any
		if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["result"] == nil {
			t.Fatalf("tools/list response %s", w.Body.String())
		}
		encoded, _ := json.Marshal(v["result"])
		if bytes.Contains(encoded, []byte("journal_append")) {
			t.Fatalf("tools/list exposed journal_append: %s", encoded)
		}
	}
	if w := call("tools/call", `{"name":"journal_append","arguments":{"body":"forbidden"}}`); !bytes.Contains(w.Body.Bytes(), []byte("unknown tool")) {
		t.Fatalf("append must be rejected as unknown tool: %s", w.Body.String())
	}
	if w := call("tools/call", `{"name":"journal_read","arguments":{}}`); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Fatalf("journal_read response %d %s", w.Code, w.Body.String())
	}
	for _, args := range []string{
		`{"external_id":"assistant-task-17","title":"from MCP","summary":"created"}`,
		`{"external_id":"assistant-task-17","title":"updated by retry","summary":"updated"}`,
	} {
		if w := call("tools/call", `{"name":"card_create","arguments":`+args+`}`); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
			t.Fatalf("card_create response %d %s", w.Code, w.Body.String())
		}
	}
	var cardCount int
	var title, summary string
	if err := db.QueryRow(`SELECT count(*),min(title),min(summary) FROM cards WHERE tenant_id=$1 AND source='mcp' AND external_id='assistant-task-17'`, tenant).Scan(&cardCount, &title, &summary); err != nil || cardCount != 1 || title != "updated by retry" || summary != "updated" {
		t.Fatalf("MCP card upsert count=%d title=%q summary=%q err=%v", cardCount, title, summary, err)
	}
	var scopes string
	if err := db.QueryRow(`SELECT scopes::text FROM mcp_tokens WHERE id=$1`, id).Scan(&scopes); err != nil {
		t.Fatal(err)
	}
	if scopes != "{journal:read,cards:create}" {
		t.Fatalf("unexpected MCP scopes: %s", scopes)
	}

	if err = s.RevokeToken(context.Background(), tenant, id); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status=%d body=%s", w.Code, w.Body.String())
	}
}
