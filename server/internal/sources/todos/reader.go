package todos

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var noteDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)
var checkboxPattern = regexp.MustCompile(`^[-*]\s+\[ \]\s+(.+?)\s*$`)
var itemTimePattern = regexp.MustCompile(`^(\d{1,2}:\d{2})\s+(.+)$`)

type Card struct {
	Source     string
	ExternalID string
	Title      string
	SortAt     time.Time
	State      string
}

// ReadDir reads unchecked tasks from today and the latest earlier daily note.
// Checked items and non-checkbox context are never cards.
func ReadDir(root string, today time.Time) ([]Card, error) {
	if root == "" {
		return nil, fmt.Errorf("daily notes directory is required")
	}
	todayDate := today.Format("2006-01-02")
	type datedNote struct {
		path string
		date string
	}
	var notes []datedNote
	var carryDate string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !noteDatePattern.MatchString(filepath.Base(path)) {
			return nil
		}
		dateText := strings.TrimSuffix(filepath.Base(path), ".md")
		if _, err := time.ParseInLocation("2006-01-02", dateText, today.Location()); err != nil || dateText > todayDate {
			return nil
		}
		if dateText < todayDate && dateText > carryDate {
			carryDate = dateText
		}
		notes = append(notes, datedNote{path: path, date: dateText})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find daily notes: %w", err)
	}
	var cards []Card
	for _, dated := range notes {
		if dated.date != todayDate && dated.date != carryDate {
			continue
		}
		noteDate, _ := time.ParseInLocation("2006-01-02", dated.date, today.Location())
		note, err := os.ReadFile(dated.path)
		if err != nil {
			return nil, fmt.Errorf("read daily note %s: %w", dated.date, err)
		}
		for _, line := range strings.Split(string(note), "\n") {
			match := checkboxPattern.FindStringSubmatch(strings.TrimSpace(line))
			if match == nil {
				continue
			}
			item := strings.TrimSpace(match[1])
			if item == "" || strings.EqualFold(item, "(добавлять по ходу дня)") {
				continue
			}
			title, sortAt := item, noteDate
			if timed := itemTimePattern.FindStringSubmatch(item); timed != nil {
				title = strings.TrimSpace(timed[2])
				if parsed, parseErr := time.ParseInLocation("15:04", timed[1], today.Location()); parseErr == nil {
					sortAt = time.Date(noteDate.Year(), noteDate.Month(), noteDate.Day(), parsed.Hour(), parsed.Minute(), 0, 0, today.Location())
				}
			}
			if title == "" {
				continue
			}
			digest := sha256.Sum256([]byte(dated.date + "\x00" + item))
			cards = append(cards, Card{
				Source:     "todo",
				ExternalID: hex.EncodeToString(digest[:]),
				Title:      title,
				SortAt:     sortAt.UTC(),
				State:      "open",
			})
		}
	}
	return cards, nil
}

func ReadDirAt(root string) ([]Card, error) {
	return ReadDir(root, time.Now())
}

// Upsert is tenant-scoped and leaves state unchanged on conflict so done cards
// remain done while the corresponding checkbox is still open in the note.
func Upsert(ctx context.Context, db *sql.DB, tenantID string, cards []Card) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id', $1, true)`, tenantID); err != nil {
		return err
	}
	for _, card := range cards {
		_, err = tx.ExecContext(ctx, `INSERT INTO cards (tenant_id,id,source,external_id,title,sort_at,state) VALUES ($1,gen_random_uuid(),'todo',$2,$3,$4,'open') ON CONFLICT (tenant_id,source,external_id) DO UPDATE SET title=EXCLUDED.title,sort_at=EXCLUDED.sort_at`, tenantID, card.ExternalID, card.Title, card.SortAt)
		if err != nil {
			return fmt.Errorf("upsert todo %q: %w", card.ExternalID, err)
		}
	}
	return tx.Commit()
}
