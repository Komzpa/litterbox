package gmailsync

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPeopleIndexCacheReusesAndBacksOff(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	c := peopleIndexCache{now: func() time.Time { return now }}
	key := peopleIndexKey{accountKey{uuid.New(), uuid.New()}, "people"}
	calls := 0
	fail := false
	build := func() (map[string]string, error) {
		calls++
		if fail {
			return nil, errors.New("people API /otherContacts: 429 Too Many Requests")
		}
		return map[string]string{"a@example.test": "photo"}, nil
	}
	// Per-minute polls within the TTL reuse one build.
	for i := 0; i < 60; i++ {
		idx, err, _ := c.get(key, build)
		if err != nil || idx["a@example.test"] != "photo" {
			t.Fatalf("poll %d: idx=%v err=%v", i, idx, err)
		}
		now = now.Add(time.Minute)
	}
	if calls != 1 {
		t.Fatalf("builds=%d within TTL, want 1", calls)
	}
	// After TTL a failing rebuild is attempted once, keeps the stale index,
	// and then backs off instead of retrying every poll.
	now = now.Add(peopleIndexTTL)
	fail = true
	idx, err, attempted := c.get(key, build)
	if err == nil || !attempted || idx["a@example.test"] != "photo" {
		t.Fatalf("failed rebuild: idx=%v err=%v attempted=%v", idx, err, attempted)
	}
	for i := 0; i < 14; i++ {
		now = now.Add(time.Minute)
		if _, err, attempted := c.get(key, build); err != nil || attempted {
			t.Fatalf("minute %d retried during backoff", i)
		}
	}
	if calls != 2 {
		t.Fatalf("builds=%d, want 2 (no retry inside backoff)", calls)
	}
	now = now.Add(2 * time.Minute)
	if _, _, attempted := c.get(key, build); !attempted {
		t.Fatal("no retry after backoff expired")
	}
	// Backoff grows on repeated failure.
	now = now.Add(16 * time.Minute)
	if _, _, attempted := c.get(key, build); attempted {
		t.Fatal("second backoff should be 30 min")
	}
	now = now.Add(15 * time.Minute)
	fail = false
	if idx, err, attempted := c.get(key, build); !attempted || err != nil || idx == nil {
		t.Fatal("recovery rebuild failed")
	}
}
