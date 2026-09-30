package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
)

type updateAuthFixture struct {
	tokenHash string
}

func (f updateAuthFixture) RedeemInvite(context.Context, string, string, string, string, string) (string, error) {
	return "", errors.New("not used in update smoke")
}

func (f updateAuthFixture) DeviceByToken(_ context.Context, hash string) (auth.Device, error) {
	if hash != f.tokenHash {
		return auth.Device{}, errors.New("unknown device token")
	}
	return auth.Device{TenantID: "00000000-0000-4000-8000-000000000001", DeviceID: "00000000-0000-4000-8000-000000000002"}, nil
}

func (f updateAuthFixture) CreateInvite(context.Context, string, string) error {
	return errors.New("not used in update smoke")
}

func (f updateAuthFixture) RevokeDevice(context.Context, string, string) error {
	return errors.New("not used in update smoke")
}

func TestAndroidUpdateRoutesThroughDeviceAuthMiddleware(t *testing.T) {
	token := "disposable-update-device-token"
	tokenDigest := sha256.Sum256([]byte(token))
	fixture := updateAuthFixture{tokenHash: hex.EncodeToString(tokenDigest[:])}
	artifact := []byte("temporary signed-apk fixture")
	path := filepath.Join(t.TempDir(), "release.apk")
	if err := os.WriteFile(path, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, download := AndroidUpdateHandlers(path, "2.4.1", "org.example.litterbox")
	mux := http.NewServeMux()
	mux.Handle("GET "+AndroidUpdateManifestPath, manifest)
	mux.Handle("GET "+AndroidUpdateAPKPath, download)
	protected := auth.NewHandler(fixture, "").Middleware(mux)

	request := func(path, bearer string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		response := httptest.NewRecorder()
		protected.ServeHTTP(response, req)
		return response
	}
	for _, route := range []string{AndroidUpdateManifestPath, AndroidUpdateAPKPath} {
		if got := request(route, ""); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s status=%d want=%d", route, got.Code, http.StatusUnauthorized)
		}
	}
	manifestResponse := request(AndroidUpdateManifestPath, token)
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("authorized manifest status=%d body=%s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var got androidUpdateManifest
	if err := json.Unmarshal(manifestResponse.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	artifactDigest := sha256.Sum256(artifact)
	if got.SHA256 != hex.EncodeToString(artifactDigest[:]) || got.DownloadPath != AndroidUpdateAPKPath {
		t.Fatalf("authorized manifest = %+v", got)
	}
	downloadResponse := request(got.DownloadPath, token)
	if downloadResponse.Code != http.StatusOK || !bytes.Equal(downloadResponse.Body.Bytes(), artifact) {
		t.Fatalf("authorized download status=%d bytes=%q", downloadResponse.Code, downloadResponse.Body.Bytes())
	}

	unavailableManifest, unavailableDownload := AndroidUpdateHandlers("", "", "")
	unavailableMux := http.NewServeMux()
	unavailableMux.Handle("GET "+AndroidUpdateManifestPath, unavailableManifest)
	unavailableMux.Handle("GET "+AndroidUpdateAPKPath, unavailableDownload)
	unavailableProtected := auth.NewHandler(fixture, "").Middleware(unavailableMux)
	for _, route := range []string{AndroidUpdateManifestPath, AndroidUpdateAPKPath} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		unavailableProtected.ServeHTTP(response, req)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("configured fixture should fail closed on %s: status=%d", route, response.Code)
		}
	}
}

func TestAndroidUpdateManifestAndDownloadUseSameConfiguredBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.apk")
	original := []byte("immutable apk candidate")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, download := AndroidUpdateHandlers(path, "2.4.1", "org.example.litterbox")
	if err := os.WriteFile(path, []byte("replacement bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifestResponse := httptest.NewRecorder()
	manifest.ServeHTTP(manifestResponse, httptest.NewRequest(http.MethodGet, AndroidUpdateManifestPath, nil))
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("manifest status = %d: %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var got androidUpdateManifest
	if err := json.Unmarshal(manifestResponse.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(original)
	if got.Version != "2.4.1" || got.Package != "org.example.litterbox" || got.SHA256 != hex.EncodeToString(digest[:]) || got.DownloadPath != AndroidUpdateAPKPath {
		t.Fatalf("manifest = %+v", got)
	}

	downloadResponse := httptest.NewRecorder()
	download.ServeHTTP(downloadResponse, httptest.NewRequest(http.MethodGet, AndroidUpdateAPKPath, nil))
	if downloadResponse.Code != http.StatusOK || downloadResponse.Body.String() != string(original) {
		t.Fatalf("download status=%d bytes=%q", downloadResponse.Code, downloadResponse.Body.Bytes())
	}
}

func TestAndroidUpdateFailsClosedWithoutConfiguredRelease(t *testing.T) {
	manifest, download := AndroidUpdateHandlers("", "", "")
	for name, handler := range map[string]http.Handler{"manifest": manifest, "download": download} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})
	}
}

func TestAndroidUpdateRejectsOversizedRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.apk")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxAndroidAPKSize + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	manifest, _ := AndroidUpdateHandlers(path, "2.4.1", "org.example.litterbox")
	response := httptest.NewRecorder()
	manifest.ServeHTTP(response, httptest.NewRequest(http.MethodGet, AndroidUpdateManifestPath, nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
