// Focused R67 regression: exact mailbox matching, default-photo rejection,
// primary preference, pagination across both People surfaces, official
// request parameters, and end-to-end decode of real provider image bytes
// through a disposable fake server. No production credentials or grants;
// the real-Google grant proof remains pending by design.
package people

import (
	"bytes"
	"context"
	"image"
	"image/color"
	_ "image/png"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakePeople serves the two documented photo surfaces plus provider image
// bytes. It records the exact personFields/readMask/readSource parameters
// so the test proves the official contracts, not just parsing.
type fakePeople struct {
	t              *testing.T
	png            []byte
	personFields   []string
	readMasks      []string
	readSources    []string
	connectionsHit int
	otherHit       int
}

func (f *fakePeople) photoURL(r *http.Request, name string) string {
	return "https://" + r.Host + "/" + name
}

func (f *fakePeople) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/people/me/connections"):
		f.connectionsHit++
		f.personFields = append(f.personFields, r.URL.Query().Get("personFields"))
		host := r.Host
		_ = host
		page := r.URL.Query().Get("pageToken")
		if page == "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"connections":[
{"emailAddresses":[{"value":"Alice@Example.COM"}],"photos":[
{"url":"https://` + r.Host + `/a.jpg","default":false,"metadata":{"primary":true}},
{"url":"https://` + r.Host + `/default.jpg","default":true}]},
{"emailAddresses":[{"value":"bob@example.com"}],"photos":[
{"url":"https://` + r.Host + `/b.jpg","default":false}]},
{"emailAddresses":[{"value":"carol@example.com"}],"photos":[
{"url":"https://` + r.Host + `/c.jpg","default":true}]}
],"nextPageToken":"p2"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connections":[
{"emailAddresses":[{"value":"dave@example.com"}],"photos":[
{"url":"https://` + r.Host + `/d.jpg","default":false}]},
{"emailAddresses":[{"value":"Frank <frank@example.com>"}],"photos":[
{"url":"https://` + r.Host + `/frank1.jpg","default":false},
{"url":"https://` + r.Host + `/frank2.jpg","default":false,"metadata":{"primary":true}}]}
]}`))
	case strings.HasPrefix(r.URL.Path, "/otherContacts"):
		f.otherHit++
		f.readMasks = append(f.readMasks, r.URL.Query().Get("readMask"))
		f.readSources = append(f.readSources, r.URL.Query().Get("readSource"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"otherContacts":[
{"emailAddresses":[{"value":"eve@example.com"}],"photos":[
{"url":"https://` + r.Host + `/e.jpg","default":false}]}
]}`))
	case strings.HasSuffix(r.URL.Path, ".jpg"):
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(f.png)
	case r.URL.Path == "/not-an-image":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	default:
		http.NotFound(w, r)
	}
}

func TestBuildIndexExactMatchRejectsDefault(t *testing.T) {
	fake := &fakePeople{t: t, png: tinyPNG(t)}
	server := httptest.NewTLSServer(fake)
	defer server.Close()
	client := &Client{HTTP: server.Client(), BaseURL: server.URL}
	index, err := client.BuildIndex(context.Background(), "fake-token")
	if err != nil {
		t.Fatal(err)
	}
	// Exact mailbox matches across both surfaces and both pages.
	for _, key := range []string{"alice@example.com", "bob@example.com", "dave@example.com", "eve@example.com"} {
		url, ok := index[key]
		if !ok || !strings.HasSuffix(url, ".jpg") {
			t.Fatalf("index[%q] = %q, %v; want a photo URL", key, url, ok)
		}
	}
	// Default-only placeholders are never reported as real photos.
	if _, ok := index["carol@example.com"]; ok {
		t.Fatal("default=true photo must not enter the index")
	}
	// Display names are never matched; dots/plus-tags never rewritten.
	if _, ok := index["Alice"]; ok {
		t.Fatal("display names must never match")
	}
	if _, ok := index["alice@example.com"]; !ok {
		t.Fatal("case-insensitive exact match broke")
	}
	// Primary wins over non-primary and over defaults.
	if got := index["alice@example.com"]; !strings.HasSuffix(got, "/a.jpg") {
		t.Fatalf("alice photo = %q, want primary /a.jpg", got)
	}
	// Official request contracts, not just parsing.
	if len(fake.personFields) == 0 || fake.personFields[0] != "emailAddresses,photos" {
		t.Fatalf("connections personFields = %v", fake.personFields)
	}
	if len(fake.readMasks) == 0 || fake.readMasks[0] != "emailAddresses,photos" {
		t.Fatalf("otherContacts readMask = %v", fake.readMasks)
	}
	if len(fake.readSources) == 0 || !strings.Contains(fake.readSources[0], "READ_SOURCE_TYPE_CONTACT") || !strings.Contains(fake.readSources[0], "READ_SOURCE_TYPE_PROFILE") {
		t.Fatalf("otherContacts readSource = %v", fake.readSources)
	}
	if fake.connectionsHit < 2 || fake.otherHit < 1 {
		t.Fatalf("pagination incomplete: connections=%d other=%d", fake.connectionsHit, fake.otherHit)
	}
}

func TestFetchPhotoDecodesRealImage(t *testing.T) {
	fake := &fakePeople{t: t, png: tinyPNG(t)}
	server := httptest.NewTLSServer(fake)
	defer server.Close()
	raw, contentType, err := FetchPhoto(context.Background(), server.Client(), server.URL+"/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q", contentType)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 1 || config.Height != 1 {
		t.Fatalf("decoded size = %dx%d, want 1x1", config.Width, config.Height)
	}
	// Negative controls: non-image payloads and non-https URLs never decode.
	if _, _, err = FetchPhoto(context.Background(), server.Client(), server.URL+"/not-an-image"); err == nil {
		t.Fatal("text payload must not decode as a photo")
	}
	if _, _, err = FetchPhoto(context.Background(), server.Client(), "http://example.test/a.jpg"); err == nil {
		t.Fatal("non-https photo URL must be rejected")
	}
}

func TestNormalizeEmail(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"  Bob@Example.COM ", "bob@example.com"},
		{"alice@example.com", "alice@example.com"},
		{"not-an-email", ""},
		{"", ""},
	} {
		if got := NormalizeEmail(tc.in); got != tc.want {
			t.Fatalf("NormalizeEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
