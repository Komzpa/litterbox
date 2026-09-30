package gmailsync

import (
	"context"
	"encoding/base64"
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

func TestClientRetrievesRawMessageWithBase64URLDecoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "expires_in": 3600})
			return
		}
		if r.URL.Path != "/messages/message-a" || r.URL.Query().Get("format") != "raw" {
			t.Errorf("unexpected raw request: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Errorf("missing account auth: %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte("Content-Type: text/plain\r\n\r\nbody"))})
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh-for-account-b"}
	got, err := c.GetMessageRaw(context.Background(), "message-a")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Content-Type: text/plain\r\n\r\nbody" {
		t.Fatalf("raw body = %q", got)
	}
}

func TestClientRejectsMalformedRawMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "expires_in": 3600})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"raw": "%%%"})
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"}
	if _, err := c.GetMessageRaw(context.Background(), "bad"); err == nil {
		t.Fatal("malformed base64url raw message was accepted")
	}
}
