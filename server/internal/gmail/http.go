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

	"github.com/Komzpa/litterbox/server/internal/httpapi"
	"github.com/google/uuid"
)

const (
	AccountsPath      = "/v1/gmail/accounts"
	ConnectPath       = "/v1/gmail/connect"
	OAuthCallbackPath = "/v1/gmail/oauth/callback"
	// PhotoConsentPath starts the incremental photo grant. It returns an
	// authorization_url for Gmail.modify plus both People photo scopes and
	// performs no grant itself; the account approves in the browser.
	PhotoConsentPath = "/v1/gmail/photo-consent"
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
	mux.HandleFunc("POST "+PhotoConsentPath, h.beginPhotoConsent)
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
	rows, e := tx.QueryContext(r.Context(), "SELECT id,address,status FROM accounts WHERE tenant_id=$1 AND status IN ('active','reauth_required') ORDER BY address", tenant)
	if e != nil {
		http.Error(w, "could not list accounts", 500)
		return
	}
	defer rows.Close()
	out := make([]map[string]string, 0)
	for rows.Next() {
		var id uuid.UUID
		var address, status string
		if e = rows.Scan(&id, &address, &status); e != nil {
			http.Error(w, "could not list accounts", 500)
			return
		}
		out = append(out, map[string]string{"id": id.String(), "address": address, "status": status})
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
	authURL := buildAuthURL(auth, h.Config.Credentials.ClientID, h.Config.CallbackURL, FullPhotoScope(), state, pkceChallenge(verifier))
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
// beginPhotoConsent returns an incremental authorization_url for Gmail.modify
// plus both People photo scopes, bound to the account the caller names in the
// POST body ({account_id}). The binding rides the existing gmail_oauth_states
// row so the shared callback can verify that the Google identity it receives is
// the intended account before storing anything. It stores PKCE state like
// beginConnect and performs no grant: the account approves in the browser, and
// the callback persists the actually granted scopes. Without this step the
// photo pipeline stays in authorization_required (Gmail.modify alone carries
// no sender-photo field).
func (h *WebHandler) beginPhotoConsent(w http.ResponseWriter, r *http.Request) {
	tenant, e := tenantFrom(r.Context())
	if e != nil {
		http.Error(w, "tenant required", 401)
		return
	}
	var in struct {
		AccountID string `json:"account_id"`
	}
	if e = json.NewDecoder(r.Body).Decode(&in); e != nil || in.AccountID == "" {
		http.Error(w, "account_id required", 400)
		return
	}
	accountID, e := uuid.Parse(in.AccountID)
	if e != nil {
		http.Error(w, "invalid account id", 400)
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
	authURL := buildAuthURL(auth, h.Config.Credentials.ClientID, h.Config.CallbackURL, FullPhotoScope(), state, pkceChallenge(verifier))
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		http.Error(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var address string
	if e = tx.QueryRowContext(r.Context(), `SELECT address FROM accounts WHERE tenant_id=$1 AND id=$2`, tenant, accountID).Scan(&address); e != nil {
		http.Error(w, "unknown account", 400)
		return
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO gmail_oauth_states(tenant_id,state_hash,code_verifier,redirect_uri,expires_at,flow,account_id) VALUES($1,$2,$3,$4,now()+interval '10 minutes','photo',$5)`, tenant, stateHash(state), verifier, h.Config.CallbackURL, accountID)
	if e != nil {
		// Old deployments without migration 024 cannot bind the consent
		// to an account: fail closed and say so instead of storing an
		// unbound state row that the callback could never verify.
		if isUndefinedColumn(e) {
			http.Error(w, "sender photo consent needs a server database update (migration 024). Plain Gmail connect still works.", 503)
			return
		}
		http.Error(w, "could not save OAuth state", 500)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "could not save OAuth state", 500)
		return
	}
	writeJSON(w, 200, map[string]string{"authorization_url": authURL})
}
// isUndefinedColumn reports PostgreSQL undefined-column (42703), used to
// degrade truthfully on deployments without the latest migration.
func isUndefinedColumn(e error) bool {
	return e != nil && strings.Contains(e.Error(), "42703")
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
		httpapi.MessagePage(w, 400, "invalid OAuth state. Start connecting Gmail again from Litterbox.")
		return
	}
	tx, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		httpapi.MessagePage(w, 500, "Database unavailable. Try connecting Gmail again from Litterbox.")
		return
	}
	defer tx.Rollback()
	var verifier, redirect, flow string
	var boundID sql.NullString
	// A failed statement aborts the whole transaction in PostgreSQL, so the
	// legacy retry below needs a savepoint: without one the retry would always
	// die with "current transaction is aborted".
	_, _ = tx.ExecContext(r.Context(), "SAVEPOINT oauth_consume")
	e = tx.QueryRowContext(r.Context(), `DELETE FROM gmail_oauth_states WHERE tenant_id=$1 AND state_hash=$2 AND expires_at>now() RETURNING code_verifier,redirect_uri,flow,account_id::text`, tenant, stateHash(state)).Scan(&verifier, &redirect, &flow, &boundID)
	if e != nil {
		// Old deployments without migration 024 degrade to the plain
		// connect flow: consume the legacy state row. Photo binding
		// needs 024 applied; it never silently degrades to unbound.
		_, _ = tx.ExecContext(r.Context(), "ROLLBACK TO SAVEPOINT oauth_consume")
		e = tx.QueryRowContext(r.Context(), `DELETE FROM gmail_oauth_states WHERE tenant_id=$1 AND state_hash=$2 AND expires_at>now() RETURNING code_verifier,redirect_uri`, tenant, stateHash(state)).Scan(&verifier, &redirect)
		if e == nil {
			flow = "connect"
		}
	} else {
		_, _ = tx.ExecContext(r.Context(), "RELEASE SAVEPOINT oauth_consume")
	}
	if e != nil {
		httpapi.MessagePage(w, 400, "Invalid or expired OAuth state. Start connecting Gmail again from Litterbox.")
		return
	}
	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		_ = tx.Commit()
		httpapi.MessagePage(w, 400, "Google authorization was denied. Start connecting Gmail again from Litterbox to grant access.")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		httpapi.MessagePage(w, 400, "Missing authorization code. Start connecting Gmail again from Litterbox.")
		return
	}
	if e = tx.Commit(); e != nil {
		httpapi.MessagePage(w, 500, "Could not consume OAuth state. Start connecting Gmail again from Litterbox.")
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
		httpapi.MessagePage(w, 502, "Gmail token exchange failed. Start connecting Gmail again from Litterbox.")
		return
	}
	profileURL := h.Config.ProfileURL
	if profileURL == "" {
		profileURL = DefaultProfileURL
	}
	email, e := fetchEmail(r.Context(), client, profileURL, tok.AccessToken)
	if e != nil {
		httpapi.MessagePage(w, 502, "Could not read Gmail account. Try connecting Gmail again from Litterbox.")
		return
	}
	// Photo-consent states are bound to one intended account: the Google
	// identity that approved must match that account's address before any
	// token is stored, so a grant approved by another identity can never land
	// on (or create) the wrong account row.
	if flow == "photo" {
		bound, parseErr := uuid.Parse(boundID.String)
		if parseErr != nil {
			httpapi.MessagePage(w, 400, "Photo authorization state is not bound to an account. Start again from the account's sender-photos row in Litterbox.")
			return
		}
		store, e := h.dbTx(r.Context(), tenant)
		if e != nil {
			httpapi.MessagePage(w, 500, "Database unavailable. Try connecting Gmail again from Litterbox.")
			return
		}
		defer store.Rollback()
		var want string
		if e = store.QueryRowContext(r.Context(), `SELECT address FROM accounts WHERE tenant_id=$1 AND id=$2`, tenant, bound).Scan(&want); e != nil {
			httpapi.MessagePage(w, 400, "Photo authorization state is not bound to an account. Start again from the account's sender-photos row in Litterbox.")
			return
		}
		if !strings.EqualFold(email, want) {
			httpapi.MessagePage(w, 400, "Google account mismatch: this photo authorization was started for "+want+" but "+email+" approved it. Nothing was saved. Sign in as the intended account and start again from its sender-photos row in Litterbox.")
			return
		}
	}
	accountID := uuid.New()
	store, e := h.dbTx(r.Context(), tenant)
	if e != nil {
		httpapi.MessagePage(w, 500, "Database unavailable. Try connecting Gmail again from Litterbox.")
		return
	}
	defer store.Rollback()
	// R67: persist the actually granted scopes (not the requested ones) so
	// the photo pipeline can gate on HasPhotoScopes without a Google call.
	// The stored scopes track one token's real authority: a grant that lacks
	// the photo scopes never downgrades a stored photo grant away and never
	// gets unioned with it (that would claim scopes the stored token does not
	// carry). Old deployments without granted_photo_scopes degrade to
	// Gmail-only: try the new column, fall back to the legacy insert.
	// A failed statement aborts the transaction, so the legacy retries below
	// share one savepoint established before the first new-column probe: the
	// scope-preservation SELECT itself fails closed (42703) on old schemas.
	_, _ = store.ExecContext(r.Context(), "SAVEPOINT oauth_store")
	newToken, newScopes := []byte(tok.RefreshToken), tok.Scope
	if !HasPhotoScopes(newScopes) {
		var oldToken []byte
		var oldScopes string
		if err := store.QueryRowContext(r.Context(), `SELECT refresh_token,granted_photo_scopes FROM accounts WHERE tenant_id=$1 AND address=$2`, tenant, email).Scan(&oldToken, &oldScopes); err == nil && HasPhotoScopes(oldScopes) {
			newToken, newScopes = oldToken, oldScopes
		} else if err != nil {
			_, _ = store.ExecContext(r.Context(), "ROLLBACK TO SAVEPOINT oauth_store")
			_, _ = store.ExecContext(r.Context(), "SAVEPOINT oauth_store")
		}
	}
	_, e = store.ExecContext(r.Context(), `INSERT INTO accounts(tenant_id,id,address,refresh_token,status,granted_photo_scopes) VALUES($1,$2,$3,$4,'active',$5) ON CONFLICT(tenant_id,address) DO UPDATE SET refresh_token=EXCLUDED.refresh_token,status='active',granted_photo_scopes=EXCLUDED.granted_photo_scopes`, tenant, accountID, email, newToken, newScopes)
	if e != nil {
		_, _ = store.ExecContext(r.Context(), "ROLLBACK TO SAVEPOINT oauth_store")
		_, e = store.ExecContext(r.Context(), `INSERT INTO accounts(tenant_id,id,address,refresh_token,status) VALUES($1,$2,$3,$4,'active') ON CONFLICT(tenant_id,address) DO UPDATE SET refresh_token=EXCLUDED.refresh_token,status='active'`, tenant, accountID, email, newToken)
	} else {
		_, _ = store.ExecContext(r.Context(), "RELEASE SAVEPOINT oauth_store")
	}
	if e != nil {
		httpapi.MessagePage(w, 500, "Could not save Gmail account. Try connecting Gmail again from Litterbox.")
		return
	}
	if e = store.Commit(); e != nil {
		httpapi.MessagePage(w, 500, "Could not save Gmail account. Try connecting Gmail again from Litterbox.")
		return
	}
	// Distinct truthful pages per grant class: what the token actually holds
	// decides the message, so a Gmail-only approval never reads as photo done.
	switch {
	case HasPhotoScopes(tok.Scope):
		httpapi.MessagePage(w, 200, "Sender photo access granted for "+email+". You can close this tab and return to Litterbox — the sender-photos row will update.")
	case grantedAnyPhotoScope(tok.Scope):
		httpapi.MessagePage(w, 200, "Partial sender photo access for "+email+": this approval was a partial grant ("+tok.Scope+"), still missing "+strings.Join(missingPhotoScopes(tok.Scope), " ")+". Sender photos need authorization again to complete the grant. You can close this tab and return to Litterbox.")
	default:
		msg := "Gmail connected: " + email + ". You can close this tab and return to Litterbox."
		if flow == "photo" {
			msg = "Gmail connected: " + email + ", but sender photo access was not granted in this approval. Sender photos need authorization from the account's sender-photos row in Litterbox."
		}
		httpapi.MessagePage(w, 200, msg)
	}
}
// grantedAnyPhotoScope reports a partial photo grant: at least one but not all
// of the photo scopes arrived on the token.
func grantedAnyPhotoScope(granted string) bool {
	for _, field := range splitScopes(granted) {
		if field == ScopeContactsReadonly || field == ScopeOtherContactsReadonly {
			return !HasPhotoScopes(granted)
		}
	}
	return false
}
// missingPhotoScopes lists the photo scopes the granted token does not carry.
func missingPhotoScopes(granted string) []string {
	missing := make([]string, 0, 2)
	for _, want := range PhotoScopes() {
		found := false
		for _, field := range splitScopes(granted) {
			if field == want {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, want)
		}
	}
	return missing
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
