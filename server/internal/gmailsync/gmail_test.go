package gmailsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientRefreshesTokenAndModifiesThread(t *testing.T) {
	var refreshes, modifications atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			refreshes.Add(1)
			if e := r.ParseForm(); e != nil {
				t.Error(e)
			}
			if r.Form.Get("refresh_token") != "refresh-for-account-b" {
				t.Errorf("wrong refresh token %q", r.Form.Get("refresh_token"))
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "expires_in": 3600})
		case "/threads/thread-b/modify":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("bad auth: %s", r.Header.Get("Authorization"))
			}
			var body map[string][]string
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
				t.Error(e)
			}
			if len(body["removeLabelIds"]) != 1 || body["removeLabelIds"][0] != "INBOX" {
				t.Errorf("wrong archive request: %#v", body)
			}
			modifications.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh-for-account-b"}
	if e := c.Modify(context.Background(), "thread-b", nil, []string{"INBOX"}); e != nil {
		t.Fatal(e)
	}
	if e := c.Modify(context.Background(), "thread-b", nil, []string{"INBOX"}); e != nil {
		t.Fatal(e)
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refresh calls=%d", refreshes.Load())
	}
	if modifications.Load() != 2 {
		t.Fatalf("modify calls=%d", modifications.Load())
	}
}
