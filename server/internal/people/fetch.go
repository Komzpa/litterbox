package people

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchPhoto downloads one provider-returned photo URL with a hard size
// bound and verifies it decodes as an image. The URL itself may carry a
// private token, so callers MUST NOT put it into logs, error messages, or
// client-visible payloads: errors below are generic on purpose.
func FetchPhoto(ctx context.Context, client *http.Client, photoURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(photoURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", fmt.Errorf("people photo: unsupported URL")
	}
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("people photo: build request")
	}
	req.Header.Set("Accept", "image/*")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("people photo: fetch failed")
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, "", fmt.Errorf("people photo: status %s", res.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, MaxPhotoBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("people photo: read failed")
	}
	if len(raw) == 0 || len(raw) > MaxPhotoBytes {
		return nil, "", fmt.Errorf("people photo: size out of bounds")
	}
	contentType := normalizeImageType(res.Header.Get("Content-Type"))
	if _, _, err := image.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return nil, "", fmt.Errorf("people photo: not a decodable image")
	}
	return raw, contentType, nil
}

func normalizeImageType(header string) string {
	media := strings.ToLower(strings.TrimSpace(strings.SplitN(header, ";", 2)[0]))
	switch media {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return media
	default:
		return "image/jpeg"
	}
}
