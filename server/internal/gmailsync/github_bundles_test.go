package gmailsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/google/uuid"
)

// Exercise the production Gmail MIME -> saveThread -> AfterIngest path, with
// both a working embedding service and its structured fallback.
func TestGitHubMailSyncKeepsRepositoryBundle(t *testing.T) {
	for _, embeddingWorks := range []bool{true, false} {
		name := "embedding_failure"
		if embeddingWorks {
			name = "embedding_success"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := testPool(t)
			ctx := context.Background()
			tenant, account, plain := uuid.New(), uuid.New(), uuid.New()
			if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1);`, tenant); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'owner@example.test','x')`, tenant, account); err != nil {
				t.Fatal(err)
			}
			// A non-GitHub card forces Cluster to exercise the embedding branch.
			if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,sender,sender_name,subject) VALUES($1,$2,$3,'plain','Jane <jane@example.test>','Jane','ordinary mail')`, tenant, plain, account); err != nil {
				t.Fatal(err)
			}
			embedCalls := 0
			ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				embedCalls++
				if !embeddingWorks {
					http.Error(w, "model unavailable", http.StatusServiceUnavailable)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{{1, 0}}})
			}))
			defer ollama.Close()
			t.Setenv("OLLAMA_HOST", ollama.URL)
			const sender = `"chatgpt-codex-connector[bot]" <notifications@github.com>`
			const subject = "Re: [Komzpa/oh-my-pi] Requirements ledger (PR #39)"
			const body = "Bot review.\nYou are receiving this because you authored the thread.\n"
			headers := []map[string]string{
				{"name": "From", "value": sender}, {"name": "Subject", "value": subject},
				{"name": "List-ID", "value": "<oh-my-pi.Komzpa.github.com>"},
				{"name": "X-GitHub-Reason", "value": "author"},
			}
			gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/token":
					json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "expires_in": 3600})
				case strings.HasSuffix(r.URL.Path, "/threads/github-thread"):
					json.NewEncoder(w).Encode(map[string]any{"id": "github-thread", "messages": []any{map[string]any{"id": "github-message", "internalDate": "1780000000000", "labelIds": []string{"INBOX"}, "payload": map[string]any{"headers": headers}}}})
				case strings.HasSuffix(r.URL.Path, "/messages/github-message"):
					raw := "From: " + sender + "\r\nSubject: " + subject + "\r\nList-ID: <oh-my-pi.Komzpa.github.com>\r\nX-GitHub-Reason: author\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
					json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
				default:
					http.NotFound(w, r)
				}
			}))
			defer gmail.Close()
			client := &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "x"}
			s := &Syncer{DB: db}
			a := Account{TenantID: tenant, ID: account}
			var firstID uuid.UUID
			for pass := range 2 {
				if err := s.saveThread(ctx, a, client, "github-thread", true); err != nil {
					t.Fatal(err)
				}
				var id uuid.UUID
				var key string
				if err := db.QueryRow(ctx, `SELECT c.id,COALESCE(b.bundle_key,'') FROM cards c LEFT JOIN bundles b ON b.tenant_id=c.tenant_id AND b.id=c.bundle_id WHERE c.tenant_id=$1 AND c.gmail_thread_id='github-thread'`, tenant).Scan(&id, &key); err != nil {
					t.Fatal(err)
				}
				if key != "github-agents:Komzpa/oh-my-pi" {
					t.Fatalf("sync pass %d: bundle key=%q, want github-agents:Komzpa/oh-my-pi (never sender:notifications@github.com)", pass, key)
				}
				if pass == 0 {
					firstID = id
				} else if id != firstID {
					t.Fatal("re-sync replaced the existing card")
				}
			}
			if embedCalls != 2 {
				t.Fatalf("embedding calls=%d, want 2 for plain mail only", embedCalls)
			}
			var plainKey string
			if err := db.QueryRow(ctx, `SELECT COALESCE(b.bundle_key,'') FROM cards c LEFT JOIN bundles b ON b.tenant_id=c.tenant_id AND b.id=c.bundle_id WHERE c.id=$1`, plain).Scan(&plainKey); err != nil {
				t.Fatal(err)
			}
			wantPlain := ""
			if !embeddingWorks {
				wantPlain = "sender:jane@example.test"
			}
			if plainKey != wantPlain {
				t.Fatalf("non-GitHub control key=%q, want %q", plainKey, wantPlain)
			}
		})
	}
}

func TestSyncAccountReassignsArchivedGitHubIdempotently(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account, card := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token,history_id) VALUES($1,$2,'owner@example.test','x','1')`, tenant, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,sender,sender_name,subject) VALUES($1,$2,$3,'legacy','archived','GitHub <notifications@github.com>','GitHub','[owner/repo] Issue #308')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := bundles.Assign(ctx, tx, tenant, card, "GitHub <notifications@github.com>", "", "", "normal"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "test", "expires_in": 3600})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/history") {
			json.NewEncoder(w).Encode(map[string]any{"historyId": "1", "history": []any{}})
			return
		}
		http.NotFound(w, r)
	}))
	defer gmail.Close()
	s := &Syncer{DB: db, Client: func(Account) *Client {
		return &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "x"}
	}}
	var firstVersion int64
	for pass := range 2 {
		if err := s.SyncAccount(ctx, Account{TenantID: tenant, ID: account}); err != nil {
			t.Fatal(err)
		}
		var key, state string
		var version int64
		if err := db.QueryRow(ctx, `SELECT b.bundle_key,c.state,c.version FROM cards c JOIN bundles b ON b.tenant_id=c.tenant_id AND b.id=c.bundle_id WHERE c.id=$1`, card).Scan(&key, &state, &version); err != nil {
			t.Fatal(err)
		}
		if key != "github:owner/repo" || state != "archived" {
			t.Fatalf("pass %d: key=%q state=%q, want github:owner/repo archived", pass, key, state)
		}
		if pass == 0 {
			firstVersion = version
		} else if version != firstVersion {
			t.Fatalf("unchanged reassignment bumped version from %d to %d", firstVersion, version)
		}
	}
}
