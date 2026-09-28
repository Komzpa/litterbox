package gmail

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeProvider is a fake Google authorization server: it records the token
// exchange request and serves the Gmail profile.
type fakeProvider struct {
	server *httptest.Server

	tokenCalls  atomic.Int32
	profileAuth atomic.Value // string

	// captured from the token exchange form
	gotVerifier    atomic.Value // string
	gotRedirectURI atomic.Value // string
	gotClientID    atomic.Value // string

	tokenStatus int
	tokenBody   string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	p := &fakeProvider{
		tokenStatus: http.StatusOK,
		tokenBody:   `{"access_token":"at-fake","refresh_token":"rt-fake","token_type":"Bearer","expires_in":3600,"scope":"https://www.googleapis.com/auth/gmail.modify"}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		p.tokenCalls.Add(1)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if got := r.PostForm.Get("grant_type"); got != "authorization_code" {
			http.Error(w, "bad grant_type "+got, http.StatusBadRequest)
			return
		}
		if got := r.PostForm.Get("code"); got != "fake-code" {
			http.Error(w, "bad code "+got, http.StatusBadRequest)
			return
		}
		p.gotVerifier.Store(r.PostForm.Get("code_verifier"))
		p.gotRedirectURI.Store(r.PostForm.Get("redirect_uri"))
		p.gotClientID.Store(r.PostForm.Get("client_id"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(p.tokenStatus)
		fmt.Fprint(w, p.tokenBody)
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		p.profileAuth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"emailAddress":"owner@example.org"}`)
	})
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// connectAgainst starts Connect against the fake provider and returns the
// captured authorization URL once it is ready.
func connectAgainst(t *testing.T, p *fakeProvider) (<-chan connectOutcome, <-chan string) {
	t.Helper()
	outCh := make(chan connectOutcome, 1)
	authURLCh := make(chan string, 1)
	go func() {
		res, err := Connect(context.Background(), &Credentials{ClientID: "fake-client-id", ClientSecret: "fake-client-secret"}, &ConnectOptions{
			AuthURL:    p.server.URL + "/auth",
			TokenURL:   p.server.URL + "/token",
			ProfileURL: p.server.URL + "/profile",
			OpenURL:    func(u string) error { authURLCh <- u; return nil },
		})
		outCh <- connectOutcome{res, err}
	}()
	return outCh, authURLCh
}

type connectOutcome struct {
	res *ConnectResult
	err error
}

// hitCallback simulates the browser redirect to the loopback listener.
func hitCallback(t *testing.T, authURL, stateOverride, code string) *http.Response {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	q := u.Query()
	redirect := q.Get("redirect_uri")
	if redirect == "" {
		t.Fatal("auth URL has no redirect_uri")
	}
	state := q.Get("state")
	if stateOverride != "" {
		state = stateOverride
	}
	resp, err := http.Get(redirect + "?state=" + url.QueryEscape(state) + "&code=" + url.QueryEscape(code))
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	return resp
}

func TestConnectLoopbackFlow(t *testing.T) {
	p := newFakeProvider(t)
	outCh, authURLCh := connectAgainst(t, p)

	var authURL string
	select {
	case authURL = <-authURLCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not produce an authorization URL")
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	q := u.Query()
	if got := q.Get("scope"); got != ScopeGmailModify {
		t.Errorf("scope = %q, want %q", got, ScopeGmailModify)
	}
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	challenge := q.Get("code_challenge")
	if challenge == "" {
		t.Fatal("auth URL has no code_challenge")
	}
	if q.Get("state") == "" {
		t.Fatal("auth URL has no state")
	}

	resp := hitCallback(t, authURL, "", "fake-code")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d, want 200", resp.StatusCode)
	}

	select {
	case out := <-outCh:
		if out.err != nil {
			t.Fatalf("Connect: %v", out.err)
		}
		if out.res.RefreshToken != "rt-fake" {
			t.Errorf("RefreshToken = %q, want rt-fake", out.res.RefreshToken)
		}
		if out.res.Email != "owner@example.org" {
			t.Errorf("Email = %q, want owner@example.org", out.res.Email)
		}
		if out.res.Expiry.IsZero() {
			t.Error("Expiry is zero, want expires_in-derived time")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not finish after callback")
	}

	// PKCE: the verifier sent to the token endpoint must hash to the
	// challenge published in the authorization URL.
	verifier, _ := p.gotVerifier.Load().(string)
	if verifier == "" {
		t.Fatal("token endpoint saw no code_verifier")
	}
	sum := sha256.Sum256([]byte(verifier))
	if got := base64.RawURLEncoding.EncodeToString(sum[:]); got != challenge {
		t.Errorf("sha256(code_verifier) = %q, want code_challenge %q", got, challenge)
	}
	if got, _ := p.gotClientID.Load().(string); got != "fake-client-id" {
		t.Errorf("client_id at token endpoint = %q, want fake-client-id", got)
	}
	redirect, _ := p.gotRedirectURI.Load().(string)
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
		t.Errorf("redirect_uri = %q, want loopback", redirect)
	}
	if got, _ := p.profileAuth.Load().(string); got != "Bearer at-fake" {
		t.Errorf("profile Authorization = %q, want Bearer at-fake", got)
	}
}

func TestConnectRejectsStateMismatch(t *testing.T) {
	p := newFakeProvider(t)
	outCh, authURLCh := connectAgainst(t, p)
	authURL := <-authURLCh

	resp := hitCallback(t, authURL, "forged-state", "fake-code")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("callback status = %d, want 400", resp.StatusCode)
	}

	select {
	case out := <-outCh:
		if out.err == nil || !strings.Contains(out.err.Error(), "state mismatch") {
			t.Fatalf("err = %v, want state mismatch error", out.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not abort on state mismatch")
	}
	if got := p.tokenCalls.Load(); got != 0 {
		t.Errorf("token endpoint called %d times despite state mismatch", got)
	}
}

func TestConnectTokenParseFailure(t *testing.T) {
	p := newFakeProvider(t)
	p.tokenBody = `<html>not json</html>`
	outCh, authURLCh := connectAgainst(t, p)
	authURL := <-authURLCh

	resp := hitCallback(t, authURL, "", "fake-code")
	resp.Body.Close()

	select {
	case out := <-outCh:
		if out.err == nil || !strings.Contains(out.err.Error(), "parse token response") {
			t.Fatalf("err = %v, want token parse error", out.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not fail on unparseable token response")
	}
}

func TestConnectMissingRefreshToken(t *testing.T) {
	p := newFakeProvider(t)
	p.tokenBody = `{"access_token":"at-fake","token_type":"Bearer","expires_in":3600}`
	outCh, authURLCh := connectAgainst(t, p)
	authURL := <-authURLCh

	resp := hitCallback(t, authURL, "", "fake-code")
	resp.Body.Close()

	select {
	case out := <-outCh:
		if out.err == nil || !strings.Contains(out.err.Error(), "refresh_token") {
			t.Fatalf("err = %v, want missing refresh_token error", out.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not fail on missing refresh_token")
	}
}

func TestConnectProviderError(t *testing.T) {
	p := newFakeProvider(t)
	outCh, authURLCh := connectAgainst(t, p)
	authURL := <-authURLCh

	u, _ := url.Parse(authURL)
	redirect := u.Query().Get("redirect_uri")
	state := u.Query().Get("state")
	resp, err := http.Get(redirect + "?state=" + url.QueryEscape(state) + "&error=access_denied")
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	resp.Body.Close()

	select {
	case out := <-outCh:
		if out.err == nil || !strings.Contains(out.err.Error(), "access_denied") {
			t.Fatalf("err = %v, want provider access_denied error", out.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not fail on provider error")
	}
	if got := p.tokenCalls.Load(); got != 0 {
		t.Errorf("token endpoint called %d times despite provider error", got)
	}
}

func TestLoadCredentialsFormats(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"installed": `{"installed":{"client_id":"id-1","client_secret":"secret-1","redirect_uris":["http://127.0.0.1"]}}`,
		"web":       `{"web":{"client_id":"id-2","client_secret":"secret-2"}}`,
		"flat":      `{"client_id":"id-3","client_secret":"secret-3"}`,
	}
	for name, body := range cases {
		path := dir + "/" + name + ".json"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		creds, err := LoadCredentials(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasPrefix(creds.ClientID, "id-") || !strings.HasPrefix(creds.ClientSecret, "secret-") {
			t.Errorf("%s: got %+v", name, creds)
		}
	}
	if _, err := LoadCredentials(dir + "/missing.json"); err == nil {
		t.Error("missing file: want error")
	}
	bad := dir + "/bad.json"
	if err := os.WriteFile(bad, []byte(`{"installed":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials(bad); err == nil {
		t.Error("empty credentials: want error")
	}
}
