package cdnserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"path/filepath"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// Ensure ProxyService implements Service at compile time.
var _ Service = (*ProxyService)(nil)

// siteRuntime holds the active configuration for a CDN site.
type siteRuntime struct {
	site          *domain.CDNSite
	origins       []*domain.CDNOrigin
	cache         CacheBackend
	rateLimiter   *RateLimiter
	healthChecker *HealthChecker
	headerRules   []cdn.HeaderRule
	waf           *WAF
}

// ProxyService implements the CDN reverse proxy server.
type ProxyService struct {
	mu         sync.RWMutex
	running    bool
	listenAddr string
	server     *http.Server
	logger     *slog.Logger

	// Domain → siteRuntime mapping.
	sites    map[string]*siteRuntime // domain → runtime
	siteByID map[domain.ID]*siteRuntime

	// Disk cache configuration.
	cacheDir      string // Base directory for disk cache. Empty = memory only.
	cacheMaxBytes int64  // Max disk cache bytes per site.

	// Prometheus metrics (optional).
	metrics *observability.Metrics

	// Internal counters.
	requestsTotal atomic.Uint64
}

// NewProxyService creates a new CDN reverse proxy service.
// The metrics parameter is optional and may be nil.
func NewProxyService(logger *slog.Logger, metrics *observability.Metrics) *ProxyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProxyService{
		sites:    make(map[string]*siteRuntime),
		siteByID: make(map[domain.ID]*siteRuntime),
		logger:   logger,
		metrics:  metrics,
	}
}

// SetCacheConfig configures disk-based caching. When set, the proxy creates
// HybridCache instances (memory + disk) instead of memory-only caches.
func (p *ProxyService) SetCacheConfig(dir string, maxBytes int64) {
	p.cacheDir = dir
	p.cacheMaxBytes = maxBytes
}

func (p *ProxyService) Start(_ context.Context, listenAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("cdn server already running")
	}

	p.listenAddr = listenAddr
	p.server = &http.Server{
		Addr:         listenAddr,
		Handler:      p,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}

	p.running = true

	go func() {
		if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			p.logger.Error("cdn server error", slog.String("error", err.Error()))
		}
	}()

	p.logger.Info("cdn proxy server started", slog.String("addr", listenAddr))
	return nil
}

func (p *ProxyService) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return fmt.Errorf("cdn server not running")
	}

	// Stop all health checkers.
	for _, sr := range p.siteByID {
		if sr.healthChecker != nil {
			sr.healthChecker.Stop()
		}
	}

	if err := p.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	p.running = false
	p.sites = make(map[string]*siteRuntime)
	p.siteByID = make(map[domain.ID]*siteRuntime)
	p.logger.Info("cdn proxy server stopped")
	return nil
}

func (p *ProxyService) Reconcile(_ context.Context, config *cdn.NodeCDNConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return fmt.Errorf("cdn server not running")
	}

	// Stop old health checkers.
	for _, sr := range p.siteByID {
		if sr.healthChecker != nil {
			sr.healthChecker.Stop()
		}
	}

	// Rebuild site routing table.
	newSites := make(map[string]*siteRuntime)
	newSiteByID := make(map[domain.ID]*siteRuntime)

	if config != nil {
		for _, swo := range config.Sites {
			sr := &siteRuntime{
				site:    swo.Site,
				origins: swo.Origins,
			}

			// Set up cache.
			if swo.Site.CacheEnabled {
				if p.cacheDir != "" {
					hc, err := NewHybridCache(10000, filepath.Join(p.cacheDir, swo.Site.ID.String()), p.cacheMaxBytes, p.logger)
					if err != nil {
						p.logger.Warn("disk cache init failed, falling back to memory",
							slog.String("site", swo.Site.Name),
							slog.String("error", err.Error()),
						)
						sr.cache = NewCache(10000)
					} else {
						sr.cache = hc
					}
				} else {
					sr.cache = NewCache(10000)
				}
			}

			// Set up rate limiter.
			if swo.Site.RateLimitRPS != nil && *swo.Site.RateLimitRPS > 0 {
				sr.rateLimiter = NewRateLimiter(*swo.Site.RateLimitRPS)
			}

			// Parse header rules.
			if swo.Site.HeaderRules != nil {
				var rules []cdn.HeaderRule
				if err := json.Unmarshal(swo.Site.HeaderRules, &rules); err == nil {
					sr.headerRules = rules
				}
			}

			// Set up WAF if enabled.
			if swo.Site.WAFEnabled {
				mode := WAFModeDetect
				if swo.Site.WAFMode == "block" {
					mode = WAFModeBlock
				}
				sr.waf = NewWAF(mode, DefaultRules(), p.logger, p.metrics)
			}

			// Set up health checker.
			if len(swo.Origins) > 0 {
				sr.healthChecker = NewHealthChecker(swo.Origins)
				sr.healthChecker.Start()
			}

			// Map each domain to this site.
			for _, d := range swo.Site.Domains {
				newSites[strings.ToLower(d)] = sr
			}
			newSiteByID[swo.Site.ID] = sr
		}
	}

	p.sites = newSites
	p.siteByID = newSiteByID

	p.logger.Info("cdn proxy reconciled", slog.Int("sites", len(newSiteByID)))
	return nil
}

func (p *ProxyService) PurgeCache(_ context.Context, siteID domain.ID) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return fmt.Errorf("cdn server not running")
	}

	sr, ok := p.siteByID[siteID]
	if !ok {
		return fmt.Errorf("site %s not found", siteID)
	}

	if sr.cache != nil {
		sr.cache.Purge()
	}

	return nil
}

func (p *ProxyService) GetStatus(_ context.Context) (*ServerStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var totalCacheEntries uint64
	var totalCacheHits uint64
	var totalCacheMisses uint64

	for _, sr := range p.siteByID {
		if sr.cache != nil {
			totalCacheEntries += uint64(sr.cache.Len())
			hits, misses := sr.cache.Stats()
			totalCacheHits += hits
			totalCacheMisses += misses
		}
	}

	return &ServerStatus{
		Listening:     p.running,
		ListenAddr:    p.listenAddr,
		SiteCount:     len(p.siteByID),
		CacheHits:     totalCacheHits,
		CacheMisses:   totalCacheMisses,
		CacheEntries:  totalCacheEntries,
		RequestsTotal: p.requestsTotal.Load(),
	}, nil
}

// countingWriter wraps an http.ResponseWriter and counts bytes written.
type countingWriter struct {
	http.ResponseWriter
	bytesWritten int64
}

func (cw *countingWriter) Write(b []byte) (int, error) {
	n, err := cw.ResponseWriter.Write(b)
	cw.bytesWritten += int64(n)
	return n, err
}

// ServeHTTP is the main request handler. It routes by Host header.
func (p *ProxyService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.requestsTotal.Add(1)

	// Look up site by Host header.
	host := strings.ToLower(r.Host)
	// Strip port if present.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	p.mu.RLock()
	sr, ok := p.sites[host]
	p.mu.RUnlock()

	if !ok {
		http.Error(w, "no CDN site configured for this host", http.StatusBadGateway)
		return
	}

	// Wrap writer to count bytes for per-tenant bandwidth metrics.
	cw := &countingWriter{ResponseWriter: w}
	w = cw
	defer func() {
		if p.metrics != nil && sr.site.TenantID.String() != "" {
			p.metrics.TenantCDNBandwidth.WithLabelValues(sr.site.TenantID.String()).Add(float64(cw.bytesWritten))
		}
	}()

	// Rate limiting.
	if sr.rateLimiter != nil && !sr.rateLimiter.Allow() {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// WAF inspection.
	if sr.waf != nil {
		if match := sr.waf.Inspect(r); match != nil {
			action := "logged"
			if sr.waf.mode == WAFModeBlock {
				action = "blocked"
			}
			p.logger.Warn("WAF match",
				slog.String("rule", match.Rule.ID),
				slog.String("category", string(match.Rule.Category)),
				slog.String("field", match.Field),
				slog.String("action", action),
				slog.String("host", host),
			)
			if p.metrics != nil {
				p.metrics.WAFMatchesTotal.WithLabelValues(string(match.Rule.Category), action).Inc()
			}
			if sr.waf.mode == WAFModeBlock {
				http.Error(w, "request blocked by WAF", http.StatusForbidden)
				return
			}
		}
	}

	// Cache check (GET only).
	if r.Method == http.MethodGet && sr.cache != nil {
		key := CacheKey(r.Method, host, r.URL.Path, r.URL.RawQuery)
		if entry, found := sr.cache.Get(key); found {
			// Write cached response.
			for k, v := range entry.headers {
				w.Header()[k] = v
			}
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(entry.statusCode)
			w.Write(entry.body) //nolint:errcheck
			return
		}
	}

	// Select origin.
	origin := p.selectOrigin(sr)
	if origin == nil {
		http.Error(w, "no healthy origin available", http.StatusBadGateway)
		return
	}

	// Build reverse proxy.
	target := &url.URL{
		Scheme: string(origin.Scheme),
		Host:   origin.Address,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = r.Host // Preserve original Host header.
		},
		ModifyResponse: func(resp *http.Response) error {
			// Apply header rules.
			applyHeaderRules(resp, sr.headerRules)

			// Cache the response if caching is enabled and method is GET.
			if r.Method == http.MethodGet && sr.cache != nil {
				p.maybeCacheResponse(sr, host, r, resp)
			}

			// Compression: if the origin didn't compress and the site has
			// compression enabled, we handle it at write time.
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			p.logger.Error("proxy error",
				slog.String("host", host),
				slog.String("origin", origin.Address),
				slog.String("error", err.Error()),
			)
			http.Error(w, "origin unavailable", http.StatusBadGateway)
		},
	}

	// Check if we should compress the response (brotli preferred over gzip).
	if sr.site.CompressionEnabled {
		if acceptsBrotli(r) {
			brw := &brotliResponseWriter{ResponseWriter: w, buf: &bytes.Buffer{}}
			w.Header().Set("X-Cache", "MISS")
			proxy.ServeHTTP(brw, r)
			brw.flush()
			return
		}
		if acceptsGzip(r) {
			gzw := &gzipResponseWriter{ResponseWriter: w, buf: &bytes.Buffer{}}
			w.Header().Set("X-Cache", "MISS")
			proxy.ServeHTTP(gzw, r)
			gzw.flush()
			return
		}
	}

	w.Header().Set("X-Cache", "MISS")
	proxy.ServeHTTP(w, r)
}

// selectOrigin chooses a healthy origin using weighted random selection.
func (p *ProxyService) selectOrigin(sr *siteRuntime) *domain.CDNOrigin {
	var candidates []*domain.CDNOrigin

	if sr.healthChecker != nil {
		candidates = sr.healthChecker.HealthyOrigins()
	} else {
		candidates = sr.origins
	}

	if len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		return candidates[0]
	}

	// Weighted random selection.
	totalWeight := 0
	for _, o := range candidates {
		totalWeight += o.Weight
	}

	if totalWeight <= 0 {
		return candidates[rand.Intn(len(candidates))]
	}

	r := rand.Intn(totalWeight)
	for _, o := range candidates {
		r -= o.Weight
		if r < 0 {
			return o
		}
	}

	return candidates[len(candidates)-1]
}

// maybeCacheResponse reads the response body, caches it, and replaces the
// body with a new reader so the proxy can still write it to the client.
func (p *ProxyService) maybeCacheResponse(sr *siteRuntime, host string, r *http.Request, resp *http.Response) {
	// Only cache 2xx.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}

	// Read body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Store in cache.
	key := CacheKey(r.Method, host, r.URL.Path, r.URL.RawQuery)
	ttl := time.Duration(sr.site.CacheTTL) * time.Second
	sr.cache.Put(key, body, resp.StatusCode, resp.Header, ttl)
}

// applyHeaderRules applies header manipulation rules to the response.
func applyHeaderRules(resp *http.Response, rules []cdn.HeaderRule) {
	for _, rule := range rules {
		switch rule.Action {
		case "add":
			resp.Header.Add(rule.Header, rule.Value)
		case "set":
			resp.Header.Set(rule.Header, rule.Value)
		case "remove":
			resp.Header.Del(rule.Header)
		}
	}
}

// acceptsBrotli checks if the client accepts brotli encoding.
func acceptsBrotli(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return strings.Contains(ae, "br")
}

// acceptsGzip checks if the client accepts gzip encoding.
func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return strings.Contains(ae, "gzip")
}

// gzipResponseWriter buffers the response and compresses it with gzip.
type gzipResponseWriter struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.statusCode = code
	// Don't write yet — we'll write after compression.
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	return g.buf.Write(data)
}

func (g *gzipResponseWriter) flush() {
	if g.statusCode == 0 {
		g.statusCode = http.StatusOK
	}

	// Only compress text-like content types.
	ct := g.ResponseWriter.Header().Get("Content-Type")
	if !shouldCompress(ct) || g.buf.Len() == 0 {
		g.ResponseWriter.WriteHeader(g.statusCode)
		g.ResponseWriter.Write(g.buf.Bytes()) //nolint:errcheck
		return
	}

	// Compress.
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(g.buf.Bytes()); err != nil {
		// Fall back to uncompressed.
		g.ResponseWriter.WriteHeader(g.statusCode)
		g.ResponseWriter.Write(g.buf.Bytes()) //nolint:errcheck
		return
	}
	gz.Close()

	g.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	g.ResponseWriter.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(g.statusCode)
	g.ResponseWriter.Write(compressed.Bytes()) //nolint:errcheck
}

// brotliResponseWriter buffers the response and compresses it with brotli.
type brotliResponseWriter struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

func (b *brotliResponseWriter) WriteHeader(code int) {
	b.statusCode = code
}

func (b *brotliResponseWriter) Write(data []byte) (int, error) {
	return b.buf.Write(data)
}

func (b *brotliResponseWriter) flush() {
	if b.statusCode == 0 {
		b.statusCode = http.StatusOK
	}

	ct := b.ResponseWriter.Header().Get("Content-Type")
	if !shouldCompress(ct) || b.buf.Len() == 0 {
		b.ResponseWriter.WriteHeader(b.statusCode)
		b.ResponseWriter.Write(b.buf.Bytes()) //nolint:errcheck
		return
	}

	var compressed bytes.Buffer
	bw := brotli.NewWriterLevel(&compressed, brotli.DefaultCompression)
	if _, err := bw.Write(b.buf.Bytes()); err != nil {
		b.ResponseWriter.WriteHeader(b.statusCode)
		b.ResponseWriter.Write(b.buf.Bytes()) //nolint:errcheck
		return
	}
	bw.Close()

	b.ResponseWriter.Header().Set("Content-Encoding", "br")
	b.ResponseWriter.Header().Del("Content-Length")
	b.ResponseWriter.WriteHeader(b.statusCode)
	b.ResponseWriter.Write(compressed.Bytes()) //nolint:errcheck
}

// shouldCompress returns true for text-like content types.
func shouldCompress(contentType string) bool {
	ct := strings.ToLower(contentType)
	compressible := []string{
		"text/",
		"application/json",
		"application/javascript",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
	}
	for _, prefix := range compressible {
		if strings.Contains(ct, prefix) {
			return true
		}
	}
	return false
}
