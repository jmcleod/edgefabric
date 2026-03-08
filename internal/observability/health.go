package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the result of a health check.
type HealthStatus string

const (
	HealthOK       HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthDown     HealthStatus = "down"
)

// HealthCheck is a named health check function.
type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

// HealthChecker manages health check registration and evaluation.
type HealthChecker struct {
	mu     sync.RWMutex
	checks []HealthCheck
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{}
}

// Register adds a health check.
func (h *HealthChecker) Register(check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, check)
}

// HealthResult is the JSON response for a health check.
type HealthResult struct {
	Status    HealthStatus          `json:"status"`
	Checks   map[string]CheckResult `json:"checks"`
	Timestamp time.Time             `json:"timestamp"`
}

// CheckResult is the result of an individual health check.
type CheckResult struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// Evaluate runs all health checks and returns the aggregate result.
func (h *HealthChecker) Evaluate(ctx context.Context) HealthResult {
	h.mu.RLock()
	checks := make([]HealthCheck, len(h.checks))
	copy(checks, h.checks)
	h.mu.RUnlock()

	result := HealthResult{
		Status:    HealthOK,
		Checks:   make(map[string]CheckResult),
		Timestamp: time.Now().UTC(),
	}

	for _, c := range checks {
		cr := CheckResult{Status: HealthOK}
		if err := c.Check(ctx); err != nil {
			cr.Status = HealthDown
			cr.Message = err.Error()
			result.Status = HealthDegraded
		}
		result.Checks[c.Name] = cr
	}

	// If any check is down, overall is degraded (not completely down
	// unless all checks fail).
	allDown := true
	for _, cr := range result.Checks {
		if cr.Status == HealthOK {
			allDown = false
			break
		}
	}
	if allDown && len(result.Checks) > 0 {
		result.Status = HealthDown
	}

	return result
}

// Handler returns an http.Handler for the /healthz endpoint.
func (h *HealthChecker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result := h.Evaluate(ctx)

		w.Header().Set("Content-Type", "application/json")
		if result.Status != HealthOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(result)
	})
}

// ReadyzHandler returns an http.Handler for the /readyz endpoint.
// It delegates to Evaluate(), serving as a readiness probe for orchestrators.
func (h *HealthChecker) ReadyzHandler() http.Handler {
	return h.Handler() // Same logic; readiness = all checks pass.
}

// LivezHandler returns an http.Handler for the /livez endpoint.
// It always returns 200 OK — the process is alive if it can respond.
func LivezHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}
