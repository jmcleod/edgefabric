package efclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tenants" {
			t.Errorf("expected /api/v1/tenants, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key auth header")
		}

		var req CreateTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Name != "Test Tenant" || req.Slug != "test-tenant" {
			t.Errorf("unexpected request body: %+v", req)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ResourceResponse{ID: "tenant-123"})
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	resp, err := client.CreateTenant(context.Background(), CreateTenantRequest{
		Name: "Test Tenant",
		Slug: "test-tenant",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "tenant-123" {
		t.Errorf("expected ID tenant-123, got %s", resp.ID)
	}
}

func TestDeleteTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tenants/tenant-123" {
			t.Errorf("expected /api/v1/tenants/tenant-123, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	err := client.DeleteTenant(context.Background(), "tenant-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ResourceResponse{ID: "tenant-123"})
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	resp, err := client.GetTenant(context.Background(), "tenant-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "tenant-123" {
		t.Errorf("expected ID tenant-123, got %s", resp.ID)
	}
}

func TestAPIError404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	_, err := client.GetTenant(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestAPIError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	_, err := client.CreateTenant(context.Background(), CreateTenantRequest{Name: "x", Slug: "x"})
	if err == nil {
		t.Fatal("expected error for 500")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
}

func TestCreateNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/nodes" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ResourceResponse{ID: "node-456"})
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	resp, err := client.CreateNode(context.Background(), CreateNodeRequest{
		Name:     "node1",
		Hostname: "node1.example.com",
		PublicIP: "1.2.3.4",
		TenantID: "tenant-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "node-456" {
		t.Errorf("expected ID node-456, got %s", resp.ID)
	}
}

func TestCreateDNSZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tenants/t1/dns/zones" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ResourceResponse{ID: "zone-789"})
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	resp, err := client.CreateDNSZone(context.Background(), "t1", CreateDNSZoneRequest{
		Name:     "example.com",
		TenantID: "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "zone-789" {
		t.Errorf("expected ID zone-789, got %s", resp.ID)
	}
}

func TestUpdateTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tenants/tenant-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL, "test-key")
	err := client.UpdateTenant(context.Background(), "tenant-123", CreateTenantRequest{
		Name: "Updated",
		Slug: "updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
