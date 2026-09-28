package gmail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	AccountsPath      = "/v1/gmail/accounts"
	ConnectPath       = "/v1/gmail/connect"
	OAuthCallbackPath = "/v1/gmail/oauth/callback"
)

type WebConfig struct {
	Credentials                              *Credentials
	CallbackURL                              string
	AuthURL, TokenURL, ProfileURL, RevokeURL string
	StateSecret                              []byte
	HTTPClient                               *http.Client
}
type WebHandler struct {
	DB     *sql.DB
	Config WebConfig
}
type tenantContextKey struct{}

func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenant)
}
func tenantFrom(ctx context.Context) (uuid.UUID, error) {
	value, _ := ctx.Value(tenantContextKey{}).(string)
	return uuid.Parse(value)
}
func (h *WebHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+AccountsPath, h.listAccounts)
	mux.HandleFunc("POST "+ConnectPath, h.beginConnect)
	mux.HandleFunc("GET "+OAuthCallbackPath, h.callback)
	mux.HandleFunc("DELETE "+AccountsPath+"/{id}", h.deleteAccount)
}
func (h *WebHandler) dbTx(ctx context.Context, tenant uuid.UUID) (*sql.Tx, error) {
	tx, e := h.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, "SELECT set_config('litterbox.tenant_id',$1,true)", tenant.String()); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (h *WebHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	tenant, e := tenantFrom(r.Context())
	if e != nil {
		http.Error(w, "tenant required", http.StatusUnauthorized)
		return
	}
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(r.Context(), "SELECT id,address FROM accounts WHERE tenant_id=$1 AND status='active' ORDER BY address", tenant)
	if e != nil {
		http.Error(w, "could not list accounts", 500)
		return
	}
	defer rows.Close()
	out := make([]map[string]string, 0)
	for rows.Next() {
		var id uuid.UUID
		var address string
		if e = rows.Scan(&id, &address); e != nil {
			http.Error(w, "could not list accounts", 500)
			return
		}
		out = append(out, map[string]string{"id": id.String(), "address": address})
	}
	if e = rows.Err(); e != nil {
		http.Error(w, "could not list accounts", 500)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "could not list accounts", 500)
		return
	}
	writeJSON(w, 200, out)
}
func (h *WebHandler) beginConnect(w http.ResponseWriter, r *http.Request) {
	tenant, e := tenantFrom(r.Context())
	if e != nil {
		http.Error(w, "tenant required", 401)
		return
	}
	if h.Config.Credentials == nil || h.Config.Credentials.ClientID == "" || h.Config.CallbackURL == "" || len(h.Config.StateSecret) < 32 {
		http.Error(w, "Gmail OAuth is not configured", 503)
		return
	}
	verifier, e := pkceVerifier()
	if e != nil {
		http.Error(w, "could not start OAuth", 500)
		return
	}
	state, e := h.newState(tenant)
	if e != nil {
		http.Error(w, "could not start OAuth", 500)
		return
	}
	auth := h.Config.AuthURL
	if auth == "" {
		auth = DefaultAuthURL
	}
	authURL := buildAuthURL(auth, h.Config.Credentials.ClientID, h.Config.CallbackURL, ScopeGmailModify, state, pkceChallenge(verifier))
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(r.Context(), `INSERT INTO gmail_oauth_states(tenant_id,state_hash,code_verifier,redirect_uri,expires_at) VALUES($1,$2,$3,$4,now()+interval '10 minutes')`, tenant, stateHash(state), verifier, h.Config.CallbackURL)
	if e != nil {
		http.Error(w, "could not save OAuth state", 500)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "could not save OAuth state", 500)
		return
	}
	writeJSON(w, 200, map[string]string{"authorization_url": authURL})
}
func (h *WebHandler) newState(tenant uuid.UUID) (string, error) {
	nonce, e := randomState()
	if e != nil {
		return "", e
	}
	payload := tenant.String() + "." + nonce
	mac := hmac.New(sha256.New, h.Config.StateSecret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func stateTenant(state string, secret []byte) (uuid.UUID, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return uuid.Nil, errors.New("invalid state")
	}
	payload, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return uuid.Nil, e
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return uuid.Nil, e
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return uuid.Nil, errors.New("invalid state signature")
	}
	fields := strings.SplitN(string(payload), ".", 2)
	if len(fields) != 2 {
		return uuid.Nil, errors.New("invalid state payload")
	}
	return uuid.Parse(fields[0])
}
func stateHash(state string) []byte { sum := sha256.Sum256([]byte(state)); return sum[:] }
func (h *WebHandler) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	tenant, e := stateTenant(state, h.Config.StateSecret)
	if e != nil {
		http.Error(w, "invalid OAuth state", 400)
		return
	}
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var verifier, redirect string
	e = tx.QueryRowContext(r.Context(), `DELETE FROM gmail_oauth_states WHERE tenant_id=$1 AND state_hash=$2 AND expires_at>now() RETURNING code_verifier,redirect_uri`, tenant, stateHash(state)).Scan(&verifier, &redirect)
	if e != nil {
		http.Error(w, "invalid or expired OAuth state", 400)
		return
	}
	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		_ = tx.Commit()
		http.Error(w, "Google authorization was denied", 400)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", 400)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "could not consume OAuth state", 500)
		return
	}
	tokenURL := h.Config.TokenURL
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	client := h.Config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	tok, e := exchangeCode(r.Context(), client, tokenURL, h.Config.Credentials, redirect, code, verifier)
	if e != nil || tok.RefreshToken == "" {
		http.Error(w, "Gmail token exchange failed", 502)
		return
	}
	profileURL := h.Config.ProfileURL
	if profileURL == "" {
		profileURL = DefaultProfileURL
	}
	email, e := fetchEmail(r.Context(), client, profileURL, tok.AccessToken)
	if e != nil {
		http.Error(w, "could not read Gmail account", 502)
		return
	}
	accountID := uuid.New()
	store, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer store.Rollback()
	_, e = store.ExecContext(r.Context(), `INSERT INTO accounts(tenant_id,id,address,refresh_token,status) VALUES($1,$2,$3,$4,'active') ON CONFLICT(tenant_id,address) DO UPDATE SET refresh_token=EXCLUDED.refresh_token,status='active'`, tenant, accountID, email, []byte(tok.RefreshToken))
	if e != nil {
		http.Error(w, "could not save Gmail account", 500)
		return
	}
	if e = store.Commit(); e != nil {
		http.Error(w, "could not save Gmail account", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, "<!doctype html><title>Gmail connected</title><p>Gmail connected. Return to Litterbox.</p>")
}
func (h *WebHandler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	tenant, e := tenantFrom(r.Context())
	if e != nil {
		http.Error(w, "tenant required", 401)
		return
	}
	id, e := uuid.Parse(r.PathValue("id"))
	if e != nil {
		http.Error(w, "invalid account id", 400)
		return
	}
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var token []byte
	if e = tx.QueryRowContext(r.Context(), "SELECT refresh_token FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, id).Scan(&token); e != nil {
		http.Error(w, "account not found", 404)
		return
	}
	if e = h.revoke(r.Context(), string(token)); e != nil {
		http.Error(w, "could not revoke Gmail token", 502)
		return
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM accounts WHERE tenant_id=$1 AND id=$2", tenant, id); e != nil {
		http.Error(w, "could not disconnect Gmail account", 500)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "could not disconnect Gmail account", 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *WebHandler) revoke(ctx context.Context, token string) error {
	endpoint := h.Config.RevokeURL
	if endpoint == "" {
		endpoint = "https://oauth2.googleapis.com/revoke"
	}
	form := url.Values{"token": {token}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := h.Config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Google revoke returned %s", resp.Status)
	}
	return nil
}
