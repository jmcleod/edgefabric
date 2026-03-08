package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivezHandler(t *testing.T) {
	handler := LivezHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/livez", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestReadyzHandler_AllPassing(t *testing.T) {
	hc := NewHealthChecker()
	hc.Register(HealthCheck{
		Name:  "test",
		Check: func(ctx context.Context) error { return nil },
	})

	handler := hc.ReadyzHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/readyz", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != HealthOK {
		t.Errorf("expected status=ok, got %q", result.Status)
	}
}

func TestReadyzHandler_Degraded(t *testing.T) {
	hc := NewHealthChecker()
	hc.Register(HealthCheck{
		Name:  "pass",
		Check: func(ctx context.Context) error { return nil },
	})
	hc.Register(HealthCheck{
		Name:  "fail",
		Check: func(ctx context.Context) error { return fmt.Errorf("database unreachable") },
	})

	handler := hc.ReadyzHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/readyz", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var result HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Status != HealthDegraded {
		t.Errorf("expected status=degraded, got %q", result.Status)
	}
}

func TestHealthChecker_Evaluate_AllDown(t *testing.T) {
	hc := NewHealthChecker()
	hc.Register(HealthCheck{
		Name:  "svc1",
		Check: func(ctx context.Context) error { return fmt.Errorf("down") },
	})
	hc.Register(HealthCheck{
		Name:  "svc2",
		Check: func(ctx context.Context) error { return fmt.Errorf("down") },
	})

	result := hc.Evaluate(context.Background())
	if result.Status != HealthDown {
		t.Errorf("expected status=down when all checks fail, got %q", result.Status)
	}
}

func TestHealthChecker_Evaluate_NoChecks(t *testing.T) {
	hc := NewHealthChecker()
	result := hc.Evaluate(context.Background())
	if result.Status != HealthOK {
		t.Errorf("expected status=ok with no checks, got %q", result.Status)
	}
}
