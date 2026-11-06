package gmailsync

import (
	"fmt"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestPhotoCacheTTLIsolationAndPollCapacity(t *testing.T) {
	now := time.Now()
	c := photoCache{now: func() time.Time { return now }}
	account := accountKey{uuid.New(), uuid.New()}
	key := photoCacheKey{account, "sender", "sender@example.test"}
	c.put(key, photoCacheEntry{}, senderPhotoMissTTL)
	for i := 0; i < 200; i++ {
		c.put(photoCacheKey{account, "sender", fmt.Sprint(i)}, photoCacheEntry{}, senderPhotoTTL)
	}
	if _, ok := c.get(key); !ok {
		t.Fatal("full sender poll evicted a fresh entry")
	}
	neighbor := key
	neighbor.account = accountKey{uuid.New(), uuid.New()}
	if _, ok := c.get(neighbor); ok {
		t.Fatal("cross-account cache hit")
	}
	now = now.Add(senderPhotoMissTTL)
	if _, ok := c.get(key); ok {
		t.Fatal("negative did not expire")
	}
	positive := photoCacheKey{account, "bimi", "example.test"}
	c.put(positive, photoCacheEntry{raw: []byte("image"), url: "old"}, senderPhotoTTL)
	now = now.Add(senderPhotoTTL)
	if e, ok := c.get(positive); ok || len(e.raw) > 0 || e.url != "" {
		t.Fatal("expired image returned")
	}
}
