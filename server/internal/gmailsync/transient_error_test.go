package gmailsync

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type transientTestTransport func(*http.Request) (*http.Response, error)

func (f transientTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestArchiveProviderFailureClassification(t *testing.T) {
	for _, kind := range []string{"archive", "bundle_archive", "gmail.archive"} {
		for _, tc := range []struct {
			name string
			status int
			transport error
			token bool
			want int
		}{
			{name: "transport", transport: syscall.ECONNRESET, want: 503},
			{name: "timeout", transport: context.DeadlineExceeded, want: 503},
			{name: "503_exhausted", status: 503, want: 503},
			{name: "429_exhausted", status: 429, want: 503},
			{name: "oauth_ipv6_dial", token: true, transport: &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 443}, Err: syscall.ENETUNREACH}, want: 503},
			{name: "oauth_503", token: true, status: 503, want: 503},
			{name: "oauth_429", token: true, status: 429, want: 503},
			{name: "400_terminal", status: 400, want: 422},
			{name: "404_terminal", status: 404, want: 422},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				f := newSpamFixture(t)
				ctx := context.Background()
				var failures atomic.Int32
				client := &Client{APIBase: "https://provider.test", TokenURL: "https://provider.test/token", RetryDelay: time.Nanosecond}
				client.HTTP = &http.Client{Transport: transientTestTransport(func(r *http.Request) (*http.Response, error) {
					status, body := 200, `{"access_token":"fixture","expires_in":3600}`
					if r.URL.Path != "/token" || tc.token {
						failures.Add(1)
						if tc.transport != nil { return nil, tc.transport }
						status, body = tc.status, `{"error":"fixture provider failure"}`
					}
					return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})}
				useTestOps(func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error) { return client, "thread-1", nil })
				var before, after string
				if err := f.db.QueryRow(ctx, `SELECT to_jsonb(c)::text FROM cards c WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card).Scan(&before); err != nil { t.Fatal(err) }
				op := uuid.New()
				// A dissolved bundle uses its queued card target, just like the real outbox.
				f.post(t, op, f.card, kind, `{}`, tc.want)
				if err := f.db.QueryRow(ctx, `SELECT to_jsonb(c)::text FROM cards c WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card).Scan(&after); err != nil { t.Fatal(err) }
				if after != before { t.Fatalf("card changed: before=%s after=%s", before, after) }
				var receipts int
				if err := f.db.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, f.tenant, op).Scan(&receipts); err != nil { t.Fatal(err) }
				if receipts != 0 { t.Fatalf("failed operation recorded %d receipts", receipts) }
				wantAttempts := int32(1)
				if !tc.token && tc.want == 503 { wantAttempts = 6 }
				if failures.Load() != wantAttempts { t.Fatalf("provider attempts=%d, want %d", failures.Load(), wantAttempts) }
			})
		}
	}
}
