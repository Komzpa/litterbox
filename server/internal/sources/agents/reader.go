package agents

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const SummaryLimit = 500

type Card struct {
	ID         string    `json:"id,omitempty"`
	Source     string    `json:"source"`
	ExternalID string    `json:"external_id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	SortAt     time.Time `json:"sort_at"`
	State      string    `json:"state"`
}

func ReadRoots(ompRoot, codexRoot string) ([]Card, error) {
	var cards []Card
	for _, source := range []struct{ name, root string }{{"omp", ompRoot}, {"codex", codexRoot}} {
		if source.root == "" {
			continue
		}
		var paths []string
		err := filepath.WalkDir(source.root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			var card Card
			var ok bool
			if source.name == "omp" {
				card, ok, err = readOMP(file)
			} else {
				card, ok, err = readCodex(file)
			}
			closeErr := file.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if ok {
				cards = append(cards, card)
			}
		}
	}
	return cards, nil
}

func readOMP(r io.Reader) (Card, bool, error) {
	var card Card
	var summary, userPrompt string
	var sortAt time.Time
	ready, childSession := false, false
	err := scanLines(r, func(line []byte) {
		var row struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			Title     string `json:"title"`
			Message   struct {
				Role        string          `json:"role"`
				Content     json.RawMessage `json:"content"`
				StopReason  string          `json:"stopReason"`
				CompletedAt int64           `json:"completedAt"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &row) != nil {
			return
		}
		switch row.Type {
		case "session":
			if isChildMetadata(line) {
				childSession = true
			}
			if row.ID != "" {
				card.ExternalID = "omp:" + row.ID
			}
			if row.Title != "" {
				card.Title = row.Title
			}
			card.SortAt, _ = time.Parse(time.RFC3339Nano, row.Timestamp)
		case "title", "title_change":
			if row.Title != "" {
				card.Title = row.Title
			}
		case "message":
			text := contentText(row.Message.Content)
			if row.Message.Role == "user" {
				ready, userPrompt = false, text
				return
			}
			if row.Message.Role != "assistant" {
				return
			}
			ready = row.Message.StopReason == "stop" || row.Message.StopReason == "endTurn"
			summary = text
			if text == "" {
				return
			}
			if row.Message.CompletedAt > 0 {
				sortAt = time.UnixMilli(row.Message.CompletedAt).UTC()
			} else {
				sortAt, _ = time.Parse(time.RFC3339Nano, row.Timestamp)
			}
		}
	})
	if err != nil {
		return Card{}, false, err
	}
	if card.ExternalID == "" || summary == "" || !ready || childSession || isNoiseResult(summary, userPrompt) {
		return Card{}, false, nil
	}
	summary = cleanSummary(summary)
	if summary == "" {
		return Card{}, false, nil
	}
	card.Source, card.Summary, card.State = "agent", truncate(summary, SummaryLimit), "open"
	if card.Title == "" {
		card.Title = truncate(strings.SplitN(summary, "\n", 2)[0], 80)
	}
	card.Title = sourceTitle("OMP", card.Title, summary)
	if strings.EqualFold(card.Summary, card.Title) {
		card.Summary = ""
	}
	if !sortAt.IsZero() {
		card.SortAt = sortAt
	}
	return card, true, nil
}

func readCodex(r io.Reader) (Card, bool, error) {
	var card Card
	var summary, userPrompt string
	var summaryAt time.Time
	completed, childSession := false, false
	err := scanLines(r, func(line []byte) {
		if isChildMetadata(line) {
			childSession = true
		}
		var row struct {
			Type      string          `json:"type"`
			Timestamp string          `json:"timestamp"`
			Payload   json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(line, &row) != nil {
			return
		}
		at, _ := time.Parse(time.RFC3339Nano, row.Timestamp)
		switch row.Type {
		case "session_meta":
			if isChildMetadata(line) {
				childSession = true
			}
			var meta struct {
				ID        string `json:"id"`
				SessionID string `json:"session_id"`
			}
			_ = json.Unmarshal(row.Payload, &meta)
			card.ExternalID = meta.ID
			if card.ExternalID == "" {
				card.ExternalID = meta.SessionID
			}
			card.SortAt = at
		case "response_item":
			var item struct {
				Type    string          `json:"type"`
				Role    string          `json:"role"`
				Phase   string          `json:"phase"`
				Content json.RawMessage `json:"content"`
			}
			_ = json.Unmarshal(row.Payload, &item)
			if item.Type == "message" && item.Role == "user" {
				completed = false
				summary = ""
				userPrompt = contentText(item.Content)
			}
			if item.Type == "message" && item.Role == "assistant" && item.Phase != "commentary" {
				summary, summaryAt = contentText(item.Content), at
				completed = item.Phase == "final"
			} else if item.Type == "custom_tool_call" || item.Type == "function_call" {
				completed = false
			}
		case "event_msg":
			var event struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(row.Payload, &event)
			if event.Type == "task_started" {
				summary = ""
				completed = false
			}
			if event.Type == "task_complete" || event.Type == "turn_complete" {
				completed = true
			}
		}
	})
	if err != nil {
		return Card{}, false, err
	}
	if card.ExternalID == "" || summary == "" || !completed || childSession || isNoiseResult(summary, userPrompt) {
		return Card{}, false, nil
	}
	summary = cleanSummary(summary)
	if summary == "" {
		return Card{}, false, nil
	}
	card.Source, card.Summary, card.State = "agent", truncate(summary, SummaryLimit), "open"
	if card.Title == "" {
		card.Title = truncate(strings.SplitN(summary, "\n", 2)[0], 80)
	}
	card.Title = sourceTitle("Codex", card.Title, summary)
	if strings.EqualFold(card.Summary, card.Title) {
		card.Summary = ""
	}
	card.ExternalID = "codex:" + card.ExternalID
	if !summaryAt.IsZero() {
		card.SortAt = summaryAt
	}
	return card, true, nil
}

func isChildMetadata(line []byte) bool {
	var value any
	if json.Unmarshal(line, &value) != nil {
		return false
	}
	return hasChildMarker(value)
}
func hasChildMarker(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			if (strings.HasPrefix(normalized, "parent") && strings.HasSuffix(normalized, "id")) || strings.Contains(normalized, "subagent") || normalized == "childsession" || normalized == "ischild" {
				if child != nil && child != "" && child != false {
					return true
				}
			}
			if normalized == "source" || strings.Contains(normalized, "agenttype") || normalized == "role" {
				if text, ok := child.(string); ok && (strings.EqualFold(text, "subagent") || strings.EqualFold(text, "child")) {
					return true
				}
			}
			if hasChildMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasChildMarker(child) {
				return true
			}
		}
	}
	return false
}
func isNoiseResult(summary, prompt string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(summary), " "))
	p := strings.ToLower(prompt)
	if strings.Contains(p, "channel") && (strings.Contains(p, "confirm") || strings.Contains(p, "exact")) && len(normalized) <= 120 {
		return true
	}
	if len(normalized) <= 80 {
		for _, ack := range []string{"channel ok", "channel confirmed", "confirmed channel", "channel confirmation", "respond with exact channel confirmation", "ack", "acknowledged", "ok", "okay"} {
			if normalized == ack {
				return true
			}
		}
	}
	for _, status := range []string{"still waiting", "stopped, owned by", "stopped owned by", "waiting for", "still running", "in progress", "working on it"} {
		if strings.HasPrefix(normalized, status) {
			return true
		}
	}
	return false
}
func cleanSummary(text string) string {
	text = stripXMLBlocks(text)
	var b strings.Builder
	inFence, inJSONBlock := false, false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\u0060\u0060\u0060") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if inJSONBlock {
			if strings.ContainsAny(line, "}]") {
				inJSONBlock = false
			}
			continue
		}
		if startsJSONBlock(line) {
			inJSONBlock = !strings.ContainsAny(line[1:], "}]")
			continue
		}
		if isJSONText(line) || (strings.Contains(line, "|") && strings.Count(line, "|") >= 2) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(line)
	}
	text = b.String()
	for {
		start := strings.Index(text, "[")
		if start < 0 {
			break
		}
		mid := strings.Index(text[start+1:], "](")
		if mid < 0 {
			break
		}
		mid += start + 1
		end := strings.IndexByte(text[mid+2:], ')')
		if end < 0 {
			break
		}
		end += mid + 2
		labelStart := start
		if start > 0 && text[start-1] == '!' {
			labelStart--
		}
		text = text[:labelStart] + text[start+1:mid] + text[end+1:]
	}
	text = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "").Replace(text)
	return strings.Join(strings.Fields(text), " ")
}
func stripXMLBlocks(text string) string {
	for {
		start := strings.IndexByte(text, '<')
		if start < 0 {
			return text
		}
		gt := strings.IndexByte(text[start:], '>')
		if gt < 0 {
			return text[:start]
		}
		gt += start
		tag := strings.TrimSpace(text[start+1 : gt])
		if tag == "" || strings.HasPrefix(tag, "/") || strings.HasPrefix(tag, "!") || strings.HasPrefix(tag, "?") {
			text = text[:start] + text[gt+1:]
			continue
		}
		nameEnd := strings.IndexAny(tag, " />\t\r\n")
		name := tag
		if nameEnd >= 0 {
			name = tag[:nameEnd]
		}
		closeAt := strings.Index(strings.ToLower(text[gt+1:]), "</"+strings.ToLower(name))
		if closeAt >= 0 {
			closeAt += gt + 1
			closeEnd := strings.IndexByte(text[closeAt:], '>')
			if closeEnd >= 0 {
				text = text[:start] + text[closeAt+closeEnd+1:]
				continue
			}
		}
		text = text[:start] + text[gt+1:]
	}
}
func startsJSONBlock(text string) bool {
	if strings.HasPrefix(text, "{") {
		return true
	}
	if len(text) < 2 || text[0] != '[' {
		return false
	}
	next := text[1]
	return next == ' ' || next == '\t' || next == '{' || next == '"' || next == ']' || next == '-' || next >= '0' && next <= '9' || next == 't' || next == 'f' || next == 'n'
}

func isJSONText(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	if text[0] == '{' || (text[0] == '"' && strings.Contains(text, "\":")) {
		return true
	}
	return len(text) >= 2 && text[0] == '[' && text[len(text)-1] == ']' && json.Valid([]byte(text))
}

func sourceTitle(source, title, summary string) string {
	title = cleanSummary(title)
	if strings.HasPrefix(strings.TrimSpace(title), "{") || strings.HasPrefix(strings.TrimSpace(title), "[") {
		title = ""
	}
	if strings.HasPrefix(strings.ToLower(title), strings.ToLower(source)+" ") {
		title = strings.TrimSpace(title[len(source):])
	}
	if title == "" {
		title = truncate(strings.SplitN(summary, "\n", 2)[0], 80)
	}
	prefix := strings.ToLower(source) + ":"
	if !strings.HasPrefix(strings.ToLower(title), prefix) {
		title = source + ": " + title
	}
	body := strings.TrimSpace(title[len(source)+1:])
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(summary)), strings.ToLower(body)) {
		title = source + " result"
	}
	return truncate(title, 100)
}

func scanLines(r io.Reader, visit func([]byte)) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for s.Scan() {
		visit(s.Bytes())
	}
	return s.Err()
}

func contentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" && (p.Type == "text" || p.Type == "output_text" || p.Type == "input_text") {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(p.Text)
			}
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}

func truncate(s string, limit int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:limit])) + "…"
}
