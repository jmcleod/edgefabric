package cdnserver

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestProxyStartStop(t *testing.T) {
	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx, addr); err == nil {
		t.Error("expected error on double start")
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if !status.Listening {
		t.Error("expected listening")
	}

	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Double stop should fail.
	if err := svc.Stop(ctx); err == nil {
		t.Error("expected error on double stop")
	}
}

func TestProxyServeHTTP(t *testing.T) {
	// Set up a test origin.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "origin response")
	}))
	defer origin.Close()

	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:      siteID,
					Name:    "test-site",
					Domains: []string{"test.example.com"},
					TLSMode: domain.TLSModeDisabled,
					Status:  domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  siteID,
						Address: origin.Listener.Addr().String(),
						Scheme:  domain.CDNOriginHTTP,
						Weight:  10,
					},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Make request to proxy.
	req, _ := http.NewRequest("GET", "http://"+addr+"/hello", nil)
	req.Host = "test.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "origin response" {
		t.Errorf("expected 'origin response', got %q", string(body))
	}
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("expected X-Cache MISS, got %q", resp.Header.Get("X-Cache"))
	}
}

func TestProxyCacheHitMiss(t *testing.T) {
	cachedCallCount := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cached" {
			cachedCallCount++
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "response-%d", cachedCallCount)
			return
		}
		// Health check or other paths.
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:           siteID,
					Name:         "cache-test",
					Domains:      []string{"cache.example.com"},
					TLSMode:      domain.TLSModeDisabled,
					CacheEnabled: true,
					CacheTTL:     60,
					Status:       domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  siteID,
						Address: origin.Listener.Addr().String(),
						Scheme:  domain.CDNOriginHTTP,
						Weight:  10,
					},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// First request — cache miss.
	req, _ := http.NewRequest("GET", "http://"+addr+"/cached", nil)
	req.Host = "cache.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	body1, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("first request: expected MISS, got %q", resp.Header.Get("X-Cache"))
	}

	// Second request — cache hit (same body).
	req2, _ := http.NewRequest("GET", "http://"+addr+"/cached", nil)
	req2.Host = "cache.example.com"

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("second request: expected HIT, got %q", resp2.Header.Get("X-Cache"))
	}

	if string(body1) != string(body2) {
		t.Errorf("expected same body, got %q and %q", string(body1), string(body2))
	}

	// Origin's /cached endpoint should have been called only once.
	if cachedCallCount != 1 {
		t.Errorf("expected 1 origin call to /cached, got %d", cachedCallCount)
	}
}

func TestProxyRateLimit(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	rps := 3
	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:           siteID,
					Name:         "rl-test",
					Domains:      []string{"rl.example.com"},
					TLSMode:      domain.TLSModeDisabled,
					RateLimitRPS: &rps,
					Status:       domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  siteID,
						Address: origin.Listener.Addr().String(),
						Scheme:  domain.CDNOriginHTTP,
						Weight:  10,
					},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Send more requests than the rate limit.
	rateLimited := 0
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", "http://"+addr+"/test", nil)
		req.Host = "rl.example.com"

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimited++
		}
	}

	if rateLimited == 0 {
		t.Error("expected some requests to be rate limited")
	}
}

func TestProxyHeaderRules(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "OriginServer/1.0")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer origin.Close()

	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	rules := []cdn.HeaderRule{
		{Action: "set", Header: "X-CDN", Value: "edgefabric"},
		{Action: "remove", Header: "Server"},
	}
	rulesJSON, _ := json.Marshal(rules)

	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:          siteID,
					Name:        "header-test",
					Domains:     []string{"header.example.com"},
					TLSMode:     domain.TLSModeDisabled,
					HeaderRules: rulesJSON,
					Status:      domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  siteID,
						Address: origin.Listener.Addr().String(),
						Scheme:  domain.CDNOriginHTTP,
						Weight:  10,
					},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://"+addr+"/test", nil)
	req.Host = "header.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.Header.Get("X-CDN") != "edgefabric" {
		t.Errorf("expected X-CDN: edgefabric, got %q", resp.Header.Get("X-CDN"))
	}
	if resp.Header.Get("Server") != "" {
		t.Errorf("expected Server header to be removed, got %q", resp.Header.Get("Server"))
	}
}

func TestProxyCompression(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Write enough data to make compression worthwhile.
		fmt.Fprint(w, strings.Repeat("hello world ", 100))
	}))
	defer origin.Close()

	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:                 siteID,
					Name:               "compress-test",
					Domains:            []string{"compress.example.com"},
					TLSMode:            domain.TLSModeDisabled,
					CompressionEnabled: true,
					Status:             domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  siteID,
						Address: origin.Listener.Addr().String(),
						Scheme:  domain.CDNOriginHTTP,
						Weight:  10,
					},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Create a transport that doesn't auto-decompress.
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	req, _ := http.NewRequest("GET", "http://"+addr+"/test", nil)
	req.Host = "compress.example.com"
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("expected Content-Encoding: gzip, got %q", resp.Header.Get("Content-Encoding"))
	}

	// Decompress and verify content.
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, _ := io.ReadAll(gz)
	gz.Close()

	expected := strings.Repeat("hello world ", 100)
	if string(body) != expected {
		t.Errorf("decompressed body length %d, expected %d", len(body), len(expected))
	}
}

func TestProxyUnknownHost(t *testing.T) {
	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	req, _ := http.NewRequest("GET", "http://"+addr+"/test", nil)
	req.Host = "unknown.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 for unknown host, got %d", resp.StatusCode)
	}
}

func TestProxyGetStatus(t *testing.T) {
	svc := NewProxyService(nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if !status.Listening {
		t.Error("expected listening")
	}
	if status.SiteCount != 0 {
		t.Errorf("expected 0 sites, got %d", status.SiteCount)
	}
	if status.RequestsTotal != 0 {
		t.Errorf("expected 0 requests, got %d", status.RequestsTotal)
	}

	// Make a request to an unknown host to increment counter.
	req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
	req.Host = "nope.com"
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Allow time for atomic update.
	time.Sleep(10 * time.Millisecond)

	status, _ = svc.GetStatus(ctx)
	if status.RequestsTotal != 1 {
		t.Errorf("expected 1 request, got %d", status.RequestsTotal)
	}
}
