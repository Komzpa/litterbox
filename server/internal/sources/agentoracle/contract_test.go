package agentoracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/sources/agents"
)

func TestReadRootsExposesOnlySubstantiveAgentResults(t *testing.T) {
	root := t.TempDir()
	ompRoot := filepath.Join(root, "omp")
	codexRoot := filepath.Join(root, "codex")
	writeSession(t, ompRoot, "probe.jsonl", `{"type":"session","id":"omp-probe"}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Respond with exact channel confirmation"}]}}
{"type":"message","message":{"role":"assistant","stopReason":"endTurn","content":[{"type":"text","text":"CHANNEL OK"}]}}`)
	writeSession(t, ompRoot, "intermediate.jsonl", `{"type":"session","id":"omp-intermediate"}
{"type":"message","message":{"role":"assistant","stopReason":"endTurn","content":[{"type":"text","text":"still waiting for the task"}]}}`)
	writeSession(t, ompRoot, "result.jsonl", `{"type":"session","id":"omp-result","title":"Evidence review"}
{"type":"message","message":{"role":"assistant","stopReason":"endTurn","content":[{"type":"text","text":"Found three independently verified findings."}]}}`)
	writeSession(t, codexRoot, "ack.jsonl", `{"type":"session_meta","payload":{"id":"codex-ack"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Respond with exact channel confirmation"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"CHANNEL OK"}]}}
{"type":"event_msg","payload":{"type":"turn_complete"}}`)
	writeSession(t, codexRoot, "intermediate.jsonl", `{"type":"session_meta","payload":{"id":"codex-intermediate"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"stopped, owned by another run"}]}}
{"type":"event_msg","payload":{"type":"task_complete"}}`)
	writeSession(t, codexRoot, "result.jsonl", `{"type":"session_meta","payload":{"id":"codex-result"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"The independent run confirmed the same three findings."}]}}
{"type":"event_msg","payload":{"type":"turn_complete"}}`)

	cards, err := agents.ReadRoots(ompRoot, codexRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d agent cards, want only the two real results: %#v", len(cards), cards)
	}
	got := make(map[string]agents.Card, len(cards))
	for _, card := range cards {
		got[card.ExternalID] = card
	}
	for _, id := range []string{"omp:omp-result", "codex:codex-result"} {
		card, ok := got[id]
		if !ok {
			t.Fatalf("missing substantive result %q (got %#v)", id, got)
		}
		if strings.EqualFold(strings.TrimSpace(card.Title), strings.TrimSpace(card.Summary)) {
			t.Fatalf("title and summary duplicate each other for %q: %#v", id, card)
		}
		if card.Summary == "" {
			t.Fatalf("substantive result %q has no summary", id)
		}
	}
	for _, id := range []string{"omp:omp-probe", "omp:omp-intermediate", "codex:codex-ack", "codex:codex-intermediate"} {
		if _, ok := got[id]; ok {
			t.Errorf("non-result session %q was emitted: %#v", id, got[id])
		}
	}
}

func writeSession(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
