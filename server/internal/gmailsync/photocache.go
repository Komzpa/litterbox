package gmailsync

import (
	"sync"
	"time"
)

const senderPhotoTTL = 24 * time.Hour
const senderPhotoMissTTL = 6 * time.Hour
const senderPhotoRetryTTL = 15 * time.Minute

type photoCacheKey struct {
	account      accountKey
	surface, key string
}
type photoCacheEntry struct {
	raw     []byte
	ct, url string
	expires time.Time
}

// Account isolation also covers public domain logos. Cache lifetime is process
// local; persisted sender_photos remain readable across restarts and failures.
// The account gate serializes builds; the mutex protects different accounts.
type photoCache struct {
	mu      sync.Mutex
	entries map[photoCacheKey]photoCacheEntry
	now     func() time.Time
}

func (c *photoCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
func (c *photoCache) get(k photoCacheKey) (photoCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok || !c.clock().Before(e.expires) {
		return photoCacheEntry{}, false
	}
	return e, true
}
func (c *photoCache) put(k photoCacheKey, e photoCacheEntry, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[photoCacheKey]photoCacheEntry{}
	}
	now := c.clock()
	for key, v := range c.entries {
		if !now.Before(v.expires) {
			delete(c.entries, key)
		}
	}
	// Sender metadata must fit the entire 200-sender poll across accounts.
	// Bound domain image bytes separately to 128 x 512 KiB.
	if len(e.raw) > 0 {
		count := 0
		var oldest photoCacheKey
		var expiry time.Time
		for key, v := range c.entries {
			if len(v.raw) > 0 {
				count++
				if expiry.IsZero() || v.expires.Before(expiry) {
					oldest, expiry = key, v.expires
				}
			}
		}
		if count >= 128 {
			delete(c.entries, oldest)
		}
	}
	if len(c.entries) >= 4096 {
		var oldest photoCacheKey
		var expiry time.Time
		for key, v := range c.entries {
			if expiry.IsZero() || v.expires.Before(expiry) {
				oldest, expiry = key, v.expires
			}
		}
		delete(c.entries, oldest)
	}
	e.expires = now.Add(ttl)
	c.entries[k] = e
}
