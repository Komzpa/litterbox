// Package gmail owns Gmail connectivity for Litterbox: the administrative
// OAuth connection flow and, later, per-account sync workers.
package gmail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Google OAuth and Gmail endpoints. Tests override them via ConnectOptions.
const (
	DefaultAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	DefaultTokenURL   = "https://oauth2.googleapis.com/token"
	DefaultProfileURL = "https://gmail.googleapis.com/gmail/v1/users/me/profile"

	// ScopeGmailModify is the only scope Litterbox requests (ARCHITECTURE).
	ScopeGmailModify = "https://www.googleapis.com/auth/gmail.modify"
)

const (
	callbackPath   = "/oauth2/callback"
	maxErrorBody   = 512
	browserTimeout = 5 * time.Minute
)

// Credentials is the Google OAuth Desktop-client pair. It is loaded from a
// JSON file kept outside the repository (R17); never commit it.
type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// LoadCredentials reads a Google client-secret JSON file. It accepts the
// downloaded "installed" (Desktop) or "web" wrapper formats as well as a
// flat {"client_id","client_secret"} object.
func LoadCredentials(path string) (*Credentials, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gmail: read credentials: %w", err)
	}
	var wrapped struct {
		Installed *Credentials `json:"installed"`
		Web       *Credentials `json:"web"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("gmail: parse credentials %s: %w", path, err)
	}
	creds := wrapped.Installed
	if creds == nil {
		creds = wrapped.Web
	}
	if creds == nil {
		var flat Credentials
		if err := json.Unmarshal(raw, &flat); err == nil && flat.ClientID != "" {
			creds = &flat
		}
	}
	if creds == nil || creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, fmt.Errorf("gmail: credentials %s: missing client_id or client_secret", path)
	}
	return creds, nil
}

// ConnectOptions tunes Connect. The zero value runs the production flow
// against Google and tries to open the user's browser.
type ConnectOptions struct {
	// AuthURL, TokenURL and ProfileURL default to Google's endpoints;
	// tests point them at a fake server.
	AuthURL    string
	TokenURL   string
	ProfileURL string
	// Scope defaults to ScopeGmailModify.
	Scope string
	// HTTPClient is used for the token exchange and profile request;
	// nil means http.DefaultClient.
	HTTPClient *http.Client
	// OpenURL is invoked with the authorization URL so the user can
	// approve the consent screen. nil means: try xdg-open and always
	// print the URL to Out. Tests capture the URL here.
	OpenURL func(authURL string) error
	// Out receives the authorization URL for manual copy/paste; nil
	// discards it.
	Out io.Writer
	// ListenAddr is the loopback callback address; empty picks
	// 127.0.0.1 with a random port, as required for Desktop clients.
	ListenAddr string
}

// ConnectResult is what the caller persists for the account.
type ConnectResult struct {
	RefreshToken string
	Email        string
	AccessToken  string
	Expiry       time.Time
}

// Connect runs the administrative Desktop-client loopback flow
// (ARCHITECTURE): PKCE (S256) plus a validated state, scope gmail.modify,
// offline access. It prints/opens the consent URL, waits for the Google
// redirect on 127.0.0.1, exchanges the code, and resolves the account
// email via the Gmail profile. The returned refresh token and email are
// for the caller to store; nothing is persisted here.
func Connect(ctx context.Context, creds *Credentials, opts *ConnectOptions) (*ConnectResult, error) {
	if creds == nil || creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, errors.New("gmail: connect requires client credentials")
	}
	o := ConnectOptions{}
	if opts != nil {
		o = *opts
	}
	if o.AuthURL == "" {
		o.AuthURL = DefaultAuthURL
	}
	if o.TokenURL == "" {
		o.TokenURL = DefaultTokenURL
	}
	if o.ProfileURL == "" {
		o.ProfileURL = DefaultProfileURL
	}
	if o.Scope == "" {
		o.Scope = ScopeGmailModify
	}
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.ListenAddr == "" {
		o.ListenAddr = "127.0.0.1:0"
	}

	verifier, err := pkceVerifier()
	if err != nil {
		return nil, err
	}
	challenge := pkceChallenge(verifier)
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", o.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("gmail: listen for OAuth callback: %w", err)
	}
	defer listener.Close()
	redirectURI := "http://" + listener.Addr().String() + callbackPath

	authURL := buildAuthURL(o.AuthURL, creds.ClientID, redirectURI, o.Scope, state, challenge)
	if o.Out != nil {
		fmt.Fprintf(o.Out, "Open this URL to connect the Gmail account:\n%s\n", authURL)
	}
	if o.OpenURL != nil {
		if err := o.OpenURL(authURL); err != nil && o.Out != nil {
			fmt.Fprintf(o.Out, "gmail: could not open a browser (%v); use the URL above.\n", err)
		}
	} else {
		_ = exec.Command("xdg-open", authURL).Start() // best effort
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: callbackHandler(state, codeCh, errCh)}
	serveDone := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(serveDone)
	}()

	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = server.Close()
		<-serveDone
		return nil, err
	case <-ctx.Done():
		_ = server.Close()
		<-serveDone
		return nil, fmt.Errorf("gmail: waiting for OAuth callback: %w", ctx.Err())
	}
	_ = server.Close()
	<-serveDone

	tok, err := exchangeCode(ctx, o.HTTPClient, o.TokenURL, creds, redirectURI, code, verifier)
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("gmail: token response has no refresh_token; revoke the app in the Google account permissions page and connect again")
	}
	email, err := fetchEmail(ctx, o.HTTPClient, o.ProfileURL, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	return &ConnectResult{
		RefreshToken: tok.RefreshToken,
		Email:        email,
		AccessToken:  tok.AccessToken,
		Expiry:       tok.Expiry,
	}, nil
}

func buildAuthURL(authURL, clientID, redirectURI, scope, state, challenge string) string {
	q := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {scope},
		"access_type":           {"offline"},
		"prompt":                {"consent"}, // refresh token on every reconnect
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	sep := "?"
	if strings.Contains(authURL, "?") {
		sep = "&"
	}
	return authURL + sep + q.Encode()
}

// callbackHandler serves the single loopback redirect. A state mismatch or
// provider error aborts the flow; the matching code is delivered on codeCh.
func callbackHandler(wantState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if providerErr := q.Get("error"); providerErr != "" {
			http.Error(w, "authorization failed; you can close this tab", http.StatusBadRequest)
			errCh <- fmt.Errorf("gmail: authorization error from provider: %s", providerErr)
			return
		}
		if got := q.Get("state"); got != wantState {
			http.Error(w, "state mismatch; you can close this tab", http.StatusBadRequest)
			errCh <- errors.New("gmail: OAuth state mismatch; possible CSRF, aborting")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Gmail account connected. You can close this tab and return to litterbox.\n")
		codeCh <- code
	})
	return mux
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`

	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`

	Expiry time.Time `json:"-"`
}

// exchangeCode swaps the authorization code for tokens, proving possession
// of the PKCE verifier.
func exchangeCode(ctx context.Context, client *http.Client, tokenURL string, creds *Credentials, redirectURI, code, verifier string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gmail: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gmail: token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("gmail: read token response: %w", err)
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("gmail: parse token response (status %s): %w", resp.Status, err)
	}
	if tok.Error != "" {
		return nil, fmt.Errorf("gmail: token exchange rejected: %s: %s", tok.Error, tok.ErrorDescription)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gmail: token exchange status %s: %s", resp.Status, truncate(body))
	}
	if tok.AccessToken == "" {
		return nil, errors.New("gmail: token response has no access_token")
	}
	if tok.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return &tok, nil
}

// fetchEmail resolves the connected account's address from the Gmail
// profile so the caller can label the stored refresh token.
func fetchEmail(ctx context.Context, client *http.Client, profileURL, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return "", fmt.Errorf("gmail: build profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gmail: fetch profile: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("gmail: read profile response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gmail: profile status %s: %s", resp.Status, truncate(body))
	}
	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return "", fmt.Errorf("gmail: parse profile response: %w", err)
	}
	if profile.EmailAddress == "" {
		return "", errors.New("gmail: profile response has no emailAddress")
	}
	return profile.EmailAddress, nil
}

// pkceVerifier returns a 43-character RFC 7636 code verifier.
func pkceVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gmail: generate PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gmail: generate state: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func truncate(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxErrorBody {
		s = s[:maxErrorBody] + "..."
	}
	return s
}
