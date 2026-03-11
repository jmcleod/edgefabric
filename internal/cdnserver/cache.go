package cdnserver

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// cacheEntry holds a cached HTTP response.
type cacheEntry struct {
	body       []byte
	statusCode int
	headers    http.Header
	storedAt   time.Time
	ttl        time.Duration

	// LRU list pointers.
	prev, next *cacheEntry
	key        string
}

func (e *cacheEntry) expired() bool {
	if e.ttl <= 0 {
		return false // No TTL means never expires (controlled by eviction only).
	}
	return time.Since(e.storedAt) > e.ttl
}

// CacheBackend is the interface for cache storage implementations.
type CacheBackend interface {
	Get(key string) (*cacheEntry, bool)
	Put(key string, body []byte, statusCode int, headers http.Header, ttl time.Duration)
	Purge()
	Len() int
	Stats() (hits, misses uint64)
}

// Ensure Cache implements CacheBackend at compile time.
var _ CacheBackend = (*Cache)(nil)

// Cache is an in-memory LRU cache for HTTP responses.
type Cache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	maxEntries int
	head, tail *cacheEntry // Doubly-linked list: head = most recent, tail = least recent.
	hits       uint64
	misses     uint64
	OnEvict    func(entry *cacheEntry) // Called when an entry is evicted due to capacity.
}

// NewCache creates a new LRU cache with the given max entries.
func NewCache(maxEntries int) *Cache {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &Cache{
		entries:    make(map[string]*cacheEntry),
		maxEntries: maxEntries,
	}
}

// CacheKey generates a deterministic cache key from the request.
func CacheKey(method, host, path, rawQuery string) string {
	var b strings.Builder
	b.WriteString(method)
	b.WriteByte('|')
	b.WriteString(host)
	b.WriteByte('|')
	b.WriteString(path)

	if rawQuery != "" {
		// Sort query parameters for deterministic keys.
		params := strings.Split(rawQuery, "&")
		sort.Strings(params)
		b.WriteByte('?')
		b.WriteString(strings.Join(params, "&"))
	}
	return b.String()
}

// Get retrieves a cache entry if it exists and hasn't expired.
func (c *Cache) Get(key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false
	}

	if entry.expired() {
		c.removeEntry(entry)
		c.misses++
		return nil, false
	}

	// Move to head (most recently used).
	c.moveToHead(entry)
	c.hits++
	return entry, true
}

// Put stores a response in the cache. Only GET responses with 2xx status
// codes should be stored. Respects Cache-Control: no-store and no-cache.
func (c *Cache) Put(key string, body []byte, statusCode int, headers http.Header, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check Cache-Control from origin.
	if shouldSkipCache(headers) {
		return
	}

	// Only cache 2xx status codes.
	if statusCode < 200 || statusCode >= 300 {
		return
	}

	// If entry exists, update it.
	if existing, ok := c.entries[key]; ok {
		existing.body = body
		existing.statusCode = statusCode
		existing.headers = headers.Clone()
		existing.storedAt = time.Now()
		existing.ttl = ttl
		c.moveToHead(existing)
		return
	}

	// Evict LRU if at capacity.
	for len(c.entries) >= c.maxEntries {
		c.evictTail()
	}

	entry := &cacheEntry{
		key:        key,
		body:       body,
		statusCode: statusCode,
		headers:    headers.Clone(),
		storedAt:   time.Now(),
		ttl:        ttl,
	}
	c.entries[key] = entry
	c.addToHead(entry)
}

// Purge removes all entries from the cache.
func (c *Cache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.head = nil
	c.tail = nil
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache hit/miss counters.
func (c *Cache) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

// --- LRU linked list operations ---

func (c *Cache) addToHead(entry *cacheEntry) {
	entry.prev = nil
	entry.next = c.head
	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry
	if c.tail == nil {
		c.tail = entry
	}
}

func (c *Cache) removeEntry(entry *cacheEntry) {
	delete(c.entries, entry.key)
	c.unlinkEntry(entry)
}

func (c *Cache) unlinkEntry(entry *cacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		c.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		c.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

func (c *Cache) moveToHead(entry *cacheEntry) {
	if c.head == entry {
		return
	}
	c.unlinkEntry(entry)
	c.addToHead(entry)
}

func (c *Cache) evictTail() {
	if c.tail == nil {
		return
	}
	evicted := c.tail
	c.removeEntry(evicted)
	if c.OnEvict != nil {
		c.OnEvict(evicted)
	}
}

// shouldSkipCache checks if Cache-Control directives prohibit caching.
func shouldSkipCache(headers http.Header) bool {
	cc := headers.Get("Cache-Control")
	if cc == "" {
		return false
	}
	ccLower := strings.ToLower(cc)
	return strings.Contains(ccLower, "no-store") || strings.Contains(ccLower, "no-cache")
}
