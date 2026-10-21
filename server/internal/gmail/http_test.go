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

	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func webTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("CARD_TEST_POSTGRES=1 required")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(b))); err != nil {
			t.Fatalf("migration %s: %v", file, err)
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

// TestPhotoConsentAccountBinding drives the incremental photo-consent flow
// against the fake provider and pins the acceptance contract: the consent
// state is bound to the intended account, the callback verifies the Google
// identity it receives against that account before storing anything, the
// recorded scopes are the token's actual authority (never a blind union of
// requested/stored scopes), the photo / Gmail-only / partial grant classes get
// distinct truthful pages, and a later Gmail-only reconnect preserves an
// existing photo grant instead of downgrading it.
func TestPhotoConsentAccountBinding(t *testing.T) {
	db := webTestDB(t)
	tenant := uuid.New()
	if _, err := db.Exec("INSERT INTO tenants(id) VALUES($1)", tenant); err != nil {
		t.Fatal(err)
	}
	provider := newFakeProvider(t)
	mux := http.NewServeMux()
	h := &WebHandler{DB: db, Config: WebConfig{Credentials: &Credentials{ClientID: "fake-client-id", ClientSecret: "fake-client-secret"}, CallbackURL: "https://example.test/v1/gmail/oauth/callback", AuthURL: provider.server.URL + "/auth", TokenURL: provider.server.URL + "/token", ProfileURL: provider.server.URL + "/profile", RevokeURL: provider.server.URL + "/revoke", StateSecret: []byte("12345678901234567890123456789012")}}
	h.Routes(mux)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(WithTenant(r.Context(), tenant.String()))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	stateOf := func(res *httptest.ResponseRecorder) string {
		t.Helper()
		if res.Code != 200 {
			t.Fatalf("start: %d %s", res.Code, res.Body.String())
		}
		var start struct {
			URL string `json:"authorization_url"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &start); err != nil {
			t.Fatal(err)
		}
		auth, err := url.Parse(start.URL)
		if err != nil {
			t.Fatal(err)
		}
		if auth.Query().Get("state") == "" {
			t.Fatalf("auth URL without state: %s", start.URL)
		}
		return auth.Query().Get("state")
	}
	callback := func(state string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", OAuthCallbackPath+"?state="+url.QueryEscape(state)+"&code=fake-code", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}

	// Gmail-only connect creates the account without photo scopes.
	provider.tokenBody = `{"access_token":"at-fake","refresh_token":"rt-gmail","token_type":"Bearer","expires_in":3600,"scope":"` + ScopeGmailModify + `"}`
	if got := call("POST", ConnectPath, ""); got.Code != 200 {
		t.Fatalf("connect: %d %s", got.Code, got.Body.String())
	}
	state := stateOf(call("POST", ConnectPath, ""))
	if got := callback(state); got.Code != 200 {
		t.Fatalf("connect callback: %d %s", got.Code, got.Body.String())
	}
	var accountID uuid.UUID
	var token []byte
	var scopes string
	if err := db.QueryRow("SELECT id,refresh_token,granted_photo_scopes FROM accounts WHERE tenant_id=$1 AND address=$2", tenant, "owner@example.org").Scan(&accountID, &token, &scopes); err != nil {
		t.Fatal(err)
	}
	if string(token) != "rt-gmail" || HasPhotoScopes(scopes) {
		t.Fatalf("gmail-only connect stored token=%q scopes=%q", token, scopes)
	}

	// Photo consent requires the intended account and requests the full scope
	// join; the state is bound to that account.
	if got := call("POST", PhotoConsentPath, ""); got.Code != 400 {
		t.Fatalf("accountless photo consent: %d %s", got.Code, got.Body.String())
	}
	if got := call("POST", PhotoConsentPath, `{"account_id":"` + uuid.NewString() + `"}`); got.Code != 400 {
		t.Fatalf("unknown-account photo consent: %d %s", got.Code, got.Body.String())
	}
	start := call("POST", PhotoConsentPath, `{"account_id":"`+accountID.String()+`"}`)
	state = stateOf(start)
	auth, _ := url.Parse(stateURL(start))
	if got := auth.Query().Get("scope"); got != FullPhotoScope() {
		t.Fatalf("photo consent scope = %q, want %q", got, FullPhotoScope())
	}

	// Wrong-account negative control: the Google identity differs from the
	// bound account. Nothing may be stored for it and the bound account must
	// keep its old token.
	provider.profileEmail = "intruder@example.org"
	provider.tokenBody = `{"access_token":"at-intruder","refresh_token":"rt-intruder","token_type":"Bearer","expires_in":3600,"scope":"` + FullPhotoScope() + `"}`
	got := callback(state)
	if got.Code == 200 {
		t.Fatalf("wrong-account callback stored a grant: %d %s", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "intruder@example.org") || !strings.Contains(got.Body.String(), "owner@example.org") {
		t.Fatalf("wrong-account page must name both identities: %q", got.Body.String())
	}
	var intruder int
	if err := db.QueryRow("SELECT count(*) FROM accounts WHERE tenant_id=$1 AND address='intruder@example.org'", tenant).Scan(&intruder); err != nil {
		t.Fatal(err)
	}
	if intruder != 0 {
		t.Fatal("wrong-account callback created an account row")
	}
	if err := db.QueryRow("SELECT refresh_token FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, accountID).Scan(&token); err != nil {
		t.Fatal(err)
	}
	if string(token) != "rt-gmail" {
		t.Fatalf("wrong-account callback overwrote the bound account token: %q", token)
	}

	// Matching identity, full photo grant: distinct truthful page and the
	// stored scopes are exactly the granted token scopes.
	provider.profileEmail = "owner@example.org"
	provider.tokenBody = `{"access_token":"at-photo","refresh_token":"rt-photo","token_type":"Bearer","expires_in":3600,"scope":"` + FullPhotoScope() + `"}`
	state = stateOf(call("POST", PhotoConsentPath, `{"account_id":"`+accountID.String()+`"}`))
	got = callback(state)
	if got.Code != 200 || !strings.Contains(got.Body.String(), "Sender photo access granted") {
		t.Fatalf("photo grant page: %d %s", got.Code, got.Body.String())
	}
	if err := db.QueryRow("SELECT refresh_token,granted_photo_scopes FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, accountID).Scan(&token, &scopes); err != nil {
		t.Fatal(err)
	}
	if string(token) != "rt-photo" || scopes != FullPhotoScope() {
		t.Fatalf("photo grant stored token=%q scopes=%q", token, scopes)
	}

	// Reconnect regression: a Gmail-only reconnect on the photo-granted
	// account must not downgrade the recorded photo authority.
	provider.tokenBody = `{"access_token":"at-re","refresh_token":"rt-reconnect","token_type":"Bearer","expires_in":3600,"scope":"` + ScopeGmailModify + `"}`
	state = stateOf(call("POST", ConnectPath, ""))
	if got := callback(state); got.Code != 200 {
		t.Fatalf("reconnect callback: %d %s", got.Code, got.Body.String())
	}
	if err := db.QueryRow("SELECT refresh_token,granted_photo_scopes FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, accountID).Scan(&token, &scopes); err != nil {
		t.Fatal(err)
	}
	if !HasPhotoScopes(scopes) {
		t.Fatalf("Gmail-only reconnect dropped the photo grant: scopes=%q", scopes)
	}
	if string(token) != "rt-photo" {
		t.Fatalf("scope kept for a token that lacks it (blind union): token=%q scopes=%q", token, scopes)
	}

	// Partial grant on a fresh account: the stored scopes are exactly the
	// partial token authority and the page says so truthfully.
	provider.profileEmail = "second@example.org"
	provider.tokenBody = `{"access_token":"at-second","refresh_token":"rt-second","token_type":"Bearer","expires_in":3600,"scope":"` + ScopeGmailModify + `"}`
	state = stateOf(call("POST", ConnectPath, ""))
	if got := callback(state); got.Code != 200 {
		t.Fatalf("second connect: %d %s", got.Code, got.Body.String())
	}
	var secondID uuid.UUID
	if err := db.QueryRow("SELECT id FROM accounts WHERE tenant_id=$1 AND address='second@example.org'", tenant).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	partial := ScopeGmailModify + " " + ScopeContactsReadonly
	provider.tokenBody = `{"access_token":"at-partial","refresh_token":"rt-partial","token_type":"Bearer","expires_in":3600,"scope":"` + partial + `"}`
	state = stateOf(call("POST", PhotoConsentPath, `{"account_id":"`+secondID.String()+`"}`))
	got = callback(state)
	if got.Code != 200 || !strings.Contains(got.Body.String(), "partial") || !strings.Contains(got.Body.String(), ScopeOtherContactsReadonly) {
		t.Fatalf("partial grant page: %d %s", got.Code, got.Body.String())
	}
	if err := db.QueryRow("SELECT granted_photo_scopes FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, secondID).Scan(&scopes); err != nil {
		t.Fatal(err)
	}
	if scopes != partial || HasPhotoScopes(scopes) {
		t.Fatalf("partial grant stored scopes=%q, want exactly %q", scopes, partial)
	}
}

// stateURL re-extracts the authorization URL from a start response body.
func stateURL(res *httptest.ResponseRecorder) string {
	var start struct {
		URL string `json:"authorization_url"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &start)
	return start.URL
}

// TestCallbackLegacySchemaConnect pins the pre-024 degrade path: on a
// deployment without the flow/account_id columns the plain connect callback
// still completes through the legacy state row, while photo consent fails
// closed with 503 instead of storing a state row the callback could never
// verify.
func TestCallbackLegacySchemaConnect(t *testing.T) {
	db := webTestDB(t)
	if _, err := db.Exec(`ALTER TABLE gmail_oauth_states DROP COLUMN IF EXISTS flow, DROP COLUMN IF EXISTS account_id`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE accounts DROP COLUMN IF EXISTS granted_photo_scopes`); err != nil {
		t.Fatal(err)
	}
	tenant := uuid.New()
	if _, err := db.Exec("INSERT INTO tenants(id) VALUES($1)", tenant); err != nil {
		t.Fatal(err)
	}
	provider := newFakeProvider(t)
	provider.tokenBody = `{"access_token":"at-fake","refresh_token":"rt-fake","token_type":"Bearer","expires_in":3600,"scope":"` + ScopeGmailModify + `"}`
	mux := http.NewServeMux()
	h := &WebHandler{DB: db, Config: WebConfig{Credentials: &Credentials{ClientID: "fake-client-id", ClientSecret: "fake-client-secret"}, CallbackURL: "https://example.test/v1/gmail/oauth/callback", AuthURL: provider.server.URL + "/auth", TokenURL: provider.server.URL + "/token", ProfileURL: provider.server.URL + "/profile", RevokeURL: provider.server.URL + "/revoke", StateSecret: []byte("12345678901234567890123456789012")}}
	h.Routes(mux)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(WithTenant(r.Context(), tenant.String()))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	start := call("POST", ConnectPath, "")
	if start.Code != 200 {
		t.Fatalf("connect: %d %s", start.Code, start.Body.String())
	}
	var parsed struct {
		URL string `json:"authorization_url"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	auth, err := url.Parse(parsed.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", OAuthCallbackPath+"?state="+url.QueryEscape(auth.Query().Get("state"))+"&code=fake-code", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("legacy connect callback: %d %s", w.Code, w.Body.String())
	}
	var accountID uuid.UUID
	if err := db.QueryRow("SELECT id FROM accounts WHERE tenant_id=$1 AND address='owner@example.org'", tenant).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	provider.tokenBody = `{"access_token":"at-photo","refresh_token":"rt-photo","token_type":"Bearer","expires_in":3600,"scope":"` + FullPhotoScope() + `"}`
	if got := call("POST", PhotoConsentPath, `{"account_id":"`+accountID.String()+`"}`); got.Code != 503 || !strings.Contains(got.Body.String(), "migration 024") {
		t.Fatalf("legacy photo consent: %d %s", got.Code, got.Body.String())
	}
}
