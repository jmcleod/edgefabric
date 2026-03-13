package cdnserver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Ensure DiskCache and HybridCache implement CacheBackend at compile time.
var (
	_ CacheBackend = (*DiskCache)(nil)
	_ CacheBackend = (*HybridCache)(nil)
)

// diskCacheMetadata is stored alongside each cached response on disk.
type diskCacheMetadata struct {
	Key        string        `json:"key"`
	StatusCode int           `json:"status_code"`
	Headers    http.Header   `json:"headers"`
	StoredAt   time.Time     `json:"stored_at"`
	TTL        time.Duration `json:"ttl"`
	BodySize   int64         `json:"body_size"`
}

// diskIndexEntry tracks a cached entry's location on disk.
type diskIndexEntry struct {
	hash     string
	storedAt time.Time
	size     int64 // body size in bytes
}

// DiskCache stores cached HTTP responses on disk using content-addressable paths.
type DiskCache struct {
	mu        sync.RWMutex
	baseDir   string
	maxBytes  int64
	usedBytes int64
	index     map[string]*diskIndexEntry // cache key → index entry
	hits      uint64
	misses    uint64
	logger    *slog.Logger
}

// NewDiskCache creates a new disk-based cache in the given directory.
func NewDiskCache(baseDir string, maxBytes int64, logger *slog.Logger) (*DiskCache, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	dc := &DiskCache{
		baseDir:  baseDir,
		maxBytes: maxBytes,
		index:    make(map[string]*diskIndexEntry),
		logger:   logger,
	}
	dc.rebuildIndex()
	return dc, nil
}

func (d *DiskCache) hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h)
}

func (d *DiskCache) pathForHash(hash string) string {
	return filepath.Join(d.baseDir, hash[:2], hash)
}

func (d *DiskCache) Get(key string) (*cacheEntry, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ie, ok := d.index[key]
	if !ok {
		d.misses++
		return nil, false
	}

	basePath := d.pathForHash(ie.hash)
	metaBytes, err := os.ReadFile(basePath + ".meta")
	if err != nil {
		d.removeEntryLocked(key, ie)
		d.misses++
		return nil, false
	}

	var meta diskCacheMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		d.removeEntryLocked(key, ie)
		d.misses++
		return nil, false
	}

	// Check TTL.
	if meta.TTL > 0 && time.Since(meta.StoredAt) > meta.TTL {
		d.removeEntryLocked(key, ie)
		d.misses++
		return nil, false
	}

	body, err := os.ReadFile(basePath + ".body")
	if err != nil {
		d.removeEntryLocked(key, ie)
		d.misses++
		return nil, false
	}

	d.hits++
	return &cacheEntry{
		key:        key,
		body:       body,
		statusCode: meta.StatusCode,
		headers:    meta.Headers,
		storedAt:   meta.StoredAt,
		ttl:        meta.TTL,
	}, true
}

func (d *DiskCache) Put(key string, body []byte, statusCode int, headers http.Header, ttl time.Duration) {
	if shouldSkipCache(headers) {
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove existing entry if present.
	if ie, ok := d.index[key]; ok {
		d.removeEntryLocked(key, ie)
	}

	// Evict oldest entries if over capacity.
	bodySize := int64(len(body))
	for d.usedBytes+bodySize > d.maxBytes && len(d.index) > 0 {
		d.evictOldestLocked()
	}

	hash := d.hashKey(key)
	basePath := d.pathForHash(hash)

	// Create subdirectory.
	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		d.logger.Warn("disk cache mkdir failed", slog.String("error", err.Error()))
		return
	}

	meta := diskCacheMetadata{
		Key:        key,
		StatusCode: statusCode,
		Headers:    headers.Clone(),
		StoredAt:   time.Now(),
		TTL:        ttl,
		BodySize:   bodySize,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return
	}

	if err := os.WriteFile(basePath+".body", body, 0o644); err != nil {
		d.logger.Warn("disk cache write body failed", slog.String("error", err.Error()))
		return
	}
	if err := os.WriteFile(basePath+".meta", metaBytes, 0o644); err != nil {
		os.Remove(basePath + ".body")
		d.logger.Warn("disk cache write meta failed", slog.String("error", err.Error()))
		return
	}

	d.index[key] = &diskIndexEntry{
		hash:     hash,
		storedAt: meta.StoredAt,
		size:     bodySize,
	}
	d.usedBytes += bodySize
}

func (d *DiskCache) Purge() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, ie := range d.index {
		d.removeFilesLocked(ie.hash)
		delete(d.index, key)
	}
	d.usedBytes = 0
}

func (d *DiskCache) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.index)
}

func (d *DiskCache) Stats() (hits, misses uint64) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hits, d.misses
}

func (d *DiskCache) removeEntryLocked(key string, ie *diskIndexEntry) {
	d.removeFilesLocked(ie.hash)
	d.usedBytes -= ie.size
	delete(d.index, key)
}

func (d *DiskCache) removeFilesLocked(hash string) {
	basePath := d.pathForHash(hash)
	os.Remove(basePath + ".body")
	os.Remove(basePath + ".meta")
}

func (d *DiskCache) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	for key, ie := range d.index {
		if oldestKey == "" || ie.storedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = ie.storedAt
		}
	}
	if oldestKey != "" {
		d.removeEntryLocked(oldestKey, d.index[oldestKey])
	}
}

// rebuildIndex scans the cache directory to rebuild the in-memory index.
func (d *DiskCache) rebuildIndex() {
	entries, err := os.ReadDir(d.baseDir)
	if err != nil {
		return
	}
	for _, dir := range entries {
		if !dir.IsDir() || len(dir.Name()) != 2 {
			continue
		}
		subDir := filepath.Join(d.baseDir, dir.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, f := range subEntries {
			name := f.Name()
			if len(name) < 6 || name[len(name)-5:] != ".meta" {
				continue
			}
			metaPath := filepath.Join(subDir, name)
			metaBytes, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var meta diskCacheMetadata
			if err := json.Unmarshal(metaBytes, &meta); err != nil {
				continue
			}
			hash := name[:len(name)-5]
			d.index[meta.Key] = &diskIndexEntry{
				hash:     hash,
				storedAt: meta.StoredAt,
				size:     meta.BodySize,
			}
			d.usedBytes += meta.BodySize
		}
	}
}

// HybridCache combines a fast in-memory LRU with a disk-based cache.
// Evicted memory entries spill to disk; disk hits are promoted to memory.
type HybridCache struct {
	memory *Cache
	disk   *DiskCache
	logger *slog.Logger
	mu     sync.RWMutex
	hits   uint64
	misses uint64
}

// NewHybridCache creates a hybrid cache with the given memory capacity and disk config.
func NewHybridCache(memEntries int, diskDir string, diskMaxBytes int64, logger *slog.Logger) (*HybridCache, error) {
	disk, err := NewDiskCache(diskDir, diskMaxBytes, logger)
	if err != nil {
		return nil, err
	}

	hc := &HybridCache{
		memory: NewCache(memEntries),
		disk:   disk,
		logger: logger,
	}

	// Wire up spill-to-disk on memory eviction.
	hc.memory.OnEvict = func(entry *cacheEntry) {
		disk.Put(entry.key, entry.body, entry.statusCode, entry.headers, entry.ttl)
	}

	return hc, nil
}

func (h *HybridCache) Get(key string) (*cacheEntry, bool) {
	// Check memory first.
	if entry, found := h.memory.Get(key); found {
		h.mu.Lock()
		h.hits++
		h.mu.Unlock()
		return entry, true
	}

	// Check disk and promote to memory on hit.
	if entry, found := h.disk.Get(key); found {
		// Promote to memory (this may evict another entry to disk).
		h.memory.Put(key, entry.body, entry.statusCode, entry.headers, entry.ttl)
		// Remove from disk since it's now in memory.
		h.disk.mu.Lock()
		if ie, ok := h.disk.index[key]; ok {
			h.disk.removeEntryLocked(key, ie)
		}
		h.disk.mu.Unlock()
		h.mu.Lock()
		h.hits++
		h.mu.Unlock()
		return entry, true
	}

	h.mu.Lock()
	h.misses++
	h.mu.Unlock()
	return nil, false
}

func (h *HybridCache) Put(key string, body []byte, statusCode int, headers http.Header, ttl time.Duration) {
	// Put in memory. If at capacity, OnEvict will spill the evicted entry to disk.
	h.memory.Put(key, body, statusCode, headers, ttl)
}

func (h *HybridCache) Purge() {
	h.memory.Purge()
	h.disk.Purge()
}

func (h *HybridCache) Len() int {
	return h.memory.Len() + h.disk.Len()
}

func (h *HybridCache) Stats() (hits, misses uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hits, h.misses
}

