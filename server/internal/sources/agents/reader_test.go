package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadRootsFinishedOnly(t *testing.T) {
	root := t.TempDir()
	omp := filepath.Join(root, "omp")
	codex := filepath.Join(root, "codex")
	for dir, files := range map[string][]string{omp: {"omp-session.jsonl", "omp-in-progress.jsonl"}, codex: {"codex-session.jsonl", "codex-in-progress.jsonl"}} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	cards, err := ReadRoots(omp, codex)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2: %#v", len(cards), cards)
	}
	got := map[string]Card{}
	for _, c := range cards {
		got[c.ExternalID] = c
	}
	for _, id := range []string{"omp:omp-fixture-1", "codex:codex-fixture-1"} {
		c, ok := got[id]
		if !ok {
			t.Fatalf("missing %s", id)
		}
		if c.Source != "agent" || c.State != "open" || c.Summary == "" || c.SortAt.IsZero() {
			t.Fatalf("bad card: %#v", c)
		}
	}
}

func TestSummaryTruncatesAtRuneBoundary(t *testing.T) {
	got := truncate("ééé", 2)
	if got != "éé…" {
		t.Fatalf("got %q", got)
	}
}

func TestOMPInFlightAndInterruptedTurnsDoNotEmitOldResult(t *testing.T) {
	fixture, err := os.ReadFile("testdata/omp-session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Synthetic hidden reasoning."},{"type":"toolCall","name":"fixture"}],"stopReason":"toolUse"}}`,
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Interrupted synthetic result."}],"stopReason":"aborted"}}`,
		`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Synthetic follow-up."}]}}`,
	} {
		_, ok, err := readOMP(strings.NewReader(string(fixture) + line + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatalf("in-flight turn emitted result for %s", line)
		}
	}
}

func TestFinishedCardProjection(t *testing.T) {
	file, err := os.Open("testdata/omp-session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	card, ok, err := readOMP(file)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("finished session absent")
	}
	if card.Title != "OMP: fixture result" || card.Summary != "Synthetic OMP result." || !card.SortAt.Equal(time.Date(2026, 9, 27, 12, 1, 0, 0, time.UTC)) {
		t.Fatalf("projection=%#v", card)
	}
}

func TestCodexCompletionDoesNotResurrectOldTurn(t *testing.T) {
	fixture, err := os.ReadFile("testdata/codex-in-progress.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := readCodex(strings.NewReader(string(fixture) + `{"type":"event_msg","payload":{"type":"task_complete"}}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("completion without a new assistant result resurrected prior turn")
	}
}

func TestCodexLegacyCompletedAndAwaitingUser(t *testing.T) {
	body := `{"type":"session_meta","payload":{"id":"legacy"}}
{"timestamp":"2026-09-27T12:01:00Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Synthetic legacy result."}]}}
{"type":"event_msg","payload":{"type":"task_complete"}}
`
	card, ok, err := readCodex(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || card.Summary != "Synthetic legacy result." {
		t.Fatalf("bad legacy result: %#v %v", card, ok)
	}
}

func TestAgentResultNoiseFilters(t *testing.T) {
	cases := []struct{ name, prompt, answer string }{
		{"ack", "", "CHANNEL OK"},
		{"requested channel ack", "Respond with exact channel confirmation", "CHANNEL OK"},
		{"intermediate waiting", "", "still waiting"},
		{"owned status", "", "stopped, owned by another run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"type":"session","id":"synthetic"}` + "\n" + `{"type":"message","message":{"role":"user","content":[{"type":"text","text":"` + tc.prompt + `"}]}}` + "\n" + `{"type":"message","message":{"role":"assistant","stopReason":"endTurn","content":[{"type":"text","text":"` + tc.answer + `"}]}}`
			_, ok, err := readOMP(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Fatal("noise result emitted")
			}
		})
	}
}

func TestAgentResultCleanupAndSourceTitle(t *testing.T) {
	raw := "Summary <citation_entries>private citation</citation_entries> keep\n| A | B |\n|---|---|\n```json\n{\"x\":1}\n```\n[visible label](https://example.invalid)"
	got := cleanSummary(raw)
	if got != "Summary keep visible label" {
		t.Fatalf("cleaned summary=%q", got)
	}
	title := sourceTitle("Codex", "Useful headline", got)
	if title != "Codex: Useful headline" {
		t.Fatalf("title=%q", title)
	}
	if strings.EqualFold(got, title) {
		t.Fatalf("summary duplicates title: %q", got)
	}
	if copied := sourceTitle("OMP", "Same headline", "Same headline with details"); copied != "OMP result" {
		t.Fatalf("summary-prefix title was not replaced: %q", copied)
	}
	if cleanSummary("<rollout_ids>secret</rollout_ids>") != "" {
		t.Fatal("XML block content survived")
	}
	if got := cleanSummary("**plain** and `inline`"); got != "plain and inline" {
		t.Fatalf("markdown text=%q", got)
	}
	if cleanSummary(`{"private":"synthetic"}`) != "" {
		t.Fatal("JSON content survived")
	}
	if cleanSummary(`{"unfinished":`) != "" {
		t.Fatal("incomplete JSON content survived")
	}
	if cleanSummary("{\n  \"report\": \"synthetic\"\n}") != "" {
		t.Fatal("multiline JSON content survived")
	}
	if got := sourceTitle("Codex", `{"report":"synthetic"}`, "Narrative result."); got != "Codex result" {
		t.Fatalf("JSON title=%q", got)
	}
}

func TestCodexChildSessionIsDropped(t *testing.T) {
	if !hasChildMarker(map[string]any{"is_subagent": true}) {
		t.Fatal("subagent flag was not recognized")
	}
	body := `{"type":"session_meta","payload":{"id":"child","parent_session_id":"parent"}}` + "\n" + `{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"final","content":[{"type":"output_text","text":"Synthetic child result."}]}}` + "\n" + `{"type":"event_msg","payload":{"type":"turn_complete"}}`
	_, ok, err := readCodex(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("child session emitted a card")
	}
}

func TestOMPChildSessionIsDropped(t *testing.T) {
	body := `{"type":"session","id":"synthetic","parentSessionId":"parent"}` + "\n" + `{"type":"message","message":{"role":"assistant","stopReason":"endTurn","content":[{"type":"text","text":"Synthetic child result."}]}}`
	_, ok, err := readOMP(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("child session emitted a card")
	}
}
