package bimi

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAssertionL(t *testing.T) {
	u, ok := parseAssertionL(`v=BIMI1; l=https://example.test/logo.svg; a=https://example.test/a.pem`)
	if !ok || u != "https://example.test/logo.svg" {
		t.Fatalf("got %q ok=%v", u, ok)
	}
	if _, ok := parseAssertionL(`v=BIMI1; l=none`); ok {
		t.Fatal("l=none must fail")
	}
	if _, ok := parseAssertionL(`v=BIMI1; l=http://insecure.example/x.svg`); ok {
		t.Fatal("http logo must fail")
	}
}

func TestLogoURLForEmailUsesDNS(t *testing.T) {
	prev := lookupTXT
	lookupTXT = func(ctx context.Context, name string) ([]string, error) {
		if name != "default._bimi.linkedin.com" {
			t.Fatalf("lookup name=%q", name)
		}
		return []string{`v=BIMI1; l=https://cdn.example/li.svg`}, nil
	}
	defer func() { lookupTXT = prev }()
	u, ok := LogoURLForEmail(context.Background(), "news@linkedin.com")
	if !ok || u != "https://cdn.example/li.svg" {
		t.Fatalf("got %q ok=%v", u, ok)
	}
}

func TestFetchLogoRasterSVG(t *testing.T) {
	// Minimal SVG; rsvg-convert must be on PATH (production host has librsvg2-bin).
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="#f00"/></svg>`)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write(svg)
	}))
	defer srv.Close()
	pngBytes, ct, err := FetchLogo(context.Background(), srv.Client(), srv.URL+"/logo.svg")
	if err != nil {
		t.Fatalf("FetchLogo: %v", err)
	}
	if ct != "image/png" {
		t.Fatalf("ct=%s", ct)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(pngBytes)); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestFetchLogoRasterPNGPassthrough(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 1, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()
	got, ct, err := FetchLogo(context.Background(), srv.Client(), srv.URL+"/x.png")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" || len(got) == 0 {
		t.Fatalf("ct=%s len=%d", ct, len(got))
	}
}

func TestParseRejectsMissingVWhenMalformed(t *testing.T) {
	// l alone without https still fails URL check
	if _, ok := parseAssertionL(`l=ftp://x`); ok {
		t.Fatal("expected fail")
	}
	_ = strings.Contains
}
