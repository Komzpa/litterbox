package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"github.com/Komzpa/litterbox/server/internal/cards"
)

type fakeStore struct {
	used       bool
	tokenHash  string
	device     Device
	revoked    bool
	inviteHash string
}

func (f *fakeStore) RedeemInvite(_ context.Context, invite, id, name, platform, hashed string) (string, error) {
	if f.used || tokenHash(invite) != f.inviteHash {
		return "", errors.New("invalid invite")
	}
	f.used = true
	f.tokenHash = hashed
	return "tenant", nil
}
func (f *fakeStore) DeviceByToken(_ context.Context, hash string) (Device, error) {
	if hash != f.tokenHash || hash == "" {
		return Device{}, errors.New("missing")
	}
	return f.device, nil
}
func (f *fakeStore) CreateInvite(_ context.Context, _ string, hash string) error {
	f.inviteHash = hash
	return nil
}
func (f *fakeStore) RevokeDevice(context.Context, string, string) error { f.revoked = true; return nil }

func TestEveryProtectedRouteRejectsMissingAuthorization(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store, "")
	mux := http.NewServeMux()
	h.Register(mux)
	mux.HandleFunc("GET /v1/cards", func(w http.ResponseWriter, r *http.Request) { tenant, ok := cards.TenantFrom(r.Context()); if !ok || tenant != "tenant" { w.WriteHeader(500); return }; w.WriteHeader(200) })
	protected := h.Middleware(mux)
	for _, tc := range []struct{ method, path string }{{"POST", "/v1/invites"}, {"DELETE", "/v1/devices/a"}, {"GET", "/v1/cards"}, {"GET", "/v1/unknown"}} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			res := httptest.NewRecorder()
			protected.ServeHTTP(res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want 401", res.Code)
			}
		})
	}
}

func TestIngestPassesSourceTokenToHandlerWithoutDeviceIdentity(t *testing.T) {
	h := NewHandler(&fakeStore{}, "")
	handler := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := DeviceFromContext(r.Context()); ok {
			t.Fatal("source-authenticated ingest acquired device identity")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer source-token")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("ingest status=%d want 204", res.Code)
	}
}

func TestIngestMethodAndOtherDeviceRoutesRemainProtected(t *testing.T) {
	h := NewHandler(&fakeStore{}, "")
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/ingest"},
		{http.MethodPost, "/v1/cards"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			handler := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want 401", res.Code)
			}
		})
	}
}
func TestInviteEnrollmentIsOneUseAndReturnsDeviceToken(t *testing.T) {
	store := &fakeStore{inviteHash: tokenHash("one-use")}
	h := NewHandler(store, "")
	mux := http.NewServeMux()
	h.Register(mux)
	wrapped := h.Middleware(mux)
	mux.HandleFunc("GET /v1/cards", func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := cards.TenantFrom(r.Context())
		if !ok || tenant != "tenant" {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	})
	body := `{"invite_code":"one-use","device_name":"phone","platform":"android"}`
	first := httptest.NewRecorder()
	wrapped.ServeHTTP(first, httptest.NewRequest("POST", "/v1/devices/enroll", strings.NewReader(body)))
	if first.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var result struct {
		Token    string `json:"token"`
		DeviceID string `json:"device_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.DeviceID == "" || result.TenantID != "tenant" {
		t.Fatalf("unexpected enrollment response: %+v", result)
	}
	if store.tokenHash == result.Token || len(store.tokenHash) != 64 {
		t.Fatalf("device token was not stored as a SHA-256 hash: %q", store.tokenHash)
	}
	store.device = Device{TenantID: "tenant", DeviceID: result.DeviceID}
	req := httptest.NewRequest("GET", "/v1/cards", nil)
	req.Header.Set("Authorization", "Bearer "+result.Token)
	res := httptest.NewRecorder()
	wrapped.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("token auth status=%d", res.Code)
	}
	second := httptest.NewRecorder()
	wrapped.ServeHTTP(second, httptest.NewRequest("POST", "/v1/devices/enroll", strings.NewReader(body)))
	if second.Code != http.StatusGone {
		t.Fatalf("second enrollment status=%d want 410", second.Code)
	}
}
func TestEnrollHandlerReturnsTooManyRequestsAfterSixtyPerIP(t *testing.T) {
	h := NewHandler(&fakeStore{}, "")
	mux := http.NewServeMux()
	h.Register(mux)
	protected := h.Middleware(mux)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("POST", "/v1/devices/enroll", strings.NewReader(`{"invite_code":"invalid","device_name":"phone","platform":"android"}`))
		req.RemoteAddr = "203.0.113.10:1234"
		res := httptest.NewRecorder()
		protected.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d unexpectedly limited", i+1)
		}
	}
	req := httptest.NewRequest("POST", "/v1/devices/enroll", strings.NewReader(`{"invite_code":"invalid","device_name":"phone","platform":"android"}`))
	req.RemoteAddr = "203.0.113.10:4567"
	res := httptest.NewRecorder()
	protected.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("request 61 status=%d want 429", res.Code)
	}
	req = httptest.NewRequest("POST", "/v1/devices/enroll", strings.NewReader(`{"invite_code":"invalid","device_name":"phone","platform":"android"}`))
	req.RemoteAddr = "203.0.113.11:1234"
	res = httptest.NewRecorder()
	protected.ServeHTTP(res, req)
	if res.Code == http.StatusTooManyRequests {
		t.Fatal("limit was not isolated by client IP")
	}
}

func TestEnrollRateLimitIsSixtyRequestsPerIP(t *testing.T) {
	limiter := NewIPLimiter(60, time.Minute)
	for i := 0; i < 60; i++ {
		if !limiter.Allow("203.0.113.7") {
			t.Fatalf("request %d unexpectedly limited", i+1)
		}
	}
	if limiter.Allow("203.0.113.7") {
		t.Fatal("request 61 was not limited")
	}
	if !limiter.Allow("203.0.113.8") {
		t.Fatal("limit was not isolated by IP")
	}
}
func TestDevTenantIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		name, tenant string
		want         int
	}{{"disabled", "", 401}, {"explicit", "dev-tenant", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(&fakeStore{}, tc.tenant)
			handler := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				d, ok := DeviceFromContext(r.Context())
				if !ok || d.TenantID != tc.tenant {
					t.Errorf("context device=%+v ok=%v", d, ok)
				}
				w.WriteHeader(200)
			}))
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest("GET", "/v1/cards", nil))
			if res.Code != tc.want {
				t.Fatalf("status=%d want %d", res.Code, tc.want)
			}
		})
	}
}

func TestRateLimitAlsoAppliesToAuthenticatedRequests(t *testing.T) {
	store := &fakeStore{tokenHash: tokenHash("device-token"), device: Device{TenantID: "tenant", DeviceID: "device"}}
	h := NewHandler(store, "")
	h.limiter = NewIPLimiter(1, time.Minute)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/cards", func(w http.ResponseWriter, r *http.Request) {
		if tenant, ok := cards.TenantFrom(r.Context()); !ok || tenant != "tenant" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	protected := h.Middleware(mux)
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/cards", nil)
		req.RemoteAddr = "203.0.113.30:1000"
		req.Header.Set("Authorization", "Bearer device-token")
		res := httptest.NewRecorder()
		protected.ServeHTTP(res, req)
		return res
	}
	if res := request(); res.Code != http.StatusOK {
		t.Fatalf("first request status=%d want 200", res.Code)
	}
	if res := request(); res.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status=%d want 429", res.Code)
	}
}
