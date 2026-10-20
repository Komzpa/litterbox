package senderphoto

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/Komzpa/litterbox/server/internal/gmail"
)

// Handler serves GET /v1/cards/{id}/sender-photo: the stored Google photo
// for the card's sender, tenant- and account-scoped. Lookup is by card id
// so raw email addresses never appear in URLs; provider URLs and tokens
// never leave the server. Missing photos are a plain 404 (initials
// fallback); accounts without the incremental photo grant get an explicit
// 403 authorization_required (never a silent initials claim).
type Handler struct {
	DB *sql.DB
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, ok := cards.TenantFrom(r.Context())
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cardID := r.PathValue("id")
	if cardID == "" {
		http.NotFound(w, r)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var accountID, granted sql.NullString
	var cardAccount sql.NullString
	var latestSender sql.NullString
	// Latest message sender wins (thread view shows the newest arrival);
	// fall back to the card's normalized sender_address for legacy rows.
	err = tx.QueryRowContext(r.Context(), `SELECT c.account_id::text,
COALESCE((SELECT m.sender_address FROM messages m WHERE m.tenant_id=c.tenant_id AND m.card_id=c.id AND m.sender_address<>'' ORDER BY m.received_at DESC LIMIT 1), COALESCE(c.sender_address,'')),
COALESCE((SELECT a.granted_photo_scopes FROM accounts a WHERE a.tenant_id=c.tenant_id AND a.id=c.account_id),'')
FROM cards c WHERE c.tenant_id=$1 AND c.id::text=$2`, tenant, cardID).Scan(&cardAccount, &latestSender, &granted)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	_ = accountID
	key := NormalizeAddress(latestSender.String)
	if key == "" {
		writeUnavailable(w)
		return
	}
	if !gmail.HasPhotoScopes(granted.String) {
		writeAuthorizationRequired(w)
		return
	}
	var photo []byte
	var contentType string
	err = tx.QueryRowContext(r.Context(), `SELECT photo,content_type FROM sender_photos WHERE tenant_id=$1 AND account_id=$2 AND sender_address=$3`,
		tenant, cardAccount.String, key).Scan(&photo, &contentType)
	if errors.Is(err, sql.ErrNoRows) || len(photo) == 0 {
		_ = tx.Commit()
		writeUnavailable(w)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", itoa(len(photo)))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(photo)
}

func writeUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
}

func writeAuthorizationRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "authorization_required",
		"required_scopes": gmail.PhotoScopes(),
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// StatusHandler serves GET /v1/gmail/sender-photos/status: per-account
// photo readiness without touching Google credentials or performing any
// grant. photo_ready means at least one decoded photo is stored;
// authorization_required names the incremental scopes still missing.
type StatusHandler struct {
	DB *sql.DB
}

func (h StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, ok := cards.TenantFrom(r.Context())
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT a.id::text,a.address,COALESCE(a.granted_photo_scopes,''),
EXISTS(SELECT 1 FROM sender_photos sp WHERE sp.tenant_id=a.tenant_id AND sp.account_id=a.id)
FROM accounts a WHERE a.tenant_id=$1 AND a.status IN ('active','reauth_required') ORDER BY a.address`, tenant)
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	type entry struct {
		AccountID             string   `json:"account_id"`
		Address               string   `json:"address"`
		PhotoReady            bool     `json:"photo_ready"`
		AuthorizationRequired bool     `json:"authorization_required"`
		RequiredScopes        []string `json:"required_scopes,omitempty"`
	}
	out := make([]entry, 0)
	for rows.Next() {
		var id string
		var address, granted string
		var ready bool
		if err = rows.Scan(&id, &address, &granted, &ready); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		e := entry{AccountID: id, PhotoReady: ready}
		// Addresses identify the account for the operator; the photo
		// route itself never carries them.
		e.Address = address
		if !gmail.HasPhotoScopes(granted) {
			e.AuthorizationRequired = true
			e.RequiredScopes = gmail.PhotoScopes()
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"accounts": out})
}

var _ = strings.TrimSpace
