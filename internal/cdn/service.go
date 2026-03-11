// Package cdn implements the controller-side CDN site and origin management
// service. It validates CDN configurations, manages site/origin lifecycle,
// and provides configuration snapshots for node-side CDN proxies to poll.
package cdn

import (
	"context"
	"encoding/json"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the controller-side CDN management interface.
type Service interface {
	// Site CRUD.
	CreateSite(ctx context.Context, req CreateSiteRequest) (*domain.CDNSite, error)
	GetSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error)
	ListSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error)
	UpdateSite(ctx context.Context, id domain.ID, req UpdateSiteRequest) (*domain.CDNSite, error)
	DeleteSite(ctx context.Context, id domain.ID) error

	// Origin CRUD.
	CreateOrigin(ctx context.Context, req CreateOriginRequest) (*domain.CDNOrigin, error)
	GetOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error)
	ListOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error)
	UpdateOrigin(ctx context.Context, id domain.ID, req UpdateOriginRequest) (*domain.CDNOrigin, error)
	DeleteOrigin(ctx context.Context, id domain.ID) error

	// Cache purge — signals cache invalidation for a site.
	PurgeSiteCache(ctx context.Context, siteID domain.ID) error

	// Config sync — returns CDN config for a specific node.
	GetNodeCDNConfig(ctx context.Context, nodeID domain.ID) (*NodeCDNConfig, error)
}

// CreateSiteRequest is the input for creating a CDN site.
type CreateSiteRequest struct {
	TenantID           domain.ID        `json:"tenant_id"`
	Name               string           `json:"name"`
	Domains            []string         `json:"domains"`
	TLSMode            domain.TLSMode   `json:"tls_mode"`
	CacheEnabled       bool             `json:"cache_enabled"`
	CacheTTL           int              `json:"cache_ttl,omitempty"`
	CompressionEnabled bool             `json:"compression_enabled"`
	RateLimitRPS       *int             `json:"rate_limit_rps,omitempty"`
	NodeGroupID        *domain.ID       `json:"node_group_id,omitempty"`
	HeaderRules        json.RawMessage  `json:"header_rules,omitempty"`
	WAFEnabled         bool             `json:"waf_enabled"`
	WAFMode            string           `json:"waf_mode,omitempty"`
}

// UpdateSiteRequest is the input for updating a CDN site.
type UpdateSiteRequest struct {
	Name               *string            `json:"name,omitempty"`
	Domains            []string           `json:"domains,omitempty"`
	TLSMode            *domain.TLSMode    `json:"tls_mode,omitempty"`
	CacheEnabled       *bool              `json:"cache_enabled,omitempty"`
	CacheTTL           *int               `json:"cache_ttl,omitempty"`
	CompressionEnabled *bool              `json:"compression_enabled,omitempty"`
	RateLimitRPS       *int               `json:"rate_limit_rps,omitempty"`
	NodeGroupID        *domain.ID         `json:"node_group_id,omitempty"`
	HeaderRules        json.RawMessage    `json:"header_rules,omitempty"`
	WAFEnabled         *bool              `json:"waf_enabled,omitempty"`
	WAFMode            *string            `json:"waf_mode,omitempty"`
	Status             *domain.CDNSiteStatus `json:"status,omitempty"`
	// ClearNodeGroup explicitly removes the node group assignment when true.
	ClearNodeGroup bool `json:"clear_node_group,omitempty"`
	// ClearRateLimit explicitly removes the rate limit when true.
	ClearRateLimit bool `json:"clear_rate_limit,omitempty"`
}

// CreateOriginRequest is the input for creating a CDN origin.
type CreateOriginRequest struct {
	SiteID              domain.ID          `json:"site_id"`
	Address             string             `json:"address"`
	Scheme              domain.CDNOriginScheme `json:"scheme"`
	Weight              int                `json:"weight,omitempty"`
	HealthCheckPath     string             `json:"health_check_path,omitempty"`
	HealthCheckInterval *int               `json:"health_check_interval,omitempty"`
}

// UpdateOriginRequest is the input for updating a CDN origin.
type UpdateOriginRequest struct {
	Address             *string                `json:"address,omitempty"`
	Scheme              *domain.CDNOriginScheme `json:"scheme,omitempty"`
	Weight              *int                   `json:"weight,omitempty"`
	HealthCheckPath     *string                `json:"health_check_path,omitempty"`
	HealthCheckInterval *int                   `json:"health_check_interval,omitempty"`
	Status              *domain.CDNOriginStatus `json:"status,omitempty"`
}

// NodeCDNConfig is the full CDN configuration snapshot for a node.
// Nodes poll this to reconcile their local CDN proxy state.
type NodeCDNConfig struct {
	Sites []SiteWithOrigins `json:"sites"`
}

// SiteWithOrigins bundles a CDN site with all its origins for sync.
type SiteWithOrigins struct {
	Site    *domain.CDNSite    `json:"site"`
	Origins []*domain.CDNOrigin `json:"origins"`
}
