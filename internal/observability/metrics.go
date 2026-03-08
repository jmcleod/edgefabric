package observability

import (
	"net/http"

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

	// System metrics
	ActiveNodes    prometheus.Gauge
	ActiveGateways prometheus.Gauge
	ActiveTenants  prometheus.Gauge
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
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPResponseSize,
		m.ActiveNodes,
		m.ActiveGateways,
		m.ActiveTenants,
	)

	return m
}

// Handler returns an http.Handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
