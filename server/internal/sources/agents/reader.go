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
	var summary string
	var sortAt time.Time
	ready := false
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
			if row.ID != "" {
				card.ExternalID = "omp:" + row.ID
			}
			if card.Title == "" {
				card.Title = row.Title
			}
			card.SortAt, _ = time.Parse(time.RFC3339Nano, row.Timestamp)
		case "title":
			if row.Title != "" {
				card.Title = row.Title
			}
		case "title_change":
			if row.Title != "" {
				card.Title = row.Title
			}
		case "message":
			if row.Message.Role == "user" {
				ready = false
			}
			if row.Message.Role != "assistant" {
				return
			}
			ready = row.Message.StopReason == "stop" || row.Message.StopReason == "endTurn"
			text := contentText(row.Message.Content)
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
	if card.ExternalID == "" || summary == "" || !ready {
		return Card{}, false, nil
	}
	card.Source, card.Summary, card.State = "agent", truncate(summary, SummaryLimit), "open"
	if card.Title == "" {
		card.Title = "OMP: " + truncate(strings.SplitN(summary, "\n", 2)[0], 120)
	}
	if !sortAt.IsZero() {
		card.SortAt = sortAt
	}
	return card, true, nil
}

func readCodex(r io.Reader) (Card, bool, error) {
	var card Card
	var summary string
	var summaryAt time.Time
	completed := false
	err := scanLines(r, func(line []byte) {
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
	if card.ExternalID == "" || summary == "" || !completed {
		return Card{}, false, nil
	}
	card.Source, card.Summary, card.State = "agent", truncate(summary, SummaryLimit), "open"
	if card.Title == "" {
		card.Title = "Codex: " + truncate(strings.SplitN(summary, "\n", 2)[0], 120)
	}
	card.ExternalID = "codex:" + card.ExternalID
	if !summaryAt.IsZero() {
		card.SortAt = summaryAt
	}
	return card, true, nil
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
