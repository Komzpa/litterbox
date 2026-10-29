package cards

import (
	"container/list"
	"crypto/sha256"
	"sync"

	"github.com/Komzpa/litterbox/server/internal/mailhtml"
)

const previewCacheEntries = 10000
const previewCacheBytes = 32 << 20

type previewKey struct {
	tenant, card, version string
	hash [sha256.Size]byte
}
type previewEntry struct {
	key previewKey
	text string
}
type previewCache struct {
	mu sync.Mutex
	items map[previewKey]*list.Element
	order list.List
	bytes int
}

// snapshot keeps entries alive for a request even if concurrent requests evict them.
func (c *previewCache) snapshot(tenant string) (map[string]previewEntry, map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries := make(map[string]previewEntry)
	versions := make(map[string]string)
	for e := c.order.Front(); e != nil; e = e.Next() {
		entry := e.Value.(previewEntry)
		if entry.key.tenant != tenant { continue }
		if _, exists := entries[entry.key.card]; exists { continue }
		entries[entry.key.card] = entry
		versions[entry.key.card] = entry.key.version
	}
	return entries, versions
}

func (c *previewCache) touch(key previewKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.items[key]; e != nil { c.order.MoveToFront(e) }
}

func (c *previewCache) project(tenant, card, version, body string) string {
	key := previewKey{tenant: tenant, card: card, version: version, hash: sha256.Sum256([]byte(body))}
	c.mu.Lock()
	if e := c.items[key]; e != nil {
		c.order.MoveToFront(e)
		text := e.Value.(previewEntry).text
		c.mu.Unlock()
		return text
	}
	c.mu.Unlock()
	text := mailhtml.PreviewText(body)
	if len(text) > previewCacheBytes { return text }
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil { c.items = make(map[previewKey]*list.Element) }
	if e := c.items[key]; e != nil {
		c.order.MoveToFront(e)
		return e.Value.(previewEntry).text
	}
	c.items[key] = c.order.PushFront(previewEntry{key: key, text: text})
	c.bytes += len(text)
	for c.order.Len() > previewCacheEntries || c.bytes > previewCacheBytes {
		e := c.order.Back()
		entry := e.Value.(previewEntry)
		delete(c.items, entry.key)
		c.bytes -= len(entry.text)
		c.order.Remove(e)
	}
	return text
}
