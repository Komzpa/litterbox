package gmailsync

import (
	"sync"
	"time"
)

// People/Directory indexes change slowly, but the sync loop runs every poll.
// Rebuilding them each minute exhausts Google's per-user People sync quota
// (429 RESOURCE_EXHAUSTED), so each account's index is reused for
// peopleIndexTTL and failures back off instead of retrying every poll.
const (
	peopleIndexTTL        = 6 * time.Hour
	peopleIndexMinBackoff = 15 * time.Minute
)

type peopleIndexEntry struct {
	index   map[string]string
	builtAt time.Time
	nextTry time.Time
	backoff time.Duration
	ok      bool
}

type peopleIndexCache struct {
	mu      sync.Mutex
	entries map[peopleIndexKey]*peopleIndexEntry
	now     func() time.Time
}

type peopleIndexKey struct {
	account accountKey
	surface string
}

func (c *peopleIndexCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns the index for key, calling build only when the cached copy is
// stale and no backoff is pending. A failed build keeps serving the last good
// index; attempted reports whether build ran (so callers log only real
// attempts).
func (c *peopleIndexCache) get(key peopleIndexKey, build func() (map[string]string, error)) (index map[string]string, err error, attempted bool) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[peopleIndexKey]*peopleIndexEntry{}
	}
	e := c.entries[key]
	if e == nil {
		e = &peopleIndexEntry{}
		c.entries[key] = e
	}
	now := c.clock()
	if (e.ok && now.Sub(e.builtAt) < peopleIndexTTL) || now.Before(e.nextTry) {
		index = e.index
		c.mu.Unlock()
		return index, nil, false
	}
	c.mu.Unlock()

	built, err := build()

	c.mu.Lock()
	defer c.mu.Unlock()
	now = c.clock()
	if err != nil {
		if e.backoff < peopleIndexMinBackoff {
			e.backoff = peopleIndexMinBackoff
		} else if e.backoff < peopleIndexTTL {
			e.backoff *= 2
			if e.backoff > peopleIndexTTL {
				e.backoff = peopleIndexTTL
			}
		}
		e.nextTry = now.Add(e.backoff)
		return e.index, err, true
	}
	e.index, e.builtAt, e.ok, e.backoff, e.nextTry = built, now, true, 0, time.Time{}
	return built, nil, true
}
