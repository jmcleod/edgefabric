package plugin

import (
	"log/slog"

	"github.com/jmcleod/edgefabric/internal/bgp"
	"github.com/jmcleod/edgefabric/internal/cdnserver"
	"github.com/jmcleod/edgefabric/internal/dnsserver"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/routeserver"
)

func init() {
	// BGP plugins.
	Register(PluginTypeBGP, "gobgp", BGPFactory(func() bgp.Service {
		return bgp.NewGoBGPService()
	}))
	Register(PluginTypeBGP, "noop", BGPFactory(func() bgp.Service {
		return bgp.NewNoopService()
	}))

	// DNS plugins.
	Register(PluginTypeDNS, "miekg", DNSFactory(func(logger *slog.Logger, metrics *observability.Metrics, axfrEnabled bool) dnsserver.Service {
		return dnsserver.NewMiekgService(logger, metrics, axfrEnabled)
	}))
	Register(PluginTypeDNS, "noop", DNSFactory(func(_ *slog.Logger, _ *observability.Metrics, _ bool) dnsserver.Service {
		return dnsserver.NewNoopService()
	}))

	// CDN plugins.
	Register(PluginTypeCDN, "proxy", CDNFactory(func(logger *slog.Logger, metrics *observability.Metrics) cdnserver.Service {
		return cdnserver.NewProxyService(logger, metrics)
	}))
	Register(PluginTypeCDN, "noop", CDNFactory(func(_ *slog.Logger, _ *observability.Metrics) cdnserver.Service {
		return cdnserver.NewNoopService()
	}))

	// Route plugins.
	Register(PluginTypeRoute, "forwarder", RouteFactory(func(logger *slog.Logger, metrics *observability.Metrics) routeserver.Service {
		return routeserver.NewForwarderService(logger, metrics)
	}))
	Register(PluginTypeRoute, "noop", RouteFactory(func(_ *slog.Logger, _ *observability.Metrics) routeserver.Service {
		return routeserver.NewNoopService()
	}))
}
