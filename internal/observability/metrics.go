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
