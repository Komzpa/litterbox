package trackers

import "testing"

func TestIsTracker(t *testing.T) {
	cases := []struct {
		name, url, wantVendor string
		want                  bool
	}{
		{name: "Mailchimp open pixel", url: "https://us12.mailchimp.com/mctx/opens/123", wantVendor: "Intuit", want: true},
		{name: "SendGrid open pixel", url: "https://sendgrid.net/wf/open?upn=opaque", wantVendor: "SendGrid", want: true},
		{name: "normal CDN image", url: "https://cdn.example.org/images/banner.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, vendor := IsTracker(tc.url)
			if got != tc.want || vendor != tc.wantVendor {
				t.Fatalf("IsTracker(%q) = (%v, %q), want (%v, %q)", tc.url, got, vendor, tc.want, tc.wantVendor)
			}
		})
	}
}

func TestIsTinyOrHidden(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]string
		want  bool
	}{
		{name: "1x1 image", attrs: map[string]string{"width": "1", "height": "1"}, want: true},
		{name: "width threshold", attrs: map[string]string{"width": "2px"}, want: true},
		{name: "display none", attrs: map[string]string{"style": "color: red; display: none"}, want: true},
		{name: "visibility hidden", attrs: map[string]string{"style": "visibility: hidden"}, want: true},
		{name: "zero opacity", attrs: map[string]string{"style": "opacity: 0"}, want: true},
		{name: "tiny CSS image", attrs: map[string]string{"style": "height: 1px !important"}, want: true},
		{name: "hidden important", attrs: map[string]string{"style": "DISPLAY: NONE ! important"}, want: true},
		{name: "percentage is not pixels", attrs: map[string]string{"width": "1%"}},
		{name: "above threshold", attrs: map[string]string{"width": "2.1"}},
		{name: "missing size", attrs: map[string]string{}},
		{name: "ordinary image", attrs: map[string]string{"width": "320", "height": "180", "style": "display:block; opacity:1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTinyOrHidden(tc.attrs); got != tc.want {
				t.Fatalf("IsTinyOrHidden(%v) = %v, want %v", tc.attrs, got, tc.want)
			}
		})
	}
}
