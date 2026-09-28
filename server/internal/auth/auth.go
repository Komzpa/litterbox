package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type Device struct {
	TenantID string
	DeviceID string
}
type Store interface {
	RedeemInvite(context.Context, string, string, string, string, string) (string, error)
	DeviceByToken(context.Context, string) (Device, error)
	CreateInvite(context.Context, string, string) error
	RevokeDevice(context.Context, string, string) error
}
type Handler struct {
	store     Store
	limiter   *IPLimiter
	devTenant string
}

func NewHandler(store Store, devTenant string) *Handler {
	return &Handler{store: store, limiter: NewIPLimiter(60, time.Minute), devTenant: devTenant}
}

type contextKey struct{}

func DeviceFromContext(ctx context.Context) (Device, bool) {
	d, ok := ctx.Value(contextKey{}).(Device)
	return d, ok
}

// Middleware requires a valid bearer token on every /v1 route, except invite redemption.
// devTenant deliberately bypasses tokens only when the caller explicitly enables -dev.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") && r.URL.Path != "/v1" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/v1/devices/enroll" && r.Method == http.MethodPost {
			if !h.limiter.Allow(clientIP(r)) {
				writeError(w, 429, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if h.devTenant != "" {
			d := Device{TenantID: h.devTenant, DeviceID: "development"}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, d)))
			return
		}
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) || strings.TrimSpace(strings.TrimPrefix(header, prefix)) == "" {
			writeError(w, 401, "unauthorized")
			return
		}
		d, err := h.store.DeviceByToken(r.Context(), tokenHash(strings.TrimSpace(strings.TrimPrefix(header, prefix))))
		if err != nil {
			writeError(w, 401, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, d)))
	})
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/devices/enroll", h.enroll)
	mux.HandleFunc("POST /v1/invites", h.createInvite)
	mux.HandleFunc("DELETE /v1/devices/{deviceID}", h.revokeDevice)
}
func (h *Handler) enroll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"invite_code"`
		DeviceName string `json:"device_name"`
		Platform   string `json:"platform"`
	}
	if decodeJSON(w, r, &req) != nil || strings.TrimSpace(req.InviteCode) == "" || strings.TrimSpace(req.DeviceName) == "" || strings.TrimSpace(req.Platform) == "" {
		writeError(w, 400, "invalid invite")
		return
	}
	device, err := randomID()
	if err != nil {
		writeError(w, 500, "enrollment failed")
		return
	}
	token, err := randomSecret(32)
	if err != nil {
		writeError(w, 500, "enrollment failed")
		return
	}
	tenant, err := h.store.RedeemInvite(r.Context(), req.InviteCode, device, req.DeviceName, req.Platform, tokenHash(token))
	if err != nil {
		writeError(w, 410, "invite is used, expired, or invalid")
		return
	}
	writeJSON(w, 201, map[string]string{"tenant_id": tenant, "device_id": device, "token": token})
}
func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) {
	d, ok := DeviceFromContext(r.Context())
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	invite, err := randomSecret(32)
	if err != nil {
		writeError(w, 500, "invite creation failed")
		return
	}
	if err = h.store.CreateInvite(r.Context(), d.TenantID, tokenHash(invite)); err != nil {
		writeError(w, 500, "invite creation failed")
		return
	}
	writeJSON(w, 201, map[string]string{"invite": invite})
}
func (h *Handler) revokeDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := DeviceFromContext(r.Context())
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	if err := h.store.RevokeDevice(r.Context(), d.TenantID, r.PathValue("deviceID")); err != nil {
		writeError(w, 404, "device not found")
		return
	}
	w.WriteHeader(204)
}

type DB interface {
	Begin(context.Context) (pgx.Tx, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type postgresStore struct{ db DB }

func NewPostgresStore(db DB) Store { return &postgresStore{db: db} }
func (s *postgresStore) RedeemInvite(ctx context.Context, invite, device, name, platform, token string) (string, error) {
	var tenant string
	err := s.db.QueryRow(ctx, "SELECT litterbox_redeem_invite(decode($1,'hex'),$2::uuid,$3,$4,decode($5,'hex'))::text", tokenHash(invite), device, name, platform, token).Scan(&tenant)
	return tenant, err
}
func (s *postgresStore) DeviceByToken(ctx context.Context, hash string) (Device, error) {
	var d Device
	err := s.db.QueryRow(ctx, "SELECT tenant_id::text,device_id::text FROM litterbox_device_by_token(decode($1,'hex'))", hash).Scan(&d.TenantID, &d.DeviceID)
	return d, err
}
func (s *postgresStore) CreateInvite(ctx context.Context, tenant, hash string) error {
	id, err := randomID()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('litterbox.tenant_id',$1,true)", tenant); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO invites(tenant_id,id,code_hash) VALUES($1::uuid,$2::uuid,decode($3,'hex'))", tenant, id, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *postgresStore) RevokeDevice(ctx context.Context, tenant, device string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('litterbox.tenant_id',$1,true)", tenant); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE devices SET revoked_at=now() WHERE tenant_id=$1::uuid AND id=$2::uuid AND revoked_at IS NULL", tenant, device)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("device not found")
	}
	return tx.Commit(ctx)
}
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:])
}
func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type IPLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func NewIPLimiter(limit int, window time.Duration) *IPLimiter {
	return &IPLimiter{limit: limit, window: window, hits: make(map[string][]time.Time)}
}
func (l *IPLimiter) Allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	all := l.hits[ip]
	active := all[:0]
	for _, t := range all {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}
	if len(active) >= l.limit {
		l.hits[ip] = active
		return false
	}
	l.hits[ip] = append(active, now)
	return true
}
