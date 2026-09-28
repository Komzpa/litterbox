package httpapi

import (
	"io"
	"net/http"
)

// Healthz reports that the HTTP process is able to serve requests.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "{\"ok\":true}\n")
}
