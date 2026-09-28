package todos

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadDirIncludesOpenTodayAndCarriedOverOnly(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "2026-09-26", "- [ ] 22:00 Older carried-over task\n")
	writeNote(t, root, "2026-09-27", "- [ ] 23:00 Yesterday's unfinished item\n- [x] 22:00 Completed item\n")
	writeNote(t, root, "2026-09-28", "- [ ] 09:30 Today's task\n- [ ] (добавлять по ходу дня)\n- [x] Completed today\n- Future note\n")
	writeNote(t, root, "2026-09-29", "- [ ] Future task\n")

	loc := time.FixedZone("Tbilisi", 4*60*60)
	cards, err := ReadDir(root, time.Date(2026, 9, 28, 14, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want today's and carried-over open items", len(cards))
	}
	if cards[0].Title != "Yesterday's unfinished item" || cards[0].SortAt.Format(time.RFC3339) != "2026-09-27T19:00:00Z" {
		t.Fatalf("unexpected carried-over card: %+v", cards[0])
	}
	if cards[1].Title != "Today's task" || cards[1].SortAt.Format(time.RFC3339) != "2026-09-28T05:30:00Z" {
		t.Fatalf("unexpected today's card: %+v", cards[1])
	}
	for _, card := range cards {
		if card.Source != "todo" || card.State != "open" || len(card.ExternalID) != 64 {
			t.Fatalf("unexpected card identity/state: %+v", card)
		}
	}
}

func TestExternalIDUsesNoteDateAndOriginalItemText(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "2026-09-27", "- [ ] 09:30 Same item\n")
	writeNote(t, root, "2026-09-28", "- [ ] 09:30 Same item\n")
	cards, err := ReadDir(root, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].ExternalID == cards[1].ExternalID {
		t.Fatalf("date-qualified stable IDs not produced: %+v", cards)
	}
	again, err := ReadDir(root, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ExternalID != cards[0].ExternalID || again[1].ExternalID != cards[1].ExternalID {
		t.Fatalf("IDs changed between reads: %v vs %v", cards, again)
	}
}

func writeNote(t *testing.T, root, date, contents string) {
	t.Helper()
	path := filepath.Join(root, "2026", "09", date+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
