package mailhtml

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/quotedprintable"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// fixtureDir regenerates the R28 acceptance fixtures with the repository's
// own sender and returns the directory holding the .eml files.
func fixtureDir(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	dir := t.TempDir()
	cmd := exec.Command("python3", filepath.Join(root, "tools", "fixtures", "send.py"),
		"--dry-run", "--run-id", "gotest",
		"--to", "inbox@example.invalid",
		"--from-address", "fixtures@example.invalid",
		"--remote-image-url", "https://example.invalid/pixel.png",
		"--output-dir", dir,
		"--manifest", filepath.Join(dir, "manifest.json"))
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("send.py --dry-run: %v\n%s", err, out)
	}
	return dir
}

func parseFixture(t *testing.T, dir, label string) *Message {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, label+"-gotest.eml"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%s): %v", label, err)
	}
	return m
}

func TestParseRichFixture(t *testing.T) {
	m := parseFixture(t, fixtureDir(t), "LB1-RICH")

	if !strings.Contains(m.Text, "Rich HTML acceptance fixture for run gotest") {
		t.Errorf("text body missing or wrong: %q", m.Text)
	}
	for _, want := range []string{
		"<h1>Litterbox rich-mail fixture</h1>",
		`src="https://example.invalid/pixel.png"`, // remote image left for the fetcher
	} {
		if !strings.Contains(m.HTML, want) {
			t.Errorf("html body missing %q in %q", want, m.HTML)
		}
	}

	cidRef := regexp.MustCompile(`src="cid:([^"]+)"`).FindStringSubmatch(m.HTML)
	if len(cidRef) != 2 {
		t.Fatalf("html body has no cid: image reference: %q", m.HTML)
	}
	if len(m.Inline) != 1 {
		t.Fatalf("inline parts = %d, want exactly the one cid image", len(m.Inline))
	}
	part, ok := m.Inline[cidRef[1]]
	if !ok {
		t.Fatalf("html references cid %q but inline keys are %v", cidRef[1], keys(m.Inline))
	}
	if part.ContentType != "image/png" || part.Filename != "pixel.png" {
		t.Errorf("inline part = %q %q, want image/png pixel.png", part.ContentType, part.Filename)
	}
	if !bytes.HasPrefix(part.Data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("inline part data is not the PNG (base64 not decoded?): % x", part.Data)
	}
	if len(m.Attachments) != 0 {
		t.Errorf("attachments = %v, want none", m.Attachments)
	}
}

func TestParsePlainFixture(t *testing.T) {
	m := parseFixture(t, fixtureDir(t), "LB1-A")
	if !strings.Contains(m.Text, "Acceptance fixture LB1-A for run gotest.") {
		t.Errorf("text body missing or wrong: %q", m.Text)
	}
	if m.HTML != "" || len(m.Inline) != 0 || len(m.Attachments) != 0 {
		t.Errorf("plain fixture parsed extra content: html=%q inline=%v attachments=%v", m.HTML, keys(m.Inline), m.Attachments)
	}
}

// encodeBody renders want in enc and wraps it for cte, so each case feeds the
// parser bytes that genuinely are the named charset and transfer encoding.
func encodeBody(t *testing.T, want string, enc encoding.Encoding, cte string) string {
	t.Helper()
	raw := []byte(want)
	if enc != nil {
		var err error
		if raw, err = enc.NewEncoder().Bytes(raw); err != nil {
			t.Fatal(err)
		}
	}
	switch cte {
	case "base64":
		return base64.StdEncoding.EncodeToString(raw)
	case "quoted-printable":
		var buf bytes.Buffer
		w := quotedprintable.NewWriter(&buf)
		if _, err := w.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	default:
		return string(raw)
	}
}

func TestParseCharsets(t *testing.T) {
	cases := []struct {
		name    string
		charset string
		enc     encoding.Encoding // nil for utf-8
		cte     string
		want    string
	}{
		{"utf-8 QP", "utf-8", nil, "quoted-printable", "Привет, мир!"},
		{"windows-1251 base64", "windows-1251", charmap.Windows1251, "base64", "Привет, мир!"},
		{"windows-1251 QP", "windows-1251", charmap.Windows1251, "quoted-printable", "Синхронизация почты"},
		{"koi8-r base64", "koi8-r", charmap.KOI8R, "base64", "Привет, мир!"},
		{"iso-8859-1 QP", "iso-8859-1", charmap.ISO8859_1, "quoted-printable", "Grüße, señor - déjà vu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, subtype := range []string{"plain", "html"} {
				raw := fmt.Sprintf("Subject: charset\r\nContent-Type: text/%s; charset=%q\r\nContent-Transfer-Encoding: %s\r\n\r\n%s\r\n",
					subtype, tc.charset, tc.cte, encodeBody(t, tc.want, tc.enc, tc.cte))
				m, err := Parse([]byte(raw))
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				got := m.Text
				if subtype == "html" {
					got = m.HTML
				}
				if strings.TrimRight(got, "\r\n") != tc.want {
					t.Errorf("text/%s %s decoded = %q, want %q", subtype, tc.charset, got, tc.want)
				}
			}
		})
	}
}

func TestParseMixedAttachment(t *testing.T) {
	attachment := []byte("col1,col2\n1,2\n")
	raw := strings.Join([]string{
		"Subject: mixed",
		"Content-Type: multipart/mixed; boundary=mix",
		"",
		"--mix",
		"Content-Type: multipart/alternative; boundary=alt",
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"plain =C3=9Cmlaut",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>html <b>body</b></p>",
		"--alt--",
		"--mix",
		"Content-Type: text/csv",
		"Content-Transfer-Encoding: base64",
		"Content-Disposition: attachment; filename=\"=?utf-8?b?" + base64.StdEncoding.EncodeToString([]byte("täble.csv")) + "?=\"",
		"",
		base64.StdEncoding.EncodeToString(attachment),
		"--mix--",
		"",
	}, "\r\n")

	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Text != "plain Ümlaut" {
		t.Errorf("text = %q, want %q", m.Text, "plain Ümlaut")
	}
	if m.HTML != "<p>html <b>body</b></p>" {
		t.Errorf("html = %q", m.HTML)
	}
	if len(m.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(m.Attachments))
	}
	got := m.Attachments[0]
	if got.Filename != "täble.csv" {
		t.Errorf("attachment filename = %q, want %q", got.Filename, "täble.csv")
	}
	if got.ContentType != "text/csv" || !bytes.Equal(got.Data, attachment) {
		t.Errorf("attachment = %q %q, want text/csv %q", got.ContentType, got.Data, attachment)
	}
}

func TestParseRelatedStartSelectsDocument(t *testing.T) {
	pixel := []byte("\x89PNG\r\n\x1a\nfake")
	raw := strings.Join([]string{
		"Subject: related",
		"Content-Type: multipart/related; boundary=rel; start=\"<doc@example.invalid>\"",
		"",
		"--rel",
		"Content-Type: image/png",
		"Content-Transfer-Encoding: base64",
		"Content-ID: <img@example.invalid>",
		"",
		base64.StdEncoding.EncodeToString(pixel),
		"--rel",
		"Content-Type: text/html; charset=utf-8",
		"Content-ID: <doc@example.invalid>",
		"",
		"<p>start document <img src=\"cid:img@example.invalid\"></p>",
		"--rel--",
		"",
	}, "\r\n")

	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.HTML != `<p>start document <img src="cid:img@example.invalid"></p>` {
		t.Errorf("html = %q, start parameter not honored", m.HTML)
	}
	part, ok := m.Inline["img@example.invalid"]
	if !ok || !bytes.Equal(part.Data, pixel) {
		t.Errorf("inline img@example.invalid = %v %q, want %q", ok, part.Data, pixel)
	}
}

func keys(m map[string]Part) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
