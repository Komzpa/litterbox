package cards

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/testdb"
)

// Exact contiguous excerpts read from tenant 3b09aac5-be4b-43d8-abca-f7b1db66f8bc
// on 2026-10-12. Keep source content distinct from preview-only cleanup.
var realSnippetCases = []struct {
	name, sender, text, want string
}{
	{
		name: "Kyivstar entities", sender: "Київстар",
		text: " ✉️ А раптом цей знак? :) І ще акції, бонуси, подарунки та інші чудові новини 👉&#8199;&#847; &#8199;&#847; &#8199;&#847;",
		want: "✉️ А раптом цей знак? :) І ще акції, бонуси, подарунки та інші чудові новини 👉",
	},
	{
		name: "PostGIS table rule", sender: "PostGIS",
		text: "#6144: ST_CreateOverview function errors\r\n-------------------------+---------------------------\r\n Reporter:  amorevosgeo\tOwner:  pramsey",
		want: "#6144: ST_CreateOverview function errors Reporter: amorevosgeo Owner: pramsey",
	},
	{
		name: "Flying Blue sender", sender: "Flying Blue",
		text: "Flying Blue\r\n \r\n \r\n  \r\n\r\n \r\n\r\n\r\n\r\n\r\n \r\n\r\n\r\n\r\n  &zwnj;&nbsp;&zwnj;&nbsp;",
		want: "",
	},
	{
		// Both stored text and HTML contain U+2070 U+0436; not a charset bug.
		name: "PostGIS genuine source text", sender: "Darafei \"Komяpa\" Praliaskouski",
		text: "⁰ж", want: "⁰ж",
	},
}

func TestCleanSnippetRealStrings(t *testing.T) {
	for _, tc := range realSnippetCases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanSnippet(tc.text, tc.sender)
			if got != tc.want {
				t.Fatalf("snippet=%q, want %q", got, tc.want)
			}
			if again := cleanSnippet(got, tc.sender); again != got {
				t.Fatalf("not idempotent: %q -> %q", got, again)
			}
		})
	}
}

func TestCleanSnippetControls(t *testing.T) {
	cases := []struct{ text, sender, want string }{
		{"\u2007\u034f\u200b\u200c\u200d\u2060\ufeff\u202eHello\t world\r\n...\n-----+-----\nNext", "", "Hello world Next"},
		{"&amp;lt;tag&amp;gt; &amp; text", "", "<tag> & text"},
		{"Flying Blue\nYour booking is ready.", "Flying Blue", "Your booking is ready."},
		{"ordinary punctuation: C++ costs $5.00! 😀", "", "ordinary punctuation: C++ costs $5.00! 😀"},
		{"Здравствуйте! Дзякуй. café", "", "Здравствуйте! Дзякуй. café"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if got := cleanSnippet(tc.text, tc.sender); got != tc.want {
			t.Errorf("cleanSnippet(%q, %q)=%q, want %q", tc.text, tc.sender, got, tc.want)
		}
		if got := cleanSnippet(tc.want, tc.sender); got != tc.want {
			t.Errorf("re-cleaned %q=%q", tc.want, got)
		}
	}
}

func TestListRecleansStoredSnippets(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("CARD_TEST_POSTGRES=1 required; use a disposable pg_virtualenv")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, name := range []string{"001_mail.sql", "002_security.sql", "003_agent_cards.sql", "004_card_time_note.sql", "005_card_notify.sql", "008_bundles.sql", "009_ingest.sql", "011_card_bodies.sql", "015_card_note_updated.sql", "017_card_sender_name.sql"} {
		body, err := os.ReadFile(filepath.Join("../../db", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := "11111111-1111-4111-8111-111111111111"
	account := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'preview@example.com',decode('00','hex'))`, tenant, account); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(db, "UTC", filepath.Join(t.TempDir(), "feedback.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range realSnippetCases {
		t.Run(tc.name, func(t *testing.T) {
			var card string
			if err := db.QueryRow(`INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender,sender_name) VALUES($1,gen_random_uuid(),$2,$3,$3,$4,$4) RETURNING id::text`, tenant, account, tc.name, tc.sender).Scan(&card); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,text,body_hash,received_at) VALUES($1,gen_random_uuid(),$2,$3,$4,decode('00','hex'),now())`, tenant, card, tc.name, tc.text); err != nil {
				t.Fatal(err)
			}
			// Simulate two client startup/sync pulls with no Gmail re-import.
			for pass := range 2 {
				req := httptest.NewRequest("GET", "/v1/cards", nil)
				req = req.WithContext(WithTenant(req.Context(), tenant))
				w := httptest.NewRecorder()
				handler.List(w, req)
				if w.Code != http.StatusOK {
					t.Fatalf("GET: %d %s", w.Code, w.Body.String())
				}
				var sections Sections
				if err := json.Unmarshal(w.Body.Bytes(), &sections); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, c := range append(append(sections.Now, sections.Later...), sections.Missed...) {
					if c.ID == card {
						found = true
						if c.Snippet != tc.want {
							t.Errorf("sync %d snippet=%q, want %q", pass, c.Snippet, tc.want)
						}
					}
				}
				if !found {
					t.Fatal("card absent from list")
				}
			}
			var stored string
			if err := db.QueryRow(`SELECT text FROM messages WHERE tenant_id=$1 AND card_id=$2`, tenant, card).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if stored != tc.text {
				t.Fatalf("preview changed source body: %q", stored)
			}
		})
	}
}
