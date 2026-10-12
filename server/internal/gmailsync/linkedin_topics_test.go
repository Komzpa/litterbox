package gmailsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// These subjects are the reported regression. Bodies deliberately contain only
// synthetic versions of LinkedIn's account-statistics/recipient/footer template.
// The deterministic embedding service distinguishes clean topic inputs from the
// shared template; SPLIT_LINKEDIN_LIVE_HOST optionally exercises the real model
// against the same isolated Gmail/DB fixtures, never a real Gmail account.
func TestLinkedInTopicsStaySeparateThroughGmailSync(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		name := "healthy_model"
		if unavailable {
			name = "unavailable_model"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := testPool(t)
			ctx := context.Background()
			tenant, account := uuid.New(), uuid.New()
			if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'komzpa@gmail.com','fixture')`, tenant, account); err != nil {
				t.Fatal(err)
			}
			const linkedin = "LinkedIn <messages-noreply@linkedin.com>"
			const digest = "0\r\nnew connections,\r\n2\r\nnetwork interactions, and\r\n15\r\npending invitations.\r\n\r\n----------------------------------------\r\nThis email was intended for Fixture Reader\r\nYou are receiving LinkedIn network update digest email.\r\nUnsubscribe: https://www.linkedin.com/fixture\r\nLinkedIn and the LinkedIn logo are registered trademarks of LinkedIn."
			type message struct {
				subject, sender, body, replyTo string
			}
			subjects := []string{
				"OpenDroneMap previews mesh quality leap via OpenMVS",
				"Sunbim tests project simulator for data centre training",
				"OpenDroneMap aligns NodeODM 3.6.2 with ODM release",
			}
			threads := map[string][]message{
				"mesh":    {{subjects[0], linkedin, digest, ""}},
				"sunbim":  {{subjects[1], linkedin, digest, ""}},
				"release": {{subjects[2], linkedin, digest, ""}},
				"reply": {
					{"Planning review", "Jane <jane@example.test>", "Can we review the plan?", ""},
					{"Re: Planning review", "Joe <joe@example.test>", "Yes, tomorrow.", "<reply-0@example.test>"},
				},
				"github-a": {{"Re: [fixture/repo] First issue (#1)", "GitHub <notifications@github.com>", "You are receiving this because you are subscribed to this thread.", ""}},
				"github-b": {{"Re: [fixture/repo] Second issue (#2)", "GitHub <notifications@github.com>", "You are receiving this because you are subscribed to this thread.", ""}},
			}
			ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if unavailable {
					http.Error(w, "model unavailable", http.StatusServiceUnavailable)
					return
				}
				var request struct{ Input string }
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				vector := []float64{0, 0, 0, 1}
				switch request.Input {
				case subjects[0]:
					vector = []float64{1, 0, 0, 0}
				case subjects[1]:
					vector = []float64{0, 1, 0, 0}
				case subjects[2]:
					vector = []float64{0.53, 0, 0.848, 0}
				default:
					if strings.Contains(request.Input, "LinkedIn network update digest") {
						// Shared boilerplate dominates the original embedding input.
						vector = []float64{1, 1, 1, 0}
					}
				}
				json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{vector}})
			}))
			defer ollama.Close()
			t.Setenv("OLLAMA_HOST", ollama.URL)
			t.Setenv("LITTERBOX_EMBED_MODEL", "all-minilm:latest")
			if host := os.Getenv("SPLIT_LINKEDIN_LIVE_HOST"); host != "" && !unavailable {
				t.Setenv("OLLAMA_HOST", host)
			}
			gmail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture", "expires_in": 3600})
					return
				}
				if strings.HasPrefix(r.URL.Path, "/threads/") {
					thread := strings.TrimPrefix(r.URL.Path, "/threads/")
					var messages []any
					for i, m := range threads[thread] {
						id := thread + "-" + string(rune('0'+i))
						messages = append(messages, map[string]any{"id": id, "threadId": thread, "internalDate": "1780000000000", "labelIds": []string{"INBOX"}, "payload": map[string]any{"headers": []map[string]string{{"name": "From", "value": m.sender}, {"name": "To", "value": "komzpa@gmail.com"}, {"name": "Subject", "value": m.subject}}}})
					}
					json.NewEncoder(w).Encode(map[string]any{"id": thread, "messages": messages})
					return
				}
				if strings.HasPrefix(r.URL.Path, "/messages/") {
					id := strings.TrimPrefix(r.URL.Path, "/messages/")
					thread, index := id[:len(id)-2], int(id[len(id)-1]-'0')
					m := threads[thread][index]
					raw := "From: " + m.sender + "\r\nTo: komzpa@gmail.com\r\nSubject: " + m.subject + "\r\nMessage-ID: <" + id + "@example.test>\r\n"
					if m.replyTo != "" {
						raw += "In-Reply-To: " + m.replyTo + "\r\nReferences: " + m.replyTo + "\r\n"
					}
					raw += "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + m.body
					json.NewEncoder(w).Encode(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
					return
				}
				http.NotFound(w, r)
			}))
			defer gmail.Close()
			client := &Client{HTTP: gmail.Client(), APIBase: gmail.URL, TokenURL: gmail.URL + "/token", RefreshToken: "fixture"}
			s := &Syncer{DB: db}
			a := Account{TenantID: tenant, ID: account}
			for _, thread := range []string{"mesh", "sunbim", "release", "reply", "github-a", "github-b"} {
				if err := s.saveThread(ctx, a, client, thread, true); err != nil {
					t.Fatal(err)
				}
			}
			// Seed the old wrong membership, then prove the normal ingestion pass
			// repairs existing rows without a migration or changing thread identity.
			legacy := uuid.New()
			if _, err := db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid,bundle_key) VALUES($1,$2,$3,'{}','semantic:legacy')`, tenant, legacy, subjects[0]); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `UPDATE cards SET bundle_id=$2 WHERE tenant_id=$1 AND gmail_thread_id IN ('mesh','sunbim','release')`, tenant, legacy); err != nil {
				t.Fatal(err)
			}
			if err := s.saveThread(ctx, a, client, "release", true); err != nil {
				t.Fatal(err)
			}
			for _, thread := range []string{"mesh", "sunbim", "release"} {
				var bundle *uuid.UUID
				if err := db.QueryRow(ctx, `SELECT bundle_id FROM cards WHERE tenant_id=$1 AND gmail_thread_id=$2`, tenant, thread).Scan(&bundle); err != nil {
					t.Fatal(err)
				}
				if bundle != nil {
					t.Errorf("unrelated %s notification still bundled: %s", thread, *bundle)
				}
			}
			var cards, messages int
			if err := db.QueryRow(ctx, `SELECT count(DISTINCT c.id),count(m.id) FROM cards c JOIN messages m ON m.tenant_id=c.tenant_id AND m.card_id=c.id WHERE c.tenant_id=$1 AND c.gmail_thread_id='reply'`, tenant).Scan(&cards, &messages); err != nil {
				t.Fatal(err)
			}
			if cards != 1 || messages != 2 {
				t.Errorf("genuine reply thread split: cards=%d messages=%d, want 1/2", cards, messages)
			}
			var githubCards int
			if err := db.QueryRow(ctx, `SELECT count(*) FROM cards c JOIN bundles b ON b.tenant_id=c.tenant_id AND b.id=c.bundle_id WHERE c.tenant_id=$1 AND b.bundle_key='github-fyi:fixture/repo'`, tenant).Scan(&githubCards); err != nil {
				t.Fatal(err)
			}
			if githubCards != 2 {
				t.Errorf("GitHub repository bundle lost: %d members, want 2", githubCards)
			}
		})
	}
}
