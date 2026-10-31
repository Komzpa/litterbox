package bundles

import "testing"

func TestRouteBitbucketPipelineBySenderAndRepository(t *testing.T) {
	// Captured from the stored Gmail header/body shape: From is preserved as
	// "Bitbucket <notifications-noreply@bitbucket.org>" and the subject carries
	// the repo in parentheses, with per-pipeline branch and SHA changing.
	const sender = "Bitbucket <notifications-noreply@bitbucket.org>"
	first, ok := RouteBitbucketMail(sender,
		"[Bitbucket] Pipeline for r4-cooling-scene-2026-10-17 failed on 77ee508 (sunbim/xmova-app)",
		"Failure reported for commit https://bitbucket.org/sunbim/xmova-app/commits/77ee508")
	if !ok {
		t.Fatal("real Bitbucket pipeline notification shape was not routed")
	}
	second, ok := RouteBitbucketMail(sender,
		"[Bitbucket] Pipeline for r4-cashflow-2026-10-17 failed on d81cf47 (sunbim/xmova-app)",
		"Failure reported for commit https://bitbucket.org/sunbim/xmova-app/commits/d81cf47")
	if !ok || first.Key != second.Key {
		t.Fatalf("same-repo pipeline notifications got different routes: first=%+v second=%+v ok=%v", first, second, ok)
	}

	otherRepo, ok := RouteBitbucketMail(sender,
		"[Bitbucket] Pipeline for r4-cooling-scene-2026-10-17 failed on 77ee508 (sunbim/another-app)",
		"Failure reported for commit https://bitbucket.org/sunbim/another-app/commits/77ee508")
	if !ok || otherRepo.Key == first.Key {
		t.Fatalf("different repository merged: first=%+v other=%+v ok=%v", first, otherRepo, ok)
	}
	otherSender, ok := RouteBitbucketMail("Bitbucket <ci@bitbucket.org>",
		"[Bitbucket] Pipeline for r4-cooling-scene-2026-10-17 failed on 77ee508 (sunbim/xmova-app)",
		"Failure reported for commit https://bitbucket.org/sunbim/xmova-app/commits/77ee508")
	if ok {
		t.Fatalf("different sender was incorrectly routed: %+v", otherSender)
	}
}

func TestRouteBitbucketFallsBackToRepositoryURL(t *testing.T) {
	route, ok := RouteBitbucketMail("Bitbucket <notifications-noreply@bitbucket.org>",
		"[Bitbucket] Pipeline failed", "Failure: https://bitbucket.org/sunbim/xmova-app/commits/abc123")
	if !ok || route.Key != "bitbucket:notifications-noreply@bitbucket.org:sunbim/xmova-app" {
		t.Fatalf("URL repo route = %+v, ok=%v", route, ok)
	}
}
