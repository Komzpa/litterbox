package cards

import (
	"testing"
	"time"
)

func TestSectionAt1500Tbilisi(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tbilisi")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 15, 0, 0, 0, loc)
	at := func(hour, minute int) *time.Time { v := time.Date(2026, 9, 28, hour, minute, 0, 0, loc); return &v }
	cards := []Card{
		{ID: "14", Source: "todo", Title: "early", At: at(14, 0), Timed: true, State: "open"},
		{ID: "17", Source: "todo", Title: "later", At: at(17, 30), Timed: true, State: "open"},
		{ID: "23", Source: "todo", Title: "sleep", At: at(23, 0), Timed: true, State: "open"},
		{ID: "u", Source: "todo", Title: "untimed", Timed: false, State: "open"},
	}
	got := Section(cards, now)
	if len(got.Now) != 2 || got.Now[0].ID != "14" || got.Now[1].ID != "u" {
		t.Fatalf("now=%+v", got.Now)
	}
	if len(got.Later) != 2 || got.Later[0].ID != "17" || got.Later[1].ID != "23" {
		t.Fatalf("later=%+v", got.Later)
	}
	if len(got.Missed) != 0 {
		t.Fatalf("missed=%+v", got.Missed)
	}
	// A newly started slot supersedes the previous slot; the previous item is missed.
	got = Section(append(cards, Card{ID: "1530", Source: "todo", Title: "started", At: at(14, 30), Timed: true, State: "open"}), now)
	if len(got.Now) != 2 || got.Now[0].ID != "1530" || len(got.Missed) != 1 || got.Missed[0].ID != "14" {
		t.Fatalf("slot transition now=%+v missed=%+v", got.Now, got.Missed)
	}
}

func TestSectionPinnedFirstWithinR30Sections(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tbilisi")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 15, 0, 0, 0, loc)
	at := func(hour int) *time.Time {
		v := time.Date(2026, 9, 28, hour, 0, 0, 0, loc)
		return &v
	}
	rank := func(n int64) *int64 { return &n }
	cards := []Card{
		{ID: "later-late", Source: "todo", At: at(19), Timed: true, State: "open"},
		{ID: "later-rank-2", Source: "todo", At: at(18), Timed: true, State: "open", PinnedRank: rank(2)},
		{ID: "later-early", Source: "todo", At: at(16), Timed: true, State: "open"},
		{ID: "later-rank-1", Source: "todo", At: at(17), Timed: true, State: "open", PinnedRank: rank(1)},
		{ID: "missed-early", Source: "todo", At: at(13), Timed: true, State: "open"},
		{ID: "missed-late", Source: "todo", At: at(14), Timed: true, State: "open"},
		{ID: "missed-rank-2", Source: "todo", At: at(12), Timed: true, State: "open", PinnedRank: rank(2)},
		{ID: "missed-rank-1", Source: "todo", At: at(11), Timed: true, State: "open", PinnedRank: rank(1)},
		{ID: "now-rank-2", Source: "todo", At: at(15), Timed: true, State: "open", PinnedRank: rank(2)},
		{ID: "now-slot", Source: "todo", At: at(15), Timed: true, State: "open"},
		{ID: "now-rank-1", Source: "todo", At: at(15), Timed: true, State: "open", PinnedRank: rank(1)},
		{ID: "untimed", Source: "todo", State: "open"},
		{ID: "agent", Source: "agent", State: "open"},
		{ID: "done-pinned", Source: "todo", At: at(16), Timed: true, State: "done", PinnedRank: rank(0)},
	}
	got := Section(cards, now)
	assertIDs := func(section []Card, want ...string) {
		t.Helper()
		if len(section) != len(want) {
			t.Fatalf("section IDs=%v, want %v", cardIDs(section), want)
		}
		for i := range want {
			if section[i].ID != want[i] {
				t.Fatalf("section IDs=%v, want %v", cardIDs(section), want)
			}
		}
	}
	assertIDs(got.Now, "now-rank-1", "now-rank-2", "now-slot", "untimed", "agent")
	assertIDs(got.Later, "later-rank-1", "later-rank-2", "later-early", "later-late")
	assertIDs(got.Missed, "missed-rank-1", "missed-rank-2", "missed-late", "missed-early")
}
func TestSectionManualCardsUseManualOrder(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	cards := []Card{
		{ID: "manual-2", Source: "manual", State: "open", order: 2},
		{ID: "todo", Source: "todo", State: "open", order: 0},
		{ID: "manual-1", Source: "manual", State: "open", order: 1},
	}
	got := Section(cards, now)
	if len(got.Now) != 3 || got.Now[0].ID != "manual-1" || got.Now[1].ID != "manual-2" || got.Now[2].ID != "todo" {
		t.Fatalf("manual card order = %v, want [manual-1 manual-2 todo]", cardIDs(got.Now))
	}
}

func cardIDs(cards []Card) []string {
	ids := make([]string, len(cards))
	for i := range cards {
		ids[i] = cards[i].ID
	}
	return ids
}
