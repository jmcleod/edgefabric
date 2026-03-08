package cdnserver

import (
	"net/http"
	"testing"
	"time"
)

func TestCacheGetPut(t *testing.T) {
	c := NewCache(100)

	key := CacheKey("GET", "example.com", "/foo", "a=1&b=2")
	headers := http.Header{"Content-Type": []string{"text/html"}}

	// Miss.
	if _, ok := c.Get(key); ok {
		t.Error("expected cache miss on empty cache")
	}

	// Put.
	c.Put(key, []byte("hello"), 200, headers, 60*time.Second)

	// Hit.
	entry, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(entry.body) != "hello" {
		t.Errorf("expected body 'hello', got %q", string(entry.body))
	}
	if entry.statusCode != 200 {
		t.Errorf("expected status 200, got %d", entry.statusCode)
	}
	if entry.headers.Get("Content-Type") != "text/html" {
		t.Errorf("expected Content-Type text/html, got %q", entry.headers.Get("Content-Type"))
	}

	// Stats.
	hits, misses := c.Stats()
	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := NewCache(3)
	headers := http.Header{}

	// Fill cache to capacity.
	c.Put(CacheKey("GET", "a.com", "/1", ""), []byte("1"), 200, headers, time.Hour)
	c.Put(CacheKey("GET", "a.com", "/2", ""), []byte("2"), 200, headers, time.Hour)
	c.Put(CacheKey("GET", "a.com", "/3", ""), []byte("3"), 200, headers, time.Hour)

	if c.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.Len())
	}

	// Adding a 4th entry should evict the LRU (key 1).
	c.Put(CacheKey("GET", "a.com", "/4", ""), []byte("4"), 200, headers, time.Hour)

	if c.Len() != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", c.Len())
	}

	// Key 1 should be evicted.
	if _, ok := c.Get(CacheKey("GET", "a.com", "/1", "")); ok {
		t.Error("key 1 should have been evicted")
	}

	// Key 4 should be present.
	if _, ok := c.Get(CacheKey("GET", "a.com", "/4", "")); !ok {
		t.Error("key 4 should be present")
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache(100)
	headers := http.Header{}

	key := CacheKey("GET", "a.com", "/expired", "")
	// TTL of 1 millisecond.
	c.Put(key, []byte("old"), 200, headers, time.Millisecond)

	// Wait for expiry.
	time.Sleep(5 * time.Millisecond)

	if _, ok := c.Get(key); ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestCachePurge(t *testing.T) {
	c := NewCache(100)
	headers := http.Header{}

	c.Put(CacheKey("GET", "a.com", "/1", ""), []byte("1"), 200, headers, time.Hour)
	c.Put(CacheKey("GET", "a.com", "/2", ""), []byte("2"), 200, headers, time.Hour)

	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}

	c.Purge()

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after purge, got %d", c.Len())
	}
}

func TestCacheKeyDeterministic(t *testing.T) {
	// Query params in different order should produce same key.
	k1 := CacheKey("GET", "example.com", "/path", "b=2&a=1")
	k2 := CacheKey("GET", "example.com", "/path", "a=1&b=2")
	if k1 != k2 {
		t.Errorf("expected same key, got %q and %q", k1, k2)
	}

	// Different methods should produce different keys.
	k3 := CacheKey("POST", "example.com", "/path", "a=1")
	k4 := CacheKey("GET", "example.com", "/path", "a=1")
	if k3 == k4 {
		t.Error("expected different keys for different methods")
	}
}

func TestCacheSkipsNonCacheable(t *testing.T) {
	c := NewCache(100)

	// Should skip non-2xx.
	c.Put("key1", []byte("err"), 500, http.Header{}, time.Hour)
	if c.Len() != 0 {
		t.Error("should not cache 5xx responses")
	}

	// Should skip no-store.
	c.Put("key2", []byte("ok"), 200, http.Header{
		"Cache-Control": []string{"no-store"},
	}, time.Hour)
	if c.Len() != 0 {
		t.Error("should not cache no-store responses")
	}

	// Should skip no-cache.
	c.Put("key3", []byte("ok"), 200, http.Header{
		"Cache-Control": []string{"no-cache"},
	}, time.Hour)
	if c.Len() != 0 {
		t.Error("should not cache no-cache responses")
	}
}

func TestCacheUpdateExisting(t *testing.T) {
	c := NewCache(100)
	headers := http.Header{}

	key := CacheKey("GET", "a.com", "/update", "")
	c.Put(key, []byte("v1"), 200, headers, time.Hour)
	c.Put(key, []byte("v2"), 200, headers, time.Hour)

	if c.Len() != 1 {
		t.Errorf("expected 1 entry after update, got %d", c.Len())
	}

	entry, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(entry.body) != "v2" {
		t.Errorf("expected body 'v2', got %q", string(entry.body))
	}
}
