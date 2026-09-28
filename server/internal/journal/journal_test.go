package journal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMCPPostgresScopesRevocationAndTools(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
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
		if _, e = db.Exec(string(b)); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := Store{DB: pool}
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
	}
	if w := call("tools/call", `{"name":"journal_append","arguments":{"body":"hello"}}`); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Fatalf("tools/call response %d %s", w.Code, w.Body.String())
	}
	if w := call("tools/call", `{"name":"card_create","arguments":{"title":"from MCP","summary":"created"}}`); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Fatalf("card_create response %d %s", w.Code, w.Body.String())
	}
	var cardCount int
	if err := db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1 AND source='mcp' AND title='from MCP'`, tenant).Scan(&cardCount); err != nil || cardCount != 1 {
		t.Fatalf("created MCP card count=%d err=%v", cardCount, err)
	}
	// Replace scopes with read-only and prove append is denied while read remains available.
	sum := sha256sum(token)
	if _, err = db.Exec(`UPDATE mcp_tokens SET scopes=$1 WHERE token_hash=$2`, []string{"journal:read"}, sum); err != nil {
		t.Fatal(err)
	}
	if w := call("tools/call", `{"name":"journal_append","arguments":{"body":"denied"}}`); !bytes.Contains(w.Body.Bytes(), []byte("scope denied")) {
		t.Fatalf("scope enforcement: %s", w.Body.String())
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
func sha256sum(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
