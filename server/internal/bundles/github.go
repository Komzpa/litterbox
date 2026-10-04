package bundles

// Deterministic GitHub mail routing: key by repository and who must act.
//
// Rationale: every GitHub notification shares the sender address
// notifications@github.com, so sender-keyed bundling lumps hundreds of
// unrelated threads into one bundle. The subject line already carries
// [owner/repo] and the body carries the machine-readable reason line
// "You are receiving this because <reason>.", so GitHub mail is routed
// without embeddings: mentions and human replies on the owner's own threads
// stay standalone cards, bot/CI traffic on his threads forms an
// agents-handle-this bundle per repo, subscription-only traffic forms a quiet
// bundle per repo, and remaining human discussion forms a discussion bundle
// per repo. Security advisories (no reason line) key by org.

import (
	"net/mail"
	"regexp"
	"strings"
)

// BundleSenderKey is the take-out exclusion identity for a sender. GitHub
// mail shares one address across all actors, so address-keyed exclusions
// would take every GitHub actor out of a bundle at once; GitHub mail is
// keyed by actor display name instead. Non-GitHub mail keeps SenderKey.
func BundleSenderKey(sender string) string {
	if IsGitHubMail(sender) {
		if a, err := mail.ParseAddress(strings.TrimSpace(sender)); err == nil {
			if name := strings.ToLower(strings.TrimSpace(a.Name)); name != "" {
				return "github-actor:" + name
			}
		}
		return "github-actor:" + strings.ToLower(strings.TrimSpace(sender))
	}
	return SenderKey(sender)
}
// IsGitHubMail reports whether the card sender is GitHub notifications.
// All GitHub notification mail shares one address; the actor sits in the
// display name, so the address alone must never be used as a bundle key.
func IsGitHubMail(sender string) bool {
	return SenderKey(sender) == "notifications@github.com"
}

var (
	githubRepoRe = regexp.MustCompile(`\[([^/\[\]\s]+)/([^\[\]/#\s]+?)(?:#\d+)?\]`)
	githubOrgRe  = regexp.MustCompile(`\[([^\[\]/\s]+)\]`)
	githubRefRe  = regexp.MustCompile(`\(([^/\s()]+)/([^/\s()#]+)#\d+\)`)
)

// ParseGitHubRepo extracts owner/repo from the subject "[owner/repo]" with a
// body "(owner/repo#N)" fallback. It returns repo as "owner/repo" and org as
// the owner part. Org-only advisory subjects ("[konturio] A security
// advisory ...") yield repo="" and org="konturio".
func ParseGitHubRepo(subject, text string) (repo, org string) {
	if m := githubRepoRe.FindStringSubmatch(subject); m != nil {
		repo = m[1] + "/" + strings.TrimSuffix(m[2], "]")
		org = m[1]
		return repo, org
	}
	if m := githubRefRe.FindStringSubmatch(text); m != nil {
		repo = m[1] + "/" + m[2]
		org = m[1]
		return repo, org
	}
	if m := githubOrgRe.FindStringSubmatch(subject); m != nil {
		org = m[1]
	}
	return "", org
}

// GitHubReason maps the "You are receiving this because <reason>." body line
// to a short reason: "mentioned", "authored", "commented", "subscribed", or
// "" when the line is absent (security advisories, state events).
func GitHubReason(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "you were mentioned"):
		return "mentioned"
	case strings.Contains(lower, "you authored the thread"):
		return "authored"
	case strings.Contains(lower, "you commented"):
		return "commented"
	case strings.Contains(lower, "you are subscribed"):
		return "subscribed"
	default:
		return ""
	}
}

var githubKnownBots = []string{
	"chatgpt-codex-connector",
	"coderabbitai",
	"github-actions",
	"copilot",
	"pre-commit-ci",
	"detail-app",
	"robo.omp",
	"linear",
}

// IsGitHubBot reports whether the sender display name is a bot or CI actor:
// a "[bot]" suffix or a known review/CI bot name.
func IsGitHubBot(sender string) bool {
	lower := strings.ToLower(sender)
	if strings.Contains(lower, "[bot]") {
		return true
	}
	for _, b := range githubKnownBots {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

// IsGitHubAdvisory reports security-advisory mail: no reason line plus
// advisory markers in subject or body.
func IsGitHubAdvisory(subject, text string) bool {
	if GitHubReason(text) != "" {
		return false
	}
	lower := strings.ToLower(subject + "\n" + text)
	return strings.Contains(lower, "security advisory") ||
		strings.Contains(lower, "security vulnerabilit") ||
		strings.Contains(lower, "view all alerts") ||
		strings.Contains(lower, "dependabot")
}

// StripGitHubFooter removes provider boilerplate before any embedding: the
// "Reply to this email directly..." footer block and the "You are receiving
// this because..." reason line. Routing reads the reason line from the raw
// text first; this keeps boilerplate out of the embedded vector.
func StripGitHubFooter(text string) string {
	if i := strings.Index(text, "Reply to this email directly"); i >= 0 {
		end := strings.LastIndex(text[:i], "\n-- \n")
		if end < 0 {
			end = strings.LastIndex(text[:i], "\n--\n")
		}
		if end >= 0 {
			text = text[:end]
		} else {
			text = strings.TrimSpace(text[:i])
		}
	}
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(ln)), "you are receiving this because") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// GitHubRoute is the deterministic placement for one GitHub mail card.
// Standalone means its own card, never inside a bundle.
type GitHubRoute struct {
	Key        string
	Title      string
	Standalone bool
}

// RouteGitHubMail keys GitHub mail by repo and who must act. It returns
// ok=false for non-GitHub mail. GitHub mail with no parseable repo or org
// falls into a "github:misc" bundle; it is never keyed by sender alone.
func RouteGitHubMail(sender, subject, text string) (route GitHubRoute, ok bool) {
	if !IsGitHubMail(sender) {
		return GitHubRoute{}, false
	}
	repo, org := ParseGitHubRepo(subject, text)
	reason := GitHubReason(text)
	bot := IsGitHubBot(sender)

	if reason == "mentioned" || (reason == "authored" && !bot) {
		return GitHubRoute{Standalone: true}, true
	}

	scope := repo
	if scope == "" {
		scope = org
	}
	if IsGitHubAdvisory(subject, text) {
		if org != "" {
			scope = org
		}
		if scope == "" {
			scope = "unknown"
		}
		return GitHubRoute{Key: "github:" + scope, Title: scope + " · security advisories"}, true
	}
	if scope == "" {
		return GitHubRoute{Key: "github:misc", Title: "GitHub · misc"}, true
	}
	switch {
	case (reason == "authored" || reason == "commented") && bot:
		return GitHubRoute{Key: "github-agents:" + scope, Title: scope + " · bot reviews"}, true
	case reason == "subscribed":
		return GitHubRoute{Key: "github-fyi:" + scope, Title: scope + " · subscribed"}, true
	default:
		return GitHubRoute{Key: "github:" + scope, Title: scope + " · discussion"}, true
	}
}
