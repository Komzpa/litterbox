package cards

import (
	"html"
	"strings"
	"unicode"
)

// mailSnippet selects body text, not a title, signature or quoted reply. The
// newest full message stays untouched; only the card projection is shortened.
func mailSnippet(body, title, sender string) string {
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "--" || strings.HasPrefix(line, ">") {
			break
		}
		line = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if unicode.Is(unicode.Cf, r) || r == '\u034f' {
				return -1
			}
			return r
		}, html.UnescapeString(line))), " ")
		if sender != "" && strings.EqualFold(line, sender) {
			// A sender repeat begins the signature when there is no body yet.
			break
		}
		if line == "" || strings.EqualFold(snippetSubject(line), snippetSubject(title)) || snippetTableRow(line) || !snippetHasWord(line) {
			continue
		}
		return line
	}
	return ""
}

func snippetSubject(subject string) string {
	for {
		subject = strings.TrimSpace(subject)
		lower := strings.ToLower(subject)
		switch {
		case strings.HasPrefix(lower, "re:") || strings.HasPrefix(lower, "fw:"):
			subject = subject[3:]
		case strings.HasPrefix(lower, "fwd:"):
			subject = subject[4:]
		case strings.HasPrefix(subject, "[") && strings.Contains(subject, "]"):
			subject = subject[strings.IndexByte(subject, ']')+1:]
		default:
			return subject
		}
	}
}

// Ticket table metadata has label:value cells separated by vertical bars.
// Prose containing a pipe is not automatically a metadata row.
func snippetTableRow(line string) bool {
	if !strings.ContainsRune(line, '|') {
		return false
	}
	for cell := range strings.SplitSeq(line, "|") {
		cell = strings.TrimSpace(cell)
		if cell != "" && !strings.ContainsRune(cell, ':') {
			return false
		}
	}
	return true
}

// Require a word or a number, not isolated glyphs. Unicode letters retain
// non-English text and short replies such as "Hi", "OK", and "Да".
func snippetHasWord(line string) bool {
	letters, digits := 0, 0
	for _, r := range line {
		if unicode.IsLetter(r) {
			letters++
		} else {
			letters = 0
		}
		if unicode.IsDigit(r) {
			digits++
		} else {
			digits = 0
		}
		if letters >= 2 || digits >= 2 {
			return true
		}
	}
	return false
}
