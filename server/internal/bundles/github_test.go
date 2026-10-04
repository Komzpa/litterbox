package bundles

// Oracle for the GitHub bundle split: real-shaped fixtures asserting keys,
// standalone cards and human titles. Pure routing tests run everywhere; the
// Cluster test needs Postgres (pg_virtualenv, BUNDLE_TEST_POSTGRES=1).

import (
	"strings"
	"testing"
)

const (
	mentionBody = `Hey @darafei, could you look at this?

--
Reply to this email directly or view it on GitHub:
https://github.com/koverstreet/ktest/pull/12#issuecomment-1
You are receiving this because you were mentioned.`

	humanOnHisPRBody = `Looks good, but please rebase onto main first.

--
Reply to this email directly or view it on GitHub:
https://github.com/Soju06/codex-lb/pull/2093#issuecomment-2
You are receiving this because you authored the thread.`

	codexBotBody = ` I've reviewed the changes and left inline comments.

--
Reply to this email directly or view it on GitHub:
https://github.com/Komzpa/oh-my-pi/pull/39#issuecomment-3
You are receiving this because you authored the thread.`

	subscribedBody = `Same issue here on the latest release.

--
Reply to this email directly or view it on GitHub:
https://github.com/bkbilly/lnxlink/issues/44#issuecomment-4
You are receiving this because you are subscribed to this thread.`

	advisoryBody = `A security advisory on tar affects at least one of your repositories.

View all alerts:
https://github.com/organizations/konturio/dependabot`

	advisoryRepoBody = `Your repository has dependencies with security vulnerabilities.

View all alerts:
https://github.com/konturio/ui/security`
)

func TestRouteGitHubMentionIsStandalone(t *testing.T) {
	route, ok := RouteGitHubMail("Koverstreet <notifications@github.com>", "Re: [koverstreet/ktest] test journal flush SRCU liveness", mentionBody)
	if !ok {
		t.Fatal("mention not recognized as GitHub mail")
	}
	if !route.Standalone {
		t.Fatalf("mention must be standalone, got %+v", route)
	}
}

func TestRouteGitHubHumanReplyOnHisPRIsStandalone(t *testing.T) {
	route, ok := RouteGitHubMail("Maintainer <notifications@github.com>", "Re: [Soju06/codex-lb] fix(proxy) (PR #2093)", humanOnHisPRBody)
	if !ok {
		t.Fatal("human reply not recognized as GitHub mail")
	}
	if !route.Standalone {
		t.Fatalf("human reply on his PR must be standalone, got %+v", route)
	}
}

func TestRouteGitHubCodexBotReviewOnHisPR(t *testing.T) {
	route, ok := RouteGitHubMail(`"chatgpt-codex-connector[bot]" <notifications@github.com>`, "Re: [Komzpa/oh-my-pi] Requirements ledger (PR #39)", codexBotBody)
	if !ok {
		t.Fatal("bot review not recognized as GitHub mail")
	}
	if route.Standalone {
		t.Fatal("bot review must be bundled, not standalone")
	}
	if route.Key != "github-agents:Komzpa/oh-my-pi" {
		t.Fatalf("bot review key=%q", route.Key)
	}
	if route.Title != "Komzpa/oh-my-pi · bot reviews" {
		t.Fatalf("bot review title=%q", route.Title)
	}
}

func TestRouteGitHubSubscribedIssue(t *testing.T) {
	route, ok := RouteGitHubMail(`"github-actions[bot]" <notifications@github.com>`, "Re: [bkbilly/lnxlink] Door lock TY0A01 (#44)", subscribedBody)
	if !ok {
		t.Fatal("subscribed mail not recognized as GitHub mail")
	}
	if route.Standalone {
		t.Fatal("subscribed mail must be bundled, not standalone")
	}
	if route.Key != "github-fyi:bkbilly/lnxlink" {
		t.Fatalf("subscribed key=%q", route.Key)
	}
	if route.Title != "bkbilly/lnxlink · subscribed" {
		t.Fatalf("subscribed title=%q", route.Title)
	}
}

func TestRouteGitHubSecurityAdvisoryKeysByOrg(t *testing.T) {
	route, ok := RouteGitHubMail("GitHub <notifications@github.com>", "[konturio] A security advisory on tar affects at least one of your repositories", advisoryBody)
	if !ok {
		t.Fatal("advisory not recognized as GitHub mail")
	}
	if route.Standalone {
		t.Fatal("advisory must be bundled, not standalone")
	}
	if route.Key != "github:konturio" {
		t.Fatalf("advisory key=%q, want org key", route.Key)
	}
	if !strings.Contains(route.Title, "konturio") {
		t.Fatalf("advisory title=%q", route.Title)
	}
	if strings.HasPrefix(route.Key, "sender:") {
		t.Fatalf("advisory keyed by sender: %q", route.Key)
	}
}

func TestRouteGitHubRepoAdvisoryKeysByOrg(t *testing.T) {
	route, ok := RouteGitHubMail("GitHub <notifications@github.com>", "[konturio/ui] Your repository has dependencies with security vulnerabilities", advisoryRepoBody)
	if !ok {
		t.Fatal("repo advisory not recognized as GitHub mail")
	}
	if route.Key != "github:konturio" {
		t.Fatalf("repo advisory key=%q, want org key", route.Key)
	}
}

func TestRouteGitHubHumanDiscussion(t *testing.T) {
	body := `I can reproduce this on 6.14 as well.

--
Reply to this email directly or view it on GitHub:
https://github.com/koverstreet/bcachefs/issues/100#issuecomment-5
You are receiving this because you commented.`
	route, ok := RouteGitHubMail("Koverstreet <notifications@github.com>", "Re: [koverstreet/bcachefs] BLK_STS_INVAL on .raw VM disk", body)
	if !ok {
		t.Fatal("discussion not recognized as GitHub mail")
	}
	if route.Standalone {
		t.Fatal("upstream discussion must be bundled")
	}
	if route.Key != "github:koverstreet/bcachefs" {
		t.Fatalf("discussion key=%q", route.Key)
	}
	if route.Title != "koverstreet/bcachefs · discussion" {
		t.Fatalf("discussion title=%q", route.Title)
	}
}

func TestRouteGitHubNeverKeysBySender(t *testing.T) {
	fixtures := []struct{ sender, subject, body string }{
		{`"chatgpt-codex-connector[bot]" <notifications@github.com>`, "Re: [Komzpa/oh-my-pi] x", codexBotBody},
		{"Koverstreet <notifications@github.com>", "Re: [koverstreet/ktest] y", mentionBody},
		{"GitHub <notifications@github.com>", "[konturio] A security advisory on tar", advisoryBody},
		{`"github-actions[bot]" <notifications@github.com>`, "Re: [bkbilly/lnxlink] z", subscribedBody},
		{"Human <notifications@github.com>", "Re: [koverstreet/bcachefs] w", "plain comment, no reason line, no footer"},
	}
	for i, f := range fixtures {
		route, ok := RouteGitHubMail(f.sender, f.subject, f.body)
		if !ok {
			t.Fatalf("fixture %d not recognized as GitHub mail", i)
		}
		if route.Standalone {
			continue
		}
		if strings.HasPrefix(route.Key, "sender:") {
			t.Fatalf("fixture %d keyed by sender: %q", i, route.Key)
		}
		if strings.Contains(route.Title, "sender:") {
			t.Fatalf("fixture %d raw key as title: %q", i, route.Title)
		}
	}
}

func TestRouteGitHubIgnoresNonGitHubMail(t *testing.T) {
	if _, ok := RouteGitHubMail("Jane <jane@example.com>", "hello", "plain mail"); ok {
		t.Fatal("non-GitHub mail must not route")
	}
}

func TestStripGitHubFooter(t *testing.T) {
	stripped := StripGitHubFooter(mentionBody)
	if strings.Contains(stripped, "Reply to this email directly") {
		t.Fatalf("footer survives: %q", stripped)
	}
	if strings.Contains(stripped, "You are receiving this because") {
		t.Fatalf("reason line survives: %q", stripped)
	}
	if !strings.Contains(stripped, "could you look at this") {
		t.Fatalf("body lost: %q", stripped)
	}
}
