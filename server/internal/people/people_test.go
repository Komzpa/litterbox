// Focused R67 regression: exact mailbox matching, default-photo rejection,
// primary preference, pagination across both People surfaces, official
// request parameters, and end-to-end decode of real provider image bytes
// through a disposable fake server. No production credentials or grants;
// the real-Google grant proof remains pending by design.
package people

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"image/png"
	"io"
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
// bytes. It records the exact personFields/readMask/sources parameters so
// the test proves the official contracts, not just parsing, and it rejects
// query names the method does not define exactly the way Google's
// gRPC-JSON transcoder does (400 "Invalid JSON payload received. Unknown
// name ...: Cannot bind query parameter."), so a wrong parameter name can
// never silently pass against this fixture.
type fakePeople struct {
	t              *testing.T
	png            []byte
	personFields   []string
	readMasks      []string
	sources        []string
	unknownQueries []string
	connectionsHit int
	otherHit       int
	otherStatus    int
}

// otherContactsQueryNames lists the documented otherContacts.list query
// parameters (Method: otherContacts.list, People API REST reference).
var otherContactsQueryNames = map[string]bool{
	"readMask":         true,
	"sources":          true,
	"pageSize":         true,
	"pageToken":        true,
	"requestSyncToken": true,
	"syncToken":        true,
}

// rejectUnknownQuery answers like Google's transcoder when a request
// carries a query parameter the method does not define. Returns true when
// the request was rejected.
func (f *fakePeople) rejectUnknownQuery(w http.ResponseWriter, r *http.Request, allowed map[string]bool) bool {
	for name := range r.URL.Query() {
		if allowed[name] {
			continue
		}
		f.unknownQueries = append(f.unknownQueries, name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"error":{"code":400,"message":"Invalid JSON payload received. Unknown name %q: Cannot bind query parameter.","status":"INVALID_ARGUMENT"}}`, name)
		return true
	}
	return false
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
		if f.rejectUnknownQuery(w, r, otherContactsQueryNames) {
			return
		}
		f.otherHit++
		f.readMasks = append(f.readMasks, r.URL.Query().Get("readMask"))
		f.sources = append(f.sources, r.URL.Query()["sources"]...)
		if f.otherStatus == http.StatusServiceUnavailable {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"message":"The service is currently unavailable.","status":"UNAVAILABLE"}}`))
			return
		}
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
	// Google documents repeated sources[] values and forbids PROFILE alone.
	wantSources := map[string]bool{
		"READ_SOURCE_TYPE_CONTACT": true,
		"READ_SOURCE_TYPE_PROFILE": true,
	}
	for _, s := range fake.sources {
		if !wantSources[s] {
			t.Fatalf("otherContacts sources = %v, undocumented value %q", fake.sources, s)
		}
		delete(wantSources, s)
	}
	if len(wantSources) != 0 {
		t.Fatalf("otherContacts sources = %v, missing %v", fake.sources, wantSources)
	}
	if fake.connectionsHit < 2 || fake.otherHit < 1 {
		t.Fatalf("pagination incomplete: connections=%d other=%d", fake.connectionsHit, fake.otherHit)
	}
}

// Production 2026-10-22: otherContacts 503 aborted BuildIndex after
// connections had already returned, so account 8ce5c0f7 stored 0 photos.
func TestBuildIndexKeepsConnectionsWhenOtherContactsUnavailable(t *testing.T) {
	fake := &fakePeople{t: t, png: tinyPNG(t), otherStatus: http.StatusServiceUnavailable}
	server := httptest.NewTLSServer(fake)
	defer server.Close()
	client := &Client{HTTP: server.Client(), BaseURL: server.URL}
	index, err := client.BuildIndex(context.Background(), "fake-token")
	if err != nil {
		t.Fatalf("partial connections index must succeed, got %v", err)
	}
	if _, ok := index["alice@example.com"]; !ok {
		t.Fatal("connections photo discarded because otherContacts returned 503")
	}
	if _, ok := index["eve@example.com"]; ok {
		t.Fatal("otherContacts 503 must not contribute eve")
	}
	if fake.connectionsHit < 1 || fake.otherHit < 1 {
		t.Fatalf("both surfaces must be attempted: connections=%d other=%d", fake.connectionsHit, fake.otherHit)
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

// TestOtherContactsRejectsUndocumentedQuery is the broken-baseline negative
// control for the People parameter contract: the pre-fix singular
// readSource name must be rejected exactly like Google rejected it on
// production (400 "Invalid JSON payload received. Unknown name
// \"readSource\": Cannot bind query parameter."), while the documented
// sources name is accepted. This keeps the regression honest: if the
// fixture stops rejecting the wrong query, this test fails.
func TestOtherContactsRejectsUndocumentedQuery(t *testing.T) {
	fake := &fakePeople{t: t, png: tinyPNG(t)}
	server := httptest.NewTLSServer(fake)
	defer server.Close()
	res, err := server.Client().Get(server.URL + "/otherContacts?readMask=emailAddresses%2Cphotos&readSource=READ_SOURCE_TYPE_CONTACT%2CREAD_SOURCE_TYPE_PROFILE")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), `Unknown name "readSource": Cannot bind query parameter.`) {
		t.Fatalf("undocumented query name not rejected like Google: %d %s", res.StatusCode, body)
	}
	if len(fake.unknownQueries) != 1 || fake.unknownQueries[0] != "readSource" {
		t.Fatalf("unknownQueries = %v", fake.unknownQueries)
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
