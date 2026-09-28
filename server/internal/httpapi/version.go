package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const APIVersion = 1

func Version(serverBuild string, minClientAPI int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			API          int    `json:"api"`
			MinClientAPI int    `json:"min_client_api"`
			ServerBuild  string `json:"server_build"`
		}{APIVersion, minClientAPI, serverBuild})
	}
}

func BuildHeader(build string, api int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Litterbox-Build", build)
		w.Header().Set("X-Litterbox-Api", fmt.Sprint(api))
		next.ServeHTTP(w, r)
	})
}
