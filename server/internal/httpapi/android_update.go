package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
)

const MaxAndroidAPKSize = 256 << 20

const AndroidUpdateManifestPath = "/v1/android/update"
const AndroidUpdateAPKPath = "/v1/android/update.apk"

type androidRelease struct {
	version    string
	packageID  string
	apk        []byte
	sha256     string
	configured bool
}

type androidUpdateManifest struct {
	Version      string `json:"version"`
	Package      string `json:"package"`
	SHA256       string `json:"sha256"`
	DownloadPath string `json:"download_path"`
}

// AndroidUpdateHandlers snapshots one configured release at startup so the
// digest and every download describe the same immutable bytes.
func AndroidUpdateHandlers(filePath, version, packageID string) (manifest, download http.Handler) {
	release := &androidRelease{version: version, packageID: packageID}
	if filePath != "" && version != "" && packageID != "" {
		if file, err := os.Open(filePath); err == nil {
			apk, readErr := io.ReadAll(io.LimitReader(file, MaxAndroidAPKSize+1))
			closeErr := file.Close()
			if readErr == nil && closeErr == nil && len(apk) > 0 && len(apk) <= MaxAndroidAPKSize {
				digest := sha256.Sum256(apk)
				release.apk = apk
				release.sha256 = hex.EncodeToString(digest[:])
				release.configured = true
			}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !release.configured {
			http.Error(w, "Android update unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(androidUpdateManifest{
			Version: release.version, Package: release.packageID,
			SHA256: release.sha256, DownloadPath: AndroidUpdateAPKPath,
		})
	}), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !release.configured {
			http.Error(w, "Android update unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="update.apk"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.Itoa(len(release.apk)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(release.apk)
	})
}
