package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionContractAndBuildHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", Version("abc123", 1))
	response := httptest.NewRecorder()
	BuildHeader("abc123", APIVersion, mux).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/version", nil))
	if response.Header().Get("X-Litterbox-Build") != "abc123" {
		t.Fatalf("build header = %q", response.Header().Get("X-Litterbox-Build"))
	}
	if response.Header().Get("X-Litterbox-Api") != "1" {
		t.Fatalf("API header = %q", response.Header().Get("X-Litterbox-Api"))
	}
	var body struct {
		API          int    `json:"api"`
		MinClientAPI int    `json:"min_client_api"`
		ServerBuild  string `json:"server_build"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.API != 1 || body.MinClientAPI != 1 || body.ServerBuild != "abc123" {
		t.Fatalf("version body = %#v", body)
	}
}
