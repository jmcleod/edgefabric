// Package cdnserver implements the node-side CDN reverse proxy server.
// It receives site/origin configuration from the controller via the CDN
// service's NodeCDNConfig and serves HTTP traffic through reverse proxies
// with in-memory caching, compression, rate limiting, and health checks.
package cdnserver

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// Service is the node-side CDN server runtime interface.
// Implementations manage the local reverse proxy and reconcile the desired
// site/origin state from the controller into active proxy configuration.
type Service interface {
	// Start initializes the CDN server and begins listening.
	Start(ctx context.Context, listenAddr string) error

	// Stop shuts down the CDN server gracefully.
	Stop(ctx context.Context) error

	// Reconcile loads the desired CDN configuration from the controller
	// and updates the local proxy routing to match.
	Reconcile(ctx context.Context, config *cdn.NodeCDNConfig) error

	// PurgeCache evicts all cached entries for a given site.
	PurgeCache(ctx context.Context, siteID domain.ID) error

	// GetStatus returns the current runtime state of the CDN server.
	GetStatus(ctx context.Context) (*ServerStatus, error)
}

// ServerStatus represents the runtime status of the CDN server.
type ServerStatus struct {
	Listening     bool   `json:"listening"`
	ListenAddr    string `json:"listen_addr"`
	SiteCount     int    `json:"site_count"`
	CacheHits     uint64 `json:"cache_hits"`
	CacheMisses   uint64 `json:"cache_misses"`
	CacheEntries  uint64 `json:"cache_entries"`
	RequestsTotal uint64 `json:"requests_total"`
}
