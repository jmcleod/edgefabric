package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jmcleod/edgefabric/internal/observability"
)

// Metrics returns middleware that records Prometheus HTTP metrics.
func Metrics(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			duration := time.Since(start).Seconds()

			// Use the route pattern (Go 1.22+) for low-cardinality labels.
			// Falls back to a generic path to prevent label explosion.
			path := r.Pattern
			if path == "" {
				path = "unmatched"
			}

			m.HTTPRequestsTotal.WithLabelValues(r.Method, path, fmt.Sprintf("%d", sw.status)).Inc()
			m.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
			m.HTTPResponseSize.WithLabelValues(r.Method, path).Observe(float64(sw.bytes))
		})
	}
}
