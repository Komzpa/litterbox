package diagnostics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
)

const javaBody = "schema=1\nversion=0.1.53\nbuild=53\npackage=org.qtproject.example.litterbox_qt\nandroid=34\ntimestamp=1793000000000\nkind=java\nreason=uncaught\nstatus=unknown\nstack=frames\nversion_scope=current-process\njava.lang.IllegalStateException\n  at org.litterbox.CrashStore.save:42\n"
const exitBody = "schema=1\nversion=unknown\nbuild=unknown\npackage=org.qtproject.example.litterbox_qt\nandroid=34\ntimestamp=1793000000000\nkind=exit\nreason=5\nstatus=-11\nstack=unavailable\nversion_scope=unknown-historical-process\n"

func wire(r Report) string { b, _ := json.Marshal(r); return string(b) }
func sample() Report       { return Report{ID: "java-123", Body: javaBody, UploaderBuild: "abcd123-dirty"} }

type tokenStore struct{ auth.Store }

func (tokenStore) DeviceByToken(_ context.Context, hash string) (auth.Device, error) {
	sum := sha256.Sum256([]byte("valid-token"))
	if hash != hex.EncodeToString(sum[:]) {
		return auth.Device{}, errors.New("unauthorized")
	}
	return auth.Device{TenantID: "tenant", DeviceID: "device"}, nil
}

type memoryStore struct {
	saves  int
	device auth.Device
	report Report
	fail   bool
}

func (s *memoryStore) Save(_ context.Context, d auth.Device, r Report, _, _ string) error {
	if s.fail {
		return errors.New("offline")
	}
	s.saves++
	s.device = d
	s.report = r
	return nil
}
func request(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRegisteredRouteRequiresDeviceAndAcknowledgesSavedID(t *testing.T) {
	s := &memoryStore{}
	mux := http.NewServeMux()
	Register(mux, s)
	h := auth.NewHandler(tokenStore{}, "").Middleware(mux)
	for _, token := range []string{"", "wrong-token"} {
		if w := request(h, "POST", Path, token, wire(sample())); w.Code != 401 {
			t.Fatalf("unauthorized: %d", w.Code)
		}
	}
	if s.saves != 0 {
		t.Fatal("unauthorized request stored")
	}
	w := request(h, "POST", Path, "valid-token", wire(sample()))
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"ack":"java-123"}` || s.saves != 1 || s.device.TenantID != "tenant" || s.device.DeviceID != "device" {
		t.Fatalf("registered route: %d %s %+v", w.Code, w.Body.String(), s)
	}
	// A middleware 401 alone would also pass for an unregistered route. These
	// authenticated negative controls prove method/path registration boundaries.
	if w := request(h, "GET", Path, "valid-token", ""); w.Code != 405 {
		t.Fatalf("GET: %d", w.Code)
	}
	if w := request(h, "POST", Path+"/missing", "valid-token", "{}"); w.Code != 404 {
		t.Fatalf("unknown route: %d", w.Code)
	}
	s.fail = true
	if w := request(h, "POST", Path, "valid-token", wire(sample())); w.Code != 503 || strings.Contains(w.Body.String(), "ack") {
		t.Fatalf("failed persistence ACK: %d %s", w.Code, w.Body.String())
	}
	dev := auth.NewHandler(tokenStore{}, "tenant").Middleware(mux)
	if w := request(dev, "POST", Path, "", wire(sample())); w.Code != 401 {
		t.Fatalf("synthetic dev device: %d", w.Code)
	}
}

func TestMalformedAndSensitiveReportsNeverReachStore(t *testing.T) {
	s := &memoryStore{}
	mux := http.NewServeMux()
	Register(mux, s)
	h := auth.NewHandler(tokenStore{}, "").Middleware(mux)
	for name, body := range map[string]string{
		"unknown attribution": strings.TrimSuffix(wire(sample()), "}") + `,"device_id":"victim","tenant_id":"victim"}`,
		"trailing json":       wire(sample()) + `{}`,
		"broken json":         `{`,
	} {
		t.Run(name, func(t *testing.T) {
			if w := request(h, "POST", Path, "valid-token", body); w.Code != 400 {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}
	for name, mutate := range map[string]func(*Report){
		"exception message":  func(r *Report) { r.Body += "secret mail body: token=credential\n" },
		"url metadata":       func(r *Report) { r.Body += "url=https://secret.invalid/token\n" },
		"duplicate metadata": func(r *Report) { r.Body += "kind=exit\n" },
		"missing metadata":   func(r *Report) { r.Body = "kind=java\n" },
		"wrong package": func(r *Report) {
			r.Body = strings.ReplaceAll(r.Body, "org.qtproject.example.litterbox_qt", "other.package")
		},
		"traversal":          func(r *Report) { r.ID = "../report" },
		"oversize":           func(r *Report) { r.Body = strings.Repeat("x", MaxBody+1) },
		"sensitive uploader": func(r *Report) { r.UploaderBuild = "https://secret.invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			r := sample()
			mutate(&r)
			if w := request(h, "POST", Path, "valid-token", wire(r)); w.Code != 422 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
	if s.saves != 0 {
		t.Fatalf("invalid writes=%d", s.saves)
	}
}

func TestHistoricalExitAndAvailabilityGrammar(t *testing.T) {
	r := sample()
	r.ID = "exit-123"
	r.Body = exitBody
	if kind, version, err := Validate(r); err != nil || kind != "exit" || version != "unknown" {
		t.Fatalf("exit: %s %s %v", kind, version, err)
	}
	r.Body = strings.ReplaceAll(strings.ReplaceAll(exitBody, "reason=5", "reason=unavailable"), "status=-11", "status=unknown") + "history=empty\n"
	if _, _, err := Validate(r); err != nil {
		t.Fatal(err)
	}
	r.Body = strings.ReplaceAll(exitBody, "version=unknown", "version=0.1.53")
	if _, _, err := Validate(r); err == nil {
		t.Fatal("historical build misattribution accepted")
	}
	for _, body := range []string{exitBody + "java.lang.Error\n", exitBody + "history=empty\n", strings.ReplaceAll(exitBody, "reason=5", "reason=uncaught"), strings.ReplaceAll(exitBody, "reason=5", "reason=unavailable")} {
		r.Body = body
		if _, _, err := Validate(r); err == nil {
			t.Fatalf("inconsistent historical evidence accepted: %q", body)
		}
	}
}

func TestFullSpoolDoesNotConsumeOrdinaryQuota(t *testing.T) {
	for _, status := range []int{200, 422, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := &memoryStore{fail: status == 503}
			mux := http.NewServeMux()
			Register(mux, s)
			a := auth.NewHandler(tokenStore{}, "")
			a.Register(mux) // /v1/invites is a real authenticated owner operation.
			mux.HandleFunc("GET /v1/mail/body", func(w http.ResponseWriter, r *http.Request) {
				if _, ok := auth.DeviceFromContext(r.Context()); !ok {
					t.Error("mail request lost device identity")
				}
				w.WriteHeader(200)
			})
			h := a.Middleware(mux)
			r := sample()
			if status == 422 {
				r.Body = "private message"
			}
			ordinary := 0
			for i := range 64 {
				if w := request(h, "POST", Path, "valid-token", wire(r)); w.Code != status {
					t.Fatalf("diagnostic %d: %d want %d", i+1, w.Code, status)
				}
				if i%4 == 0 {
					if w := request(h, "GET", "/v1/mail/body", "valid-token", ""); w.Code != 200 {
						t.Fatalf("interleaved mail: %d", w.Code)
					}
					if w := request(h, "POST", "/v1/invites", "valid-token", "{}"); w.Code != 201 {
						t.Fatalf("interleaved owner operation: %d", w.Code)
					}
					ordinary += 2
				}
			}
			if w := request(h, "POST", Path, "valid-token", wire(r)); w.Code != 429 {
				t.Fatalf("diagnostic 65: %d want 429", w.Code)
			}
			for ; ordinary < auth.DeviceBurst; ordinary++ {
				if w := request(h, "GET", "/v1/mail/body", "valid-token", ""); w.Code != 200 {
					t.Fatalf("ordinary %d: %d", ordinary+1, w.Code)
				}
			}
			if w := request(h, "GET", "/v1/mail/body", "valid-token", ""); w.Code != 429 {
				t.Fatalf("ordinary %d: %d want 429", auth.DeviceBurst+1, w.Code)
			}
		})
	}
}

func (tokenStore) CreateInvite(context.Context, string, string) error { return nil }
