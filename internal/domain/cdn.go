package domain

import (
	"encoding/json"
	"time"
)

// CDNSiteStatus represents whether a CDN site is serving.
type CDNSiteStatus string

const (
	CDNSiteActive   CDNSiteStatus = "active"
	CDNSiteDisabled CDNSiteStatus = "disabled"
)

// TLSMode determines how TLS is handled for a CDN site.
type TLSMode string

const (
	TLSModeAuto     TLSMode = "auto"
	TLSModeManual   TLSMode = "manual"
	TLSModeDisabled TLSMode = "disabled"
)

// CDNOriginStatus represents the health of an origin server.
type CDNOriginStatus string

const (
	CDNOriginHealthy   CDNOriginStatus = "healthy"
	CDNOriginUnhealthy CDNOriginStatus = "unhealthy"
	CDNOriginUnknown   CDNOriginStatus = "unknown"
)

// CDNOriginScheme is the protocol used to reach an origin.
type CDNOriginScheme string

const (
	CDNOriginHTTP  CDNOriginScheme = "http"
	CDNOriginHTTPS CDNOriginScheme = "https"
)

// CDNSite is a CDN site configuration (reverse proxy).
type CDNSite struct {
	ID                 ID              `json:"id" db:"id"`
	TenantID           ID              `json:"tenant_id" db:"tenant_id"`
	Name               string          `json:"name" db:"name"`
	Domains            []string        `json:"domains" db:"-"`
	TLSMode            TLSMode         `json:"tls_mode" db:"tls_mode"`
	TLSCertID          *ID             `json:"tls_cert_id,omitempty" db:"tls_cert_id"`
	CacheEnabled       bool            `json:"cache_enabled" db:"cache_enabled"`
	CacheTTL           int             `json:"cache_ttl" db:"cache_ttl"`
	CompressionEnabled bool            `json:"compression_enabled" db:"compression_enabled"`
	RateLimitRPS       *int            `json:"rate_limit_rps,omitempty" db:"rate_limit_rps"`
	NodeGroupID        *ID             `json:"node_group_id,omitempty" db:"node_group_id"`
	HeaderRules        json.RawMessage `json:"header_rules,omitempty" db:"header_rules"`
	Status             CDNSiteStatus   `json:"status" db:"status"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

// CDNOrigin is a backend origin server for a CDN site.
type CDNOrigin struct {
	ID                  ID              `json:"id" db:"id"`
	SiteID              ID              `json:"site_id" db:"site_id"`
	Address             string          `json:"address" db:"address"`
	Scheme              CDNOriginScheme `json:"scheme" db:"scheme"`
	Weight              int             `json:"weight" db:"weight"`
	HealthCheckPath     string          `json:"health_check_path,omitempty" db:"health_check_path"`
	HealthCheckInterval *int            `json:"health_check_interval,omitempty" db:"health_check_interval"`
	Status              CDNOriginStatus `json:"status" db:"status"`
	CreatedAt           time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
}
