// Package plugin provides a typed plugin registry for EdgeFabric services.
// Services (BGP, DNS, CDN, Route) register factory functions at init time,
// enabling dynamic service selection based on configuration mode strings.
package plugin

import (
	"log/slog"

	"github.com/jmcleod/edgefabric/internal/bgp"
	"github.com/jmcleod/edgefabric/internal/cdnserver"
	"github.com/jmcleod/edgefabric/internal/dnsserver"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/routeserver"
)

// PluginType identifies the service category a plugin belongs to.
type PluginType string

const (
	PluginTypeBGP   PluginType = "bgp"
	PluginTypeDNS   PluginType = "dns"
	PluginTypeCDN   PluginType = "cdn"
	PluginTypeRoute PluginType = "route"
)

// AllPluginTypes returns all known plugin types in a stable order.
func AllPluginTypes() []PluginType {
	return []PluginType{PluginTypeBGP, PluginTypeDNS, PluginTypeCDN, PluginTypeRoute}
}

// BGPFactory creates a BGP service instance.
type BGPFactory func() bgp.Service

// DNSFactory creates a DNS service instance.
type DNSFactory func(logger *slog.Logger, metrics *observability.Metrics, axfrEnabled bool) dnsserver.Service

// CDNFactory creates a CDN service instance.
type CDNFactory func(logger *slog.Logger, metrics *observability.Metrics) cdnserver.Service

// RouteFactory creates a route forwarding service instance.
type RouteFactory func(logger *slog.Logger, metrics *observability.Metrics) routeserver.Service
