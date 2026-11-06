package mailbody

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/mailhtml"
)

type imageResolver struct{}

func (imageResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
}

type imageDialer struct{ address string }

func (d imageDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, d.address)
}

func TestFailedImageKeepsOriginalAndRetries(t *testing.T) {
	fail := true
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if fail {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("image bytes"))
	}))
	defer srv.Close()
	fetcher := mailhtml.NewImageFetcher(imageResolver{}, imageDialer{strings.TrimPrefix(srv.URL, "http://")})
	original := "http://images.example.org/photo.png"
	source := `<img src="` + original + `" width="300" height="200">`
	failed, err := Sanitize(context.Background(), source, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-original-src="` + original + `"`, `data-fetch-state="unavailable"`, `href="` + original + `"`, "Image unavailable"} {
		if !strings.Contains(failed, want) {
			t.Fatalf("missing %s: %s", want, failed)
		}
	}
	if strings.Contains(failed, ` src="https://`) {
		t.Fatal("client must not request remote image", failed)
	}
	fail = false
	recovered, err := Sanitize(context.Background(), failed, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recovered, `src="data:image/png;base64,`) || !strings.Contains(recovered, `data-fetch-state="available"`) || strings.Contains(recovered, "open original") {
		t.Fatalf("retry failed: %s", recovered)
	}
	cached, err := Sanitize(context.Background(), recovered, fetcher)
	if err != nil || cached != recovered || requests != 2 {
		t.Fatalf("cache unstable or re-fetched: requests=%d err=%v", requests, err)
	}
}
func TestTrackerLinkKeepsDestinationButPixelRemoved(t *testing.T) {
	source := `<a href="https://clicks.bisonapp.com/redirect?target=article">Read article</a><img src="https://mailtrack.io/pixel.gif" width="1" height="1">`
	got, err := Sanitize(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `href="https://clicks.bisonapp.com/redirect?target=article"`) {
		t.Fatal("clickable destination lost", got)
	}
	if strings.Contains(got, "mailtrack.io") || strings.Contains(got, "<img") {
		t.Fatal("tracking pixel retained", got)
	}
}
func TestUnavailableImageWithoutFetcherIsExplicit(t *testing.T) {
	got, err := Sanitize(context.Background(), `<img src="https://example.org/photo.png">`, nil)
	if err != nil || !strings.Contains(got, "Image unavailable") || !strings.Contains(got, `data-original-src="https://example.org/photo.png"`) {
		t.Fatalf("%v %s", err, got)
	}
	again, _ := Sanitize(context.Background(), got, nil)
	if strings.Count(again, "open original") != 1 {
		t.Fatal("duplicated placeholder", again)
	}
}
