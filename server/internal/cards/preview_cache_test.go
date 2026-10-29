package cards

import (
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/mailhtml"
)

func TestPreviewCachePreservesProjectionAndInvalidates(t *testing.T) {
	var cache previewCache
	for _, body := range []string{"<p>First &amp; linked <a href='/'>text</a></p>", "<p>Changed text</p>", "<p>First &amp; linked <a href='/'>text</a></p>"} {
		if got, want := cache.project("tenant", "card", "version", body), mailhtml.PreviewText(body); got != want { t.Fatalf("got %q want %q", got, want) }
	}
	if len(cache.items) != 2 { t.Fatalf("entries = %d", len(cache.items)) }
	cache.project("other", "card", "version", "<p>Changed text</p>")
	if len(cache.items) != 3 { t.Fatalf("tenant entries = %d", len(cache.items)) }
}

func TestPreviewCacheBounds(t *testing.T) {
	var cache previewCache
	for i := range previewCacheEntries + 1 { cache.project("tenant", string(rune(i)), "version", "<p>body</p>") }
	if len(cache.items) != previewCacheEntries { t.Fatalf("entries = %d", len(cache.items)) }
	if cache.bytes > previewCacheBytes { t.Fatalf("bytes = %d", cache.bytes) }
}

func TestJSONCompressionNegotiation(t *testing.T) {
	for _, tc := range []struct { header string; compressed bool }{
		{"gzip", true}, {"br, gzip;q=0", false}, {"*;q=1, gzip;q=0", false}, {"br, *;q=0.5", true}, {"gzip;q=invalid", false}, {"", false},
	} {
		t.Run(tc.header, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/v1/cards", nil)
			r.Header.Set("Accept-Encoding", tc.header)
			out, closeOut := jsonResponseWriter(w, r)
			_, _ = io.WriteString(out, `{"ok":true}`)
			closeOut()
			if got := w.Header().Get("Content-Encoding") == "gzip"; got != tc.compressed { t.Fatalf("compressed = %v", got) }
			body := w.Body.String()
			if tc.compressed {
				reader, err := gzip.NewReader(strings.NewReader(body)); if err != nil { t.Fatal(err) }
				decoded, err := io.ReadAll(reader); if err != nil { t.Fatal(err) }; _ = reader.Close(); body = string(decoded)
			}
			if body != `{"ok":true}` { t.Fatalf("body = %q", body) }
		})
	}
}
