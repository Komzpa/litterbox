package mailhtml

import (
	"bytes"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"strings"
	"testing"
)

func parseLossFixture(t *testing.T, raw string) *Message {
	t.Helper()
	m, err := Parse([]byte(strings.ReplaceAll(raw, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMIMESequentialMixedAndNestedAlternative(t *testing.T) {
	m := parseLossFixture(t, `Content-Type: multipart/mixed; boundary=m

--m
Content-Type: text/plain

first <literal>
--m
Content-Type: multipart/alternative; boundary=a

--a
Content-Type: text/plain

fallback only
--a
Content-Type: text/html

<p>old alternative</p>
--a
Content-Type: text/html

<p>chosen alternative</p>
--a--
--m
Content-Type: text/plain

last section
--m
Content-Type: text/html

<p>final html</p>
--m--
`)
	previous := -1
	for _, want := range []string{"first &lt;literal&gt;", "chosen alternative", "last section", "final html"} {
		i := strings.Index(m.HTML, want)
		if i <= previous {
			t.Fatalf("missing or reordered %q in %s", want, m.HTML)
		}
		previous = i
	}
	for _, absent := range []string{"fallback only", "old alternative"} {
		if strings.Contains(m.HTML, absent) {
			t.Fatalf("duplicate alternative %q: %s", absent, m.HTML)
		}
	}
	if !strings.Contains(m.Text, "fallback only") {
		t.Fatal("plain representation lost")
	}
}

func TestMIMECIDAttachmentAndUnsupported(t *testing.T) {
	m := parseLossFixture(t, `Content-Type: multipart/related; boundary=r

--r
Content-Type: text/html

<p>document<img src="cid:photo"></p>
--r
Content-Type: image/png
Content-ID: <photo>
Content-Disposition: attachment; filename=photo.png
Content-Transfer-Encoding: base64

UE5H
--r
Content-Type: application/x-unknown

opaque bytes
--r--
`)
	if !bytes.Equal(m.Inline["photo"].Data, []byte("PNG")) {
		t.Fatalf("attachment CID lost: %#v", m.Inline)
	}
	if len(m.Attachments) != 2 || string(m.Attachments[1].Data) != "opaque bytes" {
		t.Fatalf("parts lost: %#v", m.Attachments)
	}
	if !strings.Contains(m.HTML, "Open in Gmail") || !strings.Contains(m.HTML, "application/x-unknown") {
		t.Fatalf("unsupported leaf invisible: %s", m.HTML)
	}
}

func TestMIMEISO2022JP(t *testing.T) {
	want := "日本語のお知らせ"
	body := encodeBody(t, want, japanese.ISO2022JP, "base64")
	m := parseLossFixture(t, "Content-Type: text/plain; charset=iso-2022-jp\nContent-Transfer-Encoding: base64\n\n"+body)
	if m.Text != want {
		t.Fatalf("decoded %q want %q", m.Text, want)
	}
}

func TestMIMEDecodeStateAndSource(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"unknown", "Content-Type: text/plain; charset=x-unknown\n\nprice \x96"},
		{"invalid utf8", "Content-Type: text/plain; charset=utf-8\n\nprice \xff"},
		{"invalid japanese", "Content-Type: text/plain; charset=iso-2022-jp\n\n\x1b$B\x24"},
		{"empty", "Content-Type: text/plain\n\n"},
		{"unsupported", "Content-Type: application/x-custom\n\nopaque"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(strings.ReplaceAll(tc.raw, "\n", "\r\n"))
			original := bytes.Clone(raw)
			m, err := Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(m.Raw, original) {
				t.Fatal("raw source changed")
			}
			raw[0] = 'X'
			if !bytes.Equal(m.Raw, original) {
				t.Fatal("raw source aliases caller buffer")
			}
			if m.DecodeStatus == "" || !strings.Contains(strings.ToLower(m.HTML), "open in gmail") {
				t.Fatalf("silent incomplete body: %#v", m)
			}
			if strings.Contains(tc.raw, "price") && !strings.Contains(m.HTML, "price") {
				t.Fatal("decode note hid readable text")
			}
		})
	}
}

func TestMIMEAlternativeEmptyHTMLKeepsPlain(t *testing.T) {
	m := parseLossFixture(t, "Content-Type: multipart/alternative; boundary=a\n\n--a\nContent-Type: text/plain\n\nreadable fallback\n--a\nContent-Type: text/html\n\n\n--a--\n")
	if m.Text != "readable fallback" || strings.TrimSpace(m.HTML) != "" || m.DecodeStatus != "" {
		t.Fatalf("empty HTML hid fallback: %#v", m)
	}
}

func TestMIMERepeatedPlainMixed(t *testing.T) {
	m := parseLossFixture(t, "Content-Type: multipart/mixed; boundary=a\n\n--a\nContent-Type: text/plain\n\nfirst\n--a\nContent-Type: text/plain\n\nsecond\n--a--\n")
	if m.Text != "first\nsecond" || m.HTML != "<pre>first</pre>\n<pre>second</pre>" {
		t.Fatalf("plain mixed lost section: %#v", m)
	}
}

func TestMIMELegacyCharsetsKeepSource(t *testing.T) {
	for _, label := range []string{"koi8-r", "windows-1251"} {
		t.Run(label, func(t *testing.T) {
			enc := charmap.KOI8R
			if label == "windows-1251" {
				enc = charmap.Windows1251
			}
			want := "Привет, мир!"
			raw := []byte("Content-Type: text/plain; charset=" + label + "\r\nContent-Transfer-Encoding: base64\r\n\r\n" + encodeBody(t, want, enc, "base64"))
			m, err := Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if m.Text != want || m.DecodeStatus != "" || !bytes.Equal(m.Raw, raw) {
				t.Fatalf("charset/source lost: %+v", m)
			}
		})
	}
}
