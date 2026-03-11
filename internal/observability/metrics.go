package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds global Prometheus metrics for EdgeFabric.
type Metrics struct {
	Registry *prometheus.Registry

	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPResponseSize     *prometheus.HistogramVec

	// System metrics (controller-level fleet gauges)
	ActiveNodes    prometheus.Gauge
	ActiveGateways prometheus.Gauge
	ActiveTenants  prometheus.Gauge

	// Route forwarder metrics (node + gateway)
	RouteConnectionsActive prometheus.Gauge
	RouteBytesForwarded    prometheus.Counter
	RouteListenersActive   *prometheus.GaugeVec // labels: {role, protocol}

	// CDN metrics
	CDNCacheHits    prometheus.Counter
	CDNCacheMisses  prometheus.Counter
	CDNRequestsTotal prometheus.Counter

	// DNS metrics
	DNSQueriesTotal prometheus.Counter
	DNSZonesActive  prometheus.Gauge

	// Auth metrics
	AuthFailuresTotal *prometheus.CounterVec // labels: {type} = login, totp, api_key

	// DNS query monitoring (node-side, Milestone 11.4)
	DNSQueryDuration  *prometheus.HistogramVec // labels: {zone, qtype, rcode}
	DNSQueriesByZone  *prometheus.CounterVec   // labels: {zone, qtype, rcode}

	// BGP session monitoring (node-side, Milestone 11.2)
	BGPSessionState           *prometheus.GaugeVec   // labels: {peer_address, peer_asn}
	BGPSessionTransitionsTotal *prometheus.CounterVec // labels: {peer_address, transition}

	// Route health monitoring (node-side, Milestone 11.3)
	RouteHealthChecksTotal *prometheus.CounterVec   // labels: {route_id, route_name, result}
	RouteHealthy           *prometheus.GaugeVec     // labels: {route_id, route_name}

	// Overlay health monitoring (node-side, Milestone 11.1)
	OverlayHealthChecksTotal *prometheus.CounterVec // labels: {peer, result}
	OverlayPeerHealthy       *prometheus.GaugeVec   // labels: {peer}

	// WAF metrics (node-side, Milestone 12.3)
	WAFMatchesTotal      *prometheus.CounterVec // labels: {category, action}
	WAFRequestsInspected prometheus.Counter
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: reg,

		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "edgefabric",
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latency in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "edgefabric",
				Name:      "http_response_size_bytes",
				Help:      "HTTP response size in bytes.",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
			},
			[]string{"method", "path"},
		),

		ActiveNodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "edgefabric",
			Name:      "active_nodes",
			Help:      "Number of currently active nodes.",
		}),
		ActiveGateways: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "edgefabric",
			Name:      "active_gateways",
			Help:      "Number of currently active gateways.",
		}),
		ActiveTenants: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "edgefabric",
			Name:      "active_tenants",
			Help:      "Number of currently active tenants.",
		}),

		RouteConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "edgefabric",
			Name:      "route_connections_active",
			Help:      "Number of currently active route forwarding connections.",
		}),
		RouteBytesForwarded: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "route_bytes_forwarded_total",
			Help:      "Total bytes forwarded through route forwarders.",
		}),
		RouteListenersActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "edgefabric",
				Name:      "route_listeners_active",
				Help:      "Number of active route listeners.",
			},
			[]string{"role", "protocol"},
		),

		CDNCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "cdn_cache_hits_total",
			Help:      "Total CDN cache hits.",
		}),
		CDNCacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "cdn_cache_misses_total",
			Help:      "Total CDN cache misses.",
		}),
		CDNRequestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "cdn_requests_total",
			Help:      "Total CDN proxy requests served.",
		}),

		DNSQueriesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "dns_queries_total",
			Help:      "Total DNS queries served.",
		}),
		DNSZonesActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "edgefabric",
			Name:      "dns_zones_active",
			Help:      "Number of active DNS zones loaded.",
		}),

		AuthFailuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "auth_failures_total",
				Help:      "Total authentication failures by type (login, totp, api_key).",
			},
			[]string{"type"},
		),

		// DNS query monitoring.
		DNSQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "edgefabric",
				Name:      "dns_query_duration_seconds",
				Help:      "DNS query processing duration in seconds.",
				Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
			},
			[]string{"zone", "qtype", "rcode"},
		),
		DNSQueriesByZone: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "dns_queries_by_zone_total",
				Help:      "Total DNS queries by zone, query type, and response code.",
			},
			[]string{"zone", "qtype", "rcode"},
		),

		// BGP session monitoring.
		BGPSessionState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "edgefabric",
				Name:      "bgp_session_state",
				Help:      "BGP session state (1=established, 0=not established).",
			},
			[]string{"peer_address", "peer_asn"},
		),
		BGPSessionTransitionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "bgp_session_transitions_total",
				Help:      "Total BGP session state transitions.",
			},
			[]string{"peer_address", "transition"},
		),

		// Route health monitoring.
		RouteHealthChecksTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "route_health_checks_total",
				Help:      "Total route health check probes by result.",
			},
			[]string{"route_id", "route_name", "result"},
		),
		RouteHealthy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "edgefabric",
				Name:      "route_healthy",
				Help:      "Route health status (1=healthy, 0=unhealthy).",
			},
			[]string{"route_id", "route_name"},
		),

		// Overlay health monitoring.
		OverlayHealthChecksTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "overlay_health_checks_total",
				Help:      "Total overlay peer health check probes by result.",
			},
			[]string{"peer", "result"},
		),
		OverlayPeerHealthy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "edgefabric",
				Name:      "overlay_peer_healthy",
				Help:      "Overlay peer health status (1=healthy, 0=unhealthy).",
			},
			[]string{"peer"},
		),

		// WAF metrics.
		WAFMatchesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "edgefabric",
				Name:      "waf_matches_total",
				Help:      "Total WAF rule matches by category and action.",
			},
			[]string{"category", "action"},
		),
		WAFRequestsInspected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "edgefabric",
			Name:      "waf_requests_inspected_total",
			Help:      "Total requests inspected by WAF.",
		}),
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPResponseSize,
		m.ActiveNodes,
		m.ActiveGateways,
		m.ActiveTenants,
		m.RouteConnectionsActive,
		m.RouteBytesForwarded,
		m.RouteListenersActive,
		m.CDNCacheHits,
		m.CDNCacheMisses,
		m.CDNRequestsTotal,
		m.DNSQueriesTotal,
		m.DNSZonesActive,
		m.AuthFailuresTotal,
		m.DNSQueryDuration,
		m.DNSQueriesByZone,
		m.BGPSessionState,
		m.BGPSessionTransitionsTotal,
		m.RouteHealthChecksTotal,
		m.RouteHealthy,
		m.OverlayHealthChecksTotal,
		m.OverlayPeerHealthy,
		m.WAFMatchesTotal,
		m.WAFRequestsInspected,
	)

	return m
}

// Handler returns an http.Handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// SystemGaugeUpdater is a function called periodically to refresh system-level gauges.
type SystemGaugeUpdater func(m *Metrics)

// StartGaugeUpdater runs a background goroutine that periodically calls the
// updater function to refresh system-level gauges (active nodes, gateways, tenants).
// It runs until the context is cancelled.
func StartGaugeUpdater(ctx context.Context, m *Metrics, interval time.Duration, updater SystemGaugeUpdater) {
	go func() {
		// Run once immediately on startup.
		updater(m)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updater(m)
			}
		}
	}()
}
