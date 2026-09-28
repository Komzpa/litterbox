package mailhtml

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeResolver map[string][]net.IPAddr

func (r fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ips, ok := r[host]
	if !ok {
		return nil, fmt.Errorf("unexpected hostname %q", host)
	}
	return ips, nil
}

type routeDialer struct{ target string }

func (d routeDialer) DialContext(ctx context.Context, network, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, d.target)
}

func TestFetchImageRejectsPrivateIP(t *testing.T) {
	resolver := fakeResolver{"private.test": {net.IPAddr{IP: net.ParseIP("10.1.2.3")}}}
	fetcher := NewImageFetcher(resolver, routeDialer{target: "127.0.0.1:1"})
	if _, err := fetcher.FetchImage(context.Background(), "http://private.test/image.png"); err == nil || !strings.Contains(err.Error(), "disallowed IP") {
		t.Fatalf("FetchImage error = %v, want private-IP rejection", err)
	}
}

func TestFetchImageRejectsRedirectToPrivateIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://private.test/secret.png", http.StatusFound)
	}))
	defer server.Close()
	resolver := fakeResolver{
		"public.test":  {net.IPAddr{IP: net.ParseIP("203.0.113.10")}},
		"private.test": {net.IPAddr{IP: net.ParseIP("127.0.0.1")}},
	}
	fetcher := NewImageFetcher(resolver, routeDialer{target: server.Listener.Addr().String()})
	if _, err := fetcher.FetchImage(context.Background(), "http://public.test/start.png"); err == nil || !strings.Contains(err.Error(), "disallowed IP") {
		t.Fatalf("FetchImage error = %v, want redirect private-IP rejection", err)
	}
}

func TestFetchImageRejectsOversize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, maxImageBytes+1))
	}))
	defer server.Close()
	fetcher := NewImageFetcher(fakeResolver{"public.test": {net.IPAddr{IP: net.ParseIP("203.0.113.10")}}}, routeDialer{target: server.Listener.Addr().String()})
	if _, err := fetcher.FetchImage(context.Background(), "http://public.test/image.png"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("FetchImage error = %v, want size-cap rejection", err)
	}
}

func TestFetchImageRejectsNonImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()
	fetcher := NewImageFetcher(fakeResolver{"public.test": {net.IPAddr{IP: net.ParseIP("203.0.113.10")}}}, routeDialer{target: server.Listener.Addr().String()})
	if _, err := fetcher.FetchImage(context.Background(), "http://public.test/image.png"); err == nil || !strings.Contains(err.Error(), "non-image") {
		t.Fatalf("FetchImage error = %v, want content-type rejection", err)
	}
}

func TestFetchImageAllowsPublicFakeIP(t *testing.T) {
	const want = "fake public image bytes"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=binary")
		_, _ = w.Write([]byte(want))
	}))
	defer server.Close()
	fetcher := NewImageFetcher(fakeResolver{"public.test": {net.IPAddr{IP: net.ParseIP("198.51.100.7")}}}, routeDialer{target: server.Listener.Addr().String()})
	got, err := fetcher.FetchImage(context.Background(), "http://public.test/image.png")
	if err != nil {
		t.Fatalf("FetchImage: %v", err)
	}
	if string(got.Bytes) != want || got.ContentType != "image/png" {
		t.Fatalf("FetchImage = (%q, %q), want (%q, image/png)", got.Bytes, got.ContentType, want)
	}
}

func TestFetchImageRejectsTrackersBeforeNetwork(t *testing.T) {
	fetcher := NewImageFetcher(fakeResolver{}, routeDialer{target: "127.0.0.1:1"})
	_, err := fetcher.FetchImage(context.Background(), "https://sendgrid.net/wf/open?upn=opaque")
	if err == nil || !strings.Contains(err.Error(), "tracker") {
		t.Fatalf("FetchImage error = %v, want tracker rejection before resolution", err)
	}
}

func TestFetchImageBoundsRedirects(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer server.Close()
	fetcher := NewImageFetcher(fakeResolver{"public.test": {net.IPAddr{IP: net.ParseIP("203.0.113.10")}}}, routeDialer{target: server.Listener.Addr().String()})
	_, err := fetcher.FetchImage(context.Background(), "http://public.test/start")
	if err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("FetchImage error = %v, want redirect-limit rejection", err)
	}
	if requests != maxImageRedirects+1 {
		t.Fatalf("received %d requests, want initial GET plus %d redirects", requests, maxImageRedirects)
	}
}

func TestDialControlRejectsNonPublicIPs(t *testing.T) {
	for _, ip := range []string{"10.0.0.1", "127.0.0.1", "169.254.1.2", "224.0.0.1", "0.0.0.0", "::", "::1", "fd00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1"} {
		t.Run(ip, func(t *testing.T) {
			if err := rejectUnsafeDialAddress("tcp", net.JoinHostPort(ip, "80"), nil); err == nil {
				t.Fatal("dial control accepted a non-public IP")
			}
		})
	}
}

type inspectingDialer struct {
	t          *testing.T
	want, dest string
}

func (d inspectingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if addr != d.want {
		d.t.Errorf("dialed %q, want pinned numeric address %q", addr, d.want)
	}
	return (&net.Dialer{}).DialContext(ctx, network, d.dest)
}

func TestFetchImagePinsResolvedAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()
	fetcher := NewImageFetcher(fakeResolver{"public.test": {net.IPAddr{IP: net.ParseIP("203.0.113.10")}}}, inspectingDialer{t: t, want: "203.0.113.10:80", dest: server.Listener.Addr().String()})
	if _, err := fetcher.FetchImage(context.Background(), "http://public.test/image.png"); err != nil {
		t.Fatal(err)
	}
}
