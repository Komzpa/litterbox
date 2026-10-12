package cards

import (
	"html"
	"strings"
	"unicode"
)

// cleanSnippet derives display text without changing the stored message body.
// Recomputing it on every card sync also repairs previews of legacy messages.
func cleanSnippet(text, sender string) string {
	text = snippetText(text)
	sender = strings.Join(strings.Fields(snippetText(sender)), " ")
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" || (sender != "" && strings.EqualFold(line, sender)) {
			continue
		}
		rule := true
		for _, r := range line {
			if !unicode.IsSpace(r) && !unicode.IsPunct(r) && !strings.ContainsRune("+|=~", r) {
				rule = false
				break
			}
		}
		if !rule {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

func snippetText(text string) string {
	// Decode to a fixed point so re-cleaning an already cleaned preview is
	// idempotent, including old text with double-escaped entities.
	for {
		decoded := html.UnescapeString(text)
		if decoded == text {
			break
		}
		text = decoded
	}
	return strings.Map(func(r rune) rune {
		if r == '\u2007' || r == '\u034f' || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
}
