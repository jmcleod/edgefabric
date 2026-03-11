package cdnserver

import (
	"net/http"
	"testing"
	"time"
)

func TestDiskCachePutGet(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	headers := http.Header{"Content-Type": []string{"text/html"}}
	dc.Put("GET|example.com|/index", []byte("hello world"), 200, headers, 5*time.Minute)

	entry, found := dc.Get("GET|example.com|/index")
	if !found {
		t.Fatal("expected cache hit")
	}
	if string(entry.body) != "hello world" {
		t.Errorf("body = %q, want %q", entry.body, "hello world")
	}
	if entry.statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", entry.statusCode)
	}
	if entry.headers.Get("Content-Type") != "text/html" {
		t.Errorf("Content-Type = %q, want %q", entry.headers.Get("Content-Type"), "text/html")
	}

	hits, misses := dc.Stats()
	if hits != 1 || misses != 0 {
		t.Errorf("stats: hits=%d misses=%d, want 1/0", hits, misses)
	}
}

func TestDiskCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	headers := http.Header{}
	dc.Put("GET|example.com|/expire", []byte("data"), 200, headers, 1*time.Millisecond)

	// Wait for expiry.
	time.Sleep(5 * time.Millisecond)

	_, found := dc.Get("GET|example.com|/expire")
	if found {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestDiskCacheEviction(t *testing.T) {
	dir := t.TempDir()
	// Max 100 bytes.
	dc, err := NewDiskCache(dir, 100, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	headers := http.Header{}

	// Put 60 bytes.
	dc.Put("key1", make([]byte, 60), 200, headers, 5*time.Minute)
	if dc.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", dc.Len())
	}

	// Put another 60 bytes — should evict key1 to make room.
	dc.Put("key2", make([]byte, 60), 200, headers, 5*time.Minute)
	if dc.Len() != 1 {
		t.Fatalf("expected 1 entry after eviction, got %d", dc.Len())
	}

	// key1 should be evicted.
	_, found := dc.Get("key1")
	if found {
		t.Error("expected key1 to be evicted")
	}

	// key2 should exist.
	_, found = dc.Get("key2")
	if !found {
		t.Error("expected key2 to exist")
	}
}

func TestDiskCachePurge(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	headers := http.Header{}
	dc.Put("k1", []byte("a"), 200, headers, 5*time.Minute)
	dc.Put("k2", []byte("b"), 200, headers, 5*time.Minute)

	if dc.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", dc.Len())
	}

	dc.Purge()
	if dc.Len() != 0 {
		t.Errorf("expected 0 entries after purge, got %d", dc.Len())
	}
}

func TestDiskCacheRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	headers := http.Header{"X-Test": []string{"value"}}
	dc.Put("rebuild-key", []byte("rebuild-data"), 200, headers, 5*time.Minute)

	// Create a new DiskCache pointing to the same directory.
	dc2, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache (rebuild): %v", err)
	}

	if dc2.Len() != 1 {
		t.Fatalf("expected 1 entry after rebuild, got %d", dc2.Len())
	}

	entry, found := dc2.Get("rebuild-key")
	if !found {
		t.Fatal("expected cache hit after rebuild")
	}
	if string(entry.body) != "rebuild-data" {
		t.Errorf("body = %q, want %q", entry.body, "rebuild-data")
	}
}

func TestDiskCacheSkipsNonCacheable(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}

	// no-store should be skipped.
	headers := http.Header{"Cache-Control": []string{"no-store"}}
	dc.Put("nostore", []byte("data"), 200, headers, 5*time.Minute)
	if dc.Len() != 0 {
		t.Error("expected no-store to be skipped")
	}

	// Non-2xx should be skipped.
	dc.Put("err", []byte("data"), 500, http.Header{}, 5*time.Minute)
	if dc.Len() != 0 {
		t.Error("expected non-2xx to be skipped")
	}
}

func TestHybridCacheMemoryHit(t *testing.T) {
	dir := t.TempDir()
	hc, err := NewHybridCache(100, dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewHybridCache: %v", err)
	}

	headers := http.Header{"Content-Type": []string{"text/plain"}}
	hc.Put("mem-key", []byte("mem-data"), 200, headers, 5*time.Minute)

	entry, found := hc.Get("mem-key")
	if !found {
		t.Fatal("expected hybrid cache hit from memory")
	}
	if string(entry.body) != "mem-data" {
		t.Errorf("body = %q, want %q", entry.body, "mem-data")
	}

	hits, _ := hc.Stats()
	if hits != 1 {
		t.Errorf("hits = %d, want 1", hits)
	}
}

func TestHybridCacheSpillToDisk(t *testing.T) {
	dir := t.TempDir()
	// Memory capacity of 2 entries.
	hc, err := NewHybridCache(2, dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewHybridCache: %v", err)
	}

	headers := http.Header{}
	// Fill memory.
	hc.Put("k1", []byte("v1"), 200, headers, 5*time.Minute)
	hc.Put("k2", []byte("v2"), 200, headers, 5*time.Minute)

	// This should evict k1 from memory → spill to disk.
	hc.Put("k3", []byte("v3"), 200, headers, 5*time.Minute)

	// k1 should be on disk now.
	if hc.disk.Len() == 0 {
		t.Fatal("expected at least one entry spilled to disk")
	}

	// k1 should still be retrievable (from disk).
	entry, found := hc.Get("k1")
	if !found {
		t.Fatal("expected k1 to be retrievable from disk via hybrid cache")
	}
	if string(entry.body) != "v1" {
		t.Errorf("body = %q, want %q", entry.body, "v1")
	}
}

func TestHybridCacheDiskPromotion(t *testing.T) {
	dir := t.TempDir()
	hc, err := NewHybridCache(2, dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewHybridCache: %v", err)
	}

	headers := http.Header{}
	// Fill memory to force spill.
	hc.Put("k1", []byte("v1"), 200, headers, 5*time.Minute)
	hc.Put("k2", []byte("v2"), 200, headers, 5*time.Minute)
	hc.Put("k3", []byte("v3"), 200, headers, 5*time.Minute) // k1 spills to disk

	// Verify k1 is on disk.
	diskLenBefore := hc.disk.Len()
	if diskLenBefore == 0 {
		t.Fatal("expected k1 on disk before promotion")
	}

	// Get k1 — should promote from disk to memory.
	entry, found := hc.Get("k1")
	if !found {
		t.Fatal("expected k1 from disk")
	}
	if string(entry.body) != "v1" {
		t.Errorf("body = %q, want %q", entry.body, "v1")
	}

	// k1 should now be in memory and removed from disk.
	memEntry, memFound := hc.memory.Get("k1")
	if !memFound {
		t.Error("expected k1 promoted to memory")
	} else if string(memEntry.body) != "v1" {
		t.Errorf("promoted body = %q, want %q", memEntry.body, "v1")
	}
}

func TestHybridCachePurge(t *testing.T) {
	dir := t.TempDir()
	hc, err := NewHybridCache(2, dir, 10*1024*1024, nil)
	if err != nil {
		t.Fatalf("NewHybridCache: %v", err)
	}

	headers := http.Header{}
	hc.Put("k1", []byte("v1"), 200, headers, 5*time.Minute)
	hc.Put("k2", []byte("v2"), 200, headers, 5*time.Minute)
	hc.Put("k3", []byte("v3"), 200, headers, 5*time.Minute) // spill

	hc.Purge()
	if hc.Len() != 0 {
		t.Errorf("expected 0 entries after purge, got %d", hc.Len())
	}
}

func TestCacheBackendInterface(t *testing.T) {
	// Compile-time checks are already in diskcache.go, but verify at runtime too.
	var _ CacheBackend = (*Cache)(nil)
	var _ CacheBackend = (*DiskCache)(nil)
	var _ CacheBackend = (*HybridCache)(nil)
}
