// Package bimi resolves Brand Indicators for Message Identification logos
// the same way Gmail/MBPs discover brand avatars: public DNS TXT on
// default._bimi.<domain> (and optional selector), then HTTPS GET of the
// Assertion Record l= logo URL.
//
// Spec: https://datatracker.ietf.org/doc/html/draft-brand-indicators-for-message-identification-12
// No OAuth. Personal Google profile photos are out of scope here.
package bimi

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/people"
)

// MaxLogoBytes bounds one BIMI indicator download before rasterization.
const MaxLogoBytes = people.MaxPhotoBytes

// LookupLogoURL resolves default._bimi.<domain> TXT and returns the https
// l= indicator URL. Empty domain or missing/invalid BIMI yields ("", false).
// selector is the BIMI selector label (usually "default").
func LookupLogoURL(ctx context.Context, domain, selector string) (string, bool) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return "", false
	}
	if selector == "" {
		selector = "default"
	}
	name := selector + "._bimi." + domain
	txts, err := lookupTXT(ctx, name)
	if err != nil || len(txts) == 0 {
		return "", false
	}
	for _, raw := range txts {
		if u, ok := parseAssertionL(raw); ok {
			return u, true
		}
	}
	return "", false
}

// LogoURLForEmail returns the BIMI logo URL for the domain of addr, if any.
func LogoURLForEmail(ctx context.Context, addr string) (string, bool) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	at := strings.LastIndexByte(addr, '@')
	if at < 0 || at == len(addr)-1 {
		return "", false
	}
	return LookupLogoURL(ctx, addr[at+1:], "default")
}

func normalizeDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.TrimPrefix(d, ".")
	d = strings.TrimSuffix(d, ".")
	if d == "" || strings.ContainsAny(d, " \t\r\n/") {
		return ""
	}
	return d
}

// lookupTXT is swapped in tests.
var lookupTXT = func(ctx context.Context, name string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolver := net.DefaultResolver
	return resolver.LookupTXT(ctx, name)
}

// parseAssertionL extracts the https l= URL from a BIMI Assertion Record.
// Tags are semicolon-separated; values may be quoted. Requires v=BIMI1.
func parseAssertionL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	// DNS may return chunked quoted strings already joined by LookupTXT.
	s = strings.ReplaceAll(s, `"`, "")
	if s == "" {
		return "", false
	}
	parts := strings.Split(s, ";")
	var version, logo string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(p[:eq]))
		v := strings.TrimSpace(p[eq+1:])
		switch k {
		case "v":
			version = strings.ToUpper(v)
		case "l":
			logo = v
		}
	}
	if version != "" && version != "BIMI1" {
		return "", false
	}
	if logo == "" || strings.EqualFold(logo, "none") {
		return "", false
	}
	u, err := url.Parse(logo)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", false
	}
	return u.String(), true
}

// FetchLogo downloads a BIMI indicator URL and returns PNG bytes suitable
// for senderphoto.Upsert. SVG indicators (the BIMI norm) are rasterized via
// rsvg-convert when present; raster images pass through people.FetchPhoto.
func FetchLogo(ctx context.Context, client *http.Client, logoURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(logoURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", fmt.Errorf("bimi logo: unsupported URL")
	}
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("bimi logo: build request")
	}
	req.Header.Set("Accept", "image/svg+xml,image/*,*/*")
	req.Header.Set("User-Agent", "Litterbox-BIMI/1.0")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("bimi logo: fetch failed")
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, "", fmt.Errorf("bimi logo: status %s", res.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, MaxLogoBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("bimi logo: read failed")
	}
	if len(raw) == 0 || len(raw) > MaxLogoBytes {
		return nil, "", fmt.Errorf("bimi logo: size out of bounds")
	}
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0]))
	if ct == "image/svg+xml" || ct == "text/xml" || ct == "application/xml" || looksLikeSVG(raw) {
		png, err := rasterizeSVG(ctx, raw)
		if err != nil {
			return nil, "", err
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(png)); err != nil {
			return nil, "", fmt.Errorf("bimi logo: raster not decodable")
		}
		return png, "image/png", nil
	}
	// Raster BIMI is rare but allowed; reuse People fetch rules on bytes.
	if _, _, err := image.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return nil, "", fmt.Errorf("bimi logo: not a decodable image")
	}
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return raw, ct, nil
	default:
		return raw, "image/png", nil
	}
}

func looksLikeSVG(b []byte) bool {
	s := strings.TrimSpace(string(b))
	if len(s) > 256 {
		s = s[:256]
	}
	low := strings.ToLower(s)
	return strings.Contains(low, "<svg") || strings.Contains(low, "<!doctype svg")
}

// rasterizeSVGCmd is the converter binary; tests may override.
var rasterizeSVGCmd = "rsvg-convert"

func rasterizeSVG(ctx context.Context, svg []byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "lb-bimi-*")
	if err != nil {
		return nil, fmt.Errorf("bimi logo: temp dir")
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "in.svg")
	out := filepath.Join(dir, "out.png")
	if err := os.WriteFile(in, svg, 0o600); err != nil {
		return nil, fmt.Errorf("bimi logo: write svg")
	}
	cmd := exec.CommandContext(ctx, rasterizeSVGCmd, "-w", "128", "-h", "128", "-f", "png", "-o", out, in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bimi logo: rasterize failed")
	}
	png, err := os.ReadFile(out)
	if err != nil || len(png) == 0 || len(png) > MaxLogoBytes {
		return nil, fmt.Errorf("bimi logo: raster read failed")
	}
	return png, nil
}
