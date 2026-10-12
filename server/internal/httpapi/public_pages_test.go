package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
)

func TestPublicPagesUnauthenticated(t *testing.T) {
	mux := http.NewServeMux()
	RegisterPublicPages(mux)
	// A nil store proves these public requests do not perform token lookup.
	handler := auth.NewHandler(nil, "").Middleware(mux)
	for _, tc := range []struct {
		path string
		text []string
	}{
		{"/", []string{"Litterbox", "A personal inbox app by Darafei Praliaskouski.", "komzpa@gmail.com", `href="/privacy"`}},
		{"/privacy", []string{"Litterbox Privacy Policy", "Gmail data is read only to show and sync the owner's own mail inside Litterbox.", "stored on the owner's own server", "not shared with third parties", "not used for ads or model training", "Google API Services User Data Policy", "Limited Use requirements", "komzpa@gmail.com"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("content type = %q", got)
			}
			for _, text := range tc.text {
				if !strings.Contains(response.Body.String(), text) {
					t.Errorf("page is missing %q", text)
				}
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", response.Code)
	}
}
