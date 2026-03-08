package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
)

func TestRequestIDGenerated(t *testing.T) {
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.RequestIDFromContext(r.Context())
		if id == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID response header")
	}
}

func TestRequestIDPreserved(t *testing.T) {
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.RequestIDFromContext(r.Context())
		if id != "custom-request-id-123" {
			t.Errorf("expected custom ID, got %s", id)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-request-id-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "custom-request-id-123" {
		t.Errorf("expected custom ID in response, got %s", rec.Header().Get("X-Request-ID"))
	}
}
