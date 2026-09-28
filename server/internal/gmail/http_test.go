package gmail

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func webTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("CARD_TEST_POSTGRES=1 required")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "006_mail_sync.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	return db
}

func TestWebOAuthStateTokenAndDisconnect(t *testing.T) {
	db := webTestDB(t)
	tenant, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{tenant, other} {
		if _, err := db.Exec("INSERT INTO tenants(id) VALUES($1)", id); err != nil {
			t.Fatal(err)
		}
	}
	provider := newFakeProvider(t)
	mux := http.NewServeMux()
	h := &WebHandler{DB: db, Config: WebConfig{Credentials: &Credentials{ClientID: "fake-client-id", ClientSecret: "fake-client-secret"}, CallbackURL: "https://example.test/v1/gmail/oauth/callback", AuthURL: provider.server.URL + "/auth", TokenURL: provider.server.URL + "/token", ProfileURL: provider.server.URL + "/profile", RevokeURL: provider.server.URL + "/revoke", StateSecret: []byte("12345678901234567890123456789012")}}
	h.Routes(mux)
	request := func(tenantID uuid.UUID, method, path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if tenantID != uuid.Nil {
			r = r.WithContext(WithTenant(r.Context(), tenantID.String()))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	connect := request(tenant, "POST", ConnectPath)
	if connect.Code != 200 {
		t.Fatalf("connect: %d %s", connect.Code, connect.Body.String())
	}
	var start struct {
		URL string `json:"authorization_url"`
	}
	if err := json.Unmarshal(connect.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	auth, err := url.Parse(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	state := auth.Query().Get("state")
	if state == "" || auth.Query().Get("scope") != ScopeGmailModify {
		t.Fatalf("auth URL: %s", start.URL)
	}
	if got := request(uuid.Nil, "GET", OAuthCallbackPath+"?state=corrupted&code=fake-code"); got.Code != 400 {
		t.Fatalf("bad state: %d", got.Code)
	}
	forged := strings.Split(state, ".")
	forged[0] = "invalid"
	if got := request(uuid.Nil, "GET", OAuthCallbackPath+"?state="+url.QueryEscape(strings.Join(forged, "."))+"&code=fake-code"); got.Code != 400 {
		t.Fatalf("forged state: %d", got.Code)
	}
	if provider.tokenCalls.Load() != 0 {
		t.Fatal("bad state exchanged a token")
	}
	if got := request(uuid.Nil, "GET", OAuthCallbackPath+"?state="+url.QueryEscape(state)+"&code=fake-code"); got.Code != 200 {
		t.Fatalf("callback: %d %s", got.Code, got.Body.String())
	}
	if provider.tokenCalls.Load() != 1 {
		t.Fatalf("token calls: %d", provider.tokenCalls.Load())
	}
	var id uuid.UUID
	var token []byte
	if err := db.QueryRowContext(context.Background(), "SELECT id,refresh_token FROM accounts WHERE tenant_id=$1 AND address=$2", tenant, "owner@example.org").Scan(&id, &token); err != nil {
		t.Fatal(err)
	}
	if string(token) != "rt-fake" {
		t.Fatalf("stored token: %q", token)
	}
	if got := request(uuid.Nil, "GET", OAuthCallbackPath+"?state="+url.QueryEscape(state)+"&code=fake-code"); got.Code != 400 {
		t.Fatalf("replayed state: %d", got.Code)
	}
	if got := request(other, "GET", AccountsPath); got.Code != 200 || strings.Contains(got.Body.String(), "owner@example.org") {
		t.Fatalf("other tenant accounts: %d %s", got.Code, got.Body.String())
	}
	if got := request(tenant, "GET", AccountsPath); got.Code != 200 || !strings.Contains(got.Body.String(), "owner@example.org") {
		t.Fatalf("account list: %d %s", got.Code, got.Body.String())
	}
	if got := request(other, "DELETE", AccountsPath+"/"+id.String()); got.Code != 404 {
		t.Fatalf("cross-tenant delete: %d", got.Code)
	}
	if got := request(tenant, "DELETE", AccountsPath+"/"+id.String()); got.Code != 204 {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
	if got := request(tenant, "GET", AccountsPath); got.Code != 200 || strings.Contains(got.Body.String(), "owner@example.org") {
		t.Fatalf("after delete: %d %s", got.Code, got.Body.String())
	}
}
