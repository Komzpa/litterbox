package mailhtml

import "testing"

func TestPreviewText(t *testing.T) {
 for _, tc := range []struct{name, body, want string}{
  {"linked-prose", `<p>Please <a href="https://example.test/track">read the report</a> today.</p>`, "Please read the report today."},
  {"link-only", `<a href="https://example.test">Read the report</a>`, "Read the report"},
  {"short-reply", `<p>OK</p>`, "OK"},
  {"newsletter", `<div hidden>Hidden preheader</div><a href="https://example.test"><img alt="Company logo"></a><p>News &amp; events</p>`, "News & events"},
 } { t.Run(tc.name,func(t *testing.T){ if got:=PreviewText(tc.body);got!=tc.want {t.Fatalf("got %q, want %q",got,tc.want)} }) }
}
