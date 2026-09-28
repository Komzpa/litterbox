package mailbody

import (
	"context"
	"strings"
	"testing"
)

func TestSanitizeDropsTrackersAndScripts(t *testing.T) {
	got, err := sanitize(context.Background(), `<div><p>Keep me</p><a href="https://sendgrid.net/wf/open?upn=opaque">tracked link</a><script>alert(1)</script><img src="https://mailtrack.io/pixel.gif" width="1" height="1"><img src="https://example.org/photo.jpg" width="300" height="200"></div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Keep me") {
		t.Fatalf("content lost: %s", got)
	}
	if strings.Contains(got, "script") || strings.Contains(got, "mailtrack.io") || strings.Contains(got, "example.org") {
		t.Fatalf("unsafe/unfetched resources remain: %s", got)
	}
}
