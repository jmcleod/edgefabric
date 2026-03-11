package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

func TestNew(t *testing.T) {
	c := New("https://ctrl.example.com", "gw-123", "tok-abc")

	if c.baseURL != "https://ctrl.example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://ctrl.example.com")
	}
	if c.gatewayID != "gw-123" {
		t.Errorf("gatewayID = %q, want %q", c.gatewayID, "gw-123")
	}
	if c.apiToken != "tok-abc" {
		t.Errorf("apiToken = %q, want %q", c.apiToken, "tok-abc")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestFetchRouteConfig(t *testing.T) {
	gwID := domain.NewID()
	routeID := domain.NewID()
	tenantID := domain.NewID()
	port := 8080

	wantRoutes := []*domain.Route{
		{
			ID:              routeID,
			TenantID:        tenantID,
			Name:            "web-route",
			Protocol:        domain.RouteProtocolTCP,
			EntryIP:         "10.0.0.1",
			EntryPort:       &port,
			GatewayID:       gwID,
			DestinationIP:   "192.168.1.10",
			DestinationPort: &port,
			Status:          domain.RouteStatusActive,
		},
	}

	wantConfig := route.GatewayRouteConfig{Routes: wantRoutes}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path.
		expectedPath := fmt.Sprintf("/api/v1/gateways/%s/config/routes", gwID)
		if r.URL.Path != expectedPath {
			t.Errorf("request path = %q, want %q", r.URL.Path, expectedPath)
		}
		// Verify method.
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		// Verify auth header.
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}

		data, _ := json.Marshal(wantConfig)
		resp := fmt.Sprintf(`{"data":%s}`, data)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer srv.Close()

	c := New(srv.URL, gwID.String(), "test-token")
	got, err := c.FetchRouteConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchRouteConfig() error = %v", err)
	}

	if len(got.Routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(got.Routes))
	}
	r := got.Routes[0]
	if r.ID != routeID {
		t.Errorf("route ID = %v, want %v", r.ID, routeID)
	}
	if r.Name != "web-route" {
		t.Errorf("route Name = %q, want %q", r.Name, "web-route")
	}
	if r.Protocol != domain.RouteProtocolTCP {
		t.Errorf("route Protocol = %q, want %q", r.Protocol, domain.RouteProtocolTCP)
	}
	if r.DestinationIP != "192.168.1.10" {
		t.Errorf("route DestinationIP = %q, want %q", r.DestinationIP, "192.168.1.10")
	}
	if r.Status != domain.RouteStatusActive {
		t.Errorf("route Status = %q, want %q", r.Status, domain.RouteStatusActive)
	}
}

func TestFetchRouteConfigHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := New(srv.URL, "gw-id", "tok")
	_, err := c.FetchRouteConfig(context.Background())
	if err == nil {
		t.Fatal("FetchRouteConfig() expected error for HTTP 500, got nil")
	}

	// The error should mention the HTTP status code.
	if got := err.Error(); !contains(got, "500") {
		t.Errorf("error = %q, want it to contain %q", got, "500")
	}
}

func TestFetchRouteConfigEmptyData(t *testing.T) {
	t.Run("empty string data", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": ""}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "gw-id", "tok")
		_, err := c.FetchRouteConfig(context.Background())
		if err == nil {
			t.Fatal("FetchRouteConfig() expected error for empty string data, got nil")
		}
	})

	t.Run("missing data field", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "gw-id", "tok")
		_, err := c.FetchRouteConfig(context.Background())
		if err == nil {
			t.Fatal("FetchRouteConfig() expected error for missing data field, got nil")
		}
	})

	t.Run("null data returns zero-value config", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": null}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "gw-id", "tok")
		got, err := c.FetchRouteConfig(context.Background())
		if err != nil {
			t.Fatalf("FetchRouteConfig() unexpected error: %v", err)
		}
		if got.Routes != nil {
			t.Errorf("expected nil Routes for null data, got %v", got.Routes)
		}
	})
}

// contains checks if s contains substr (avoids importing strings for one call).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
