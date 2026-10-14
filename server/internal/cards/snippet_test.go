package cards

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"
)

// The CSV contains complete, unmodified messages.text values exported read-only
// from production tenant 3b09aac5 on 2026-10-12, with source message IDs.
func TestCardSnippetProductionBodies(t *testing.T) {
	file, err := os.Open("testdata/prod-snippets.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range records[1:] {
		name, want := "QGISNegativeControl", "Hi list"
		if strings.Contains(row[2], "#6144:") {
			name, want = "PostGIS6144", "PostgreSQL and POSTGIS version:"
		} else if strings.Contains(row[2], "#6143:") {
			// This reply contains only a symbol fragment, signature and quote.
			name, want = "PostGIS6143", ""
		}
		t.Run(name, func(t *testing.T) {
			from, err := mail.ParseAddress(row[3])
			if err != nil {
				t.Fatal(err)
			}
			got := snippetFromList(t, row[2], from.Name, row[3], row[4])
			t.Logf("message=%s preview=%q", row[1], got)
			if got != want {
				t.Fatalf("preview=%q, want %q", got, want)
			}
		})
	}
}

func TestMailSnippetRules(t *testing.T) {
	cases := []struct{ name, body, title, sender, want string }{
		{"entities-and-dots", "…\n...\n&#8230;\nTom &amp; Jerry &lt;news&gt;", "Subject", "Sender", "Tom & Jerry <news>"},
		{"ascii-separators", "----+----|----\nBody starts here", "Subject", "Sender", "Body starts here"},
		{"box-drawing", "┌────┬────┐\n│    │    │\n└────┴────┘\nBody starts here", "Subject", "Sender", "Body starts here"},
		{"symbol-fragment", "⁰ж\nBody starts here", "Subject", "Sender", "Body starts here"},
		{"sender-only", "Sender\nSignature team", "Subject", "Sender", ""},
		{"quoted-only", "> Earlier body", "Subject", "Sender", ""},
		{"signature-only", "-- \nSignature team", "Subject", "Sender", ""},
		{"short-English", "OK", "Subject", "Sender", "OK"},
		{"short-Cyrillic", "Да", "Subject", "Sender", "Да"},
		{"CJK", "你好", "Subject", "Sender", "你好"},
		{"prose-pipe", "Try A | B", "Subject", "Sender", "Try A | B"},
		{"invisible-filler", "\u034f\u200c\nConfirm your email address\u034f", "Subject", "Sender", "Confirm your email address"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mailSnippet(tc.body, tc.title, tc.sender); got != tc.want {
				t.Fatalf("preview=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestCardSnippetNormalMailUnchanged(t *testing.T) {
	const body = "Latest thread preview"
	if got := snippetFromList(t, "Normal subject", "Sender", "sender@example.test", body); got != body {
		t.Fatalf("preview=%q, want unchanged %q", got, body)
	}
}

func snippetFromList(t *testing.T, title, senderName, sender, text string) string {
	t.Helper()
	// Exercise the real List projection without writes to the production DB.
	// Only database rows are supplied by this test driver; no preview is mocked.
	db := sql.OpenDB(snippetConnector{snippetRowsDriver{title, senderName, sender, text}})
	defer db.Close()
	h := Handler{DB: db, Location: time.UTC}
	req := httptest.NewRequest("GET", "/v1/cards", nil)
	req = req.WithContext(WithTenant(req.Context(), "3b09aac5"))
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != 200 {
		t.Fatalf("List: %d %s", w.Code, w.Body.String())
	}
	var sections Sections
	if err := json.Unmarshal(w.Body.Bytes(), &sections); err != nil {
		t.Fatal(err)
	}
	if len(sections.Now) != 1 {
		t.Fatalf("cards=%+v", sections)
	}
	return sections.Now[0].Snippet
}

type snippetConnector struct{ snippetRowsDriver }
func (c snippetConnector) Connect(context.Context) (driver.Conn, error) {
	return snippetConn{c.snippetRowsDriver}, nil
}
func (c snippetConnector) Driver() driver.Driver { return c.snippetRowsDriver }

type snippetRowsDriver struct{ title, senderName, sender, text string }
func (d snippetRowsDriver) Open(string) (driver.Conn, error) { return snippetConn{d}, nil }
type snippetConn struct{ snippetRowsDriver }
func (c snippetConn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("unexpected prepare") }
func (c snippetConn) Close() error { return nil }
func (c snippetConn) Begin() (driver.Tx, error) { return snippetTx{}, nil }
func (c snippetConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (c snippetConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "FROM cards c") || !strings.Contains(query, "m.text") {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	now := time.Now().UTC()
	return &snippetRows{values: []driver.Value{"card", "mail", c.title, "", nil, false, "open", "", now, int64(0), nil, nil, "owner@example.test", nil, false, true, "", c.senderName, c.sender, now, c.text, "", []byte("[]"), "unknown"}}, nil
}
type snippetTx struct{}
func (snippetTx) Commit() error { return nil }
func (snippetTx) Rollback() error { return nil }
type snippetRows struct {
	values []driver.Value
	done   bool
}
func (r *snippetRows) Columns() []string {
	columns := make([]string, len(r.values))
	for i := range columns {
		columns[i] = fmt.Sprint(i)
	}
	return columns
}
func (r *snippetRows) Close() error { return nil }
func (r *snippetRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	copy(dest, r.values)
	r.done = true
	return nil
}
