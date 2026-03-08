package cdnserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestHealthCheckerHealthy(t *testing.T) {
	// Set up a healthy origin.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	origin := &domain.CDNOrigin{
		ID:              domain.NewID(),
		Address:         server.Listener.Addr().String(),
		Scheme:          domain.CDNOriginHTTP,
		HealthCheckPath: "/",
	}

	hc := NewHealthChecker([]*domain.CDNOrigin{origin})
	hc.Start()
	defer hc.Stop()

	// Wait for first check.
	time.Sleep(100 * time.Millisecond)

	healthy := hc.HealthyOrigins()
	if len(healthy) != 1 {
		t.Errorf("expected 1 healthy origin, got %d", len(healthy))
	}
}

func TestHealthCheckerUnhealthy(t *testing.T) {
	// Set up an origin that always fails.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origin := &domain.CDNOrigin{
		ID:              domain.NewID(),
		Address:         server.Listener.Addr().String(),
		Scheme:          domain.CDNOriginHTTP,
		HealthCheckPath: "/health",
	}

	hc := NewHealthChecker([]*domain.CDNOrigin{origin})

	// Manually check multiple times to trigger unhealthy.
	for i := 0; i < unhealthyThreshold+1; i++ {
		hc.checkAll()
	}

	healthy := hc.HealthyOrigins()
	if len(healthy) != 0 {
		t.Errorf("expected 0 healthy origins after failures, got %d", len(healthy))
	}
}

func TestHealthCheckerRecovery(t *testing.T) {
	callCount := 0
	// Set up an origin that fails 3 times then succeeds.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= unhealthyThreshold {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	origin := &domain.CDNOrigin{
		ID:              domain.NewID(),
		Address:         server.Listener.Addr().String(),
		Scheme:          domain.CDNOriginHTTP,
		HealthCheckPath: "/",
	}

	hc := NewHealthChecker([]*domain.CDNOrigin{origin})

	// Drive it to unhealthy.
	for i := 0; i < unhealthyThreshold; i++ {
		hc.checkAll()
	}
	if len(hc.HealthyOrigins()) != 0 {
		t.Error("expected 0 healthy origins after failures")
	}

	// One success should recover (healthyThreshold = 1).
	hc.checkAll()
	if len(hc.HealthyOrigins()) != 1 {
		t.Error("expected 1 healthy origin after recovery")
	}
}

func TestHealthCheckerNoOrigins(t *testing.T) {
	hc := NewHealthChecker(nil)
	healthy := hc.HealthyOrigins()
	if len(healthy) != 0 {
		t.Errorf("expected 0 healthy origins for nil input, got %d", len(healthy))
	}
}
