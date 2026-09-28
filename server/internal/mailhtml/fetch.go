package mailhtml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Komzpa/litterbox/server/internal/mailhtml/trackers"
)

const (
	maxImageBytes     = 10 << 20
	maxImageRedirects = 5
	imageFetchTimeout = 15 * time.Second
)

// ImageFetcher retrieves bounded image resources from public HTTP(S) URLs.
// A resolver and dialer may be supplied for deterministic callers/tests; nil
// dependencies use the system resolver and a dialer that rechecks every IP.
type ImageFetcher struct {
	resolver IPResolver
	dialer   ContextDialer
}

// IPResolver resolves a hostname before dialing it.
type IPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// ContextDialer establishes a connection to an already-resolved address.
type ContextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Image is the fetched image bytes and their validated media type.
type Image struct {
	Bytes       []byte
	ContentType string
}

// NewImageFetcher constructs a fetcher. Supplying nil dependencies selects
// safe system defaults.
func NewImageFetcher(resolver IPResolver, dialer ContextDialer) *ImageFetcher {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dialer == nil {
		dialer = &net.Dialer{Control: rejectUnsafeDialAddress}
	}
	return &ImageFetcher{resolver: resolver, dialer: dialer}
}

// FetchImage downloads one public image, rejecting trackers, unsafe network
// destinations, redirects beyond the limit, non-image content, and oversized
// bodies.
func (f *ImageFetcher) FetchImage(ctx context.Context, rawURL string) (Image, error) {
	var empty Image
	if f == nil || f.resolver == nil || f.dialer == nil {
		return empty, errors.New("image fetcher is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, imageFetchTimeout)
	defer cancel()

	transport := &http.Transport{DialContext: f.dialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   imageFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxImageRedirects {
				return errors.New("too many image redirects")
			}
			return validateImageURL(req.URL)
		},
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return empty, fmt.Errorf("invalid image URL: %w", err)
	}
	if err := validateImageURL(parsed); err != nil {
		return empty, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return empty, fmt.Errorf("create image request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return empty, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return empty, fmt.Errorf("image server returned %s", resp.Status)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return empty, errors.New("image response has non-image content type")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return empty, fmt.Errorf("read image response: %w", err)
	}
	if len(body) > maxImageBytes {
		return empty, fmt.Errorf("image response exceeds %d bytes", maxImageBytes)
	}
	return Image{Bytes: body, ContentType: strings.ToLower(mediaType)}, nil
}

func (f *ImageFetcher) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid image destination: %w", err)
	}
	ips, err := f.resolver.LookupIPAddr(ctx, strings.TrimSuffix(host, "."))
	if err != nil {
		return nil, fmt.Errorf("resolve image destination: %w", err)
	}
	if len(ips) == 0 {
		return nil, errors.New("image destination resolved to no IP addresses")
	}
	for _, candidate := range ips {
		if !publicIP(candidate.IP) {
			return nil, fmt.Errorf("image destination resolves to disallowed IP %s", candidate.IP)
		}
	}
	var lastErr error
	for _, candidate := range ips {
		conn, dialErr := f.dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("dial image destination: %w", lastErr)
}

func validateImageURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return errors.New("image URL must be an absolute HTTP(S) URL without credentials")
	}
	if isTracker, _ := trackers.IsTracker(u.String()); isTracker {
		return errors.New("refusing to fetch tracker image")
	}
	if port := u.Port(); port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return errors.New("invalid image URL port")
		}
	}
	return nil
}

func publicIP(ip net.IP) bool {
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsMulticast() && !ip.IsUnspecified()
}

func rejectUnsafeDialAddress(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if !publicIP(ip) {
		return fmt.Errorf("refusing connection to disallowed IP %q", host)
	}
	return nil
}
