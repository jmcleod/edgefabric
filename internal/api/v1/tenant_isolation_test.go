package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

// TestTenantIsolation_ListNodesFilteredByTenant verifies that a non-superuser
// only sees their own tenant's nodes.
func TestTenantIsolation_ListNodesFilteredByTenant(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()

	tenantA := domain.NewID()
	tenantB := domain.NewID()

	// Pre-populate fleet with nodes from two tenants.
	fleetSvc := &mockFleetService{
		nodes: []*domain.Node{
			{ID: domain.NewID(), TenantID: &tenantA, Name: "node-a1", Status: domain.NodeStatusOnline},
			{ID: domain.NewID(), TenantID: &tenantA, Name: "node-a2", Status: domain.NodeStatusOnline},
			{ID: domain.NewID(), TenantID: &tenantB, Name: "node-b1", Status: domain.NodeStatusOnline},
		},
	}

	handler := NewNodeHandler(fleetSvc, authorizer, auditLog)

	// Tenant A admin should only see their 2 nodes.
	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantA,
		Role:     domain.RoleAdmin,
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("tenant A should see 2 nodes, got %d", len(resp.Data))
	}
}

// TestTenantIsolation_ListNodesSuperUserSeesAll verifies that a superuser
// sees all nodes across all tenants.
func TestTenantIsolation_ListNodesSuperUserSeesAll(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()

	tenantA := domain.NewID()
	tenantB := domain.NewID()

	fleetSvc := &mockFleetService{
		nodes: []*domain.Node{
			{ID: domain.NewID(), TenantID: &tenantA, Name: "node-a1"},
			{ID: domain.NewID(), TenantID: &tenantB, Name: "node-b1"},
		},
	}

	handler := NewNodeHandler(fleetSvc, authorizer, auditLog)

	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleSuperUser,
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data) != 2 {
		t.Errorf("superuser should see all 2 nodes, got %d", len(resp.Data))
	}
}

// TestTenantIsolation_ReadOnlyCantCreate verifies RBAC enforcement:
// ReadOnly users cannot create nodes.
func TestTenantIsolation_ReadOnlyCantCreate(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()
	tokenSvc := newTestTokenService()

	fleetSvc := &mockFleetService{}
	handler := NewNodeHandler(fleetSvc, authorizer, auditLog)

	mux := http.NewServeMux()
	authMW := middleware.Auth(tokenSvc, nil)
	handler.Register(mux, authMW)

	tenantID := domain.NewID()
	token, _ := tokenSvc.Issue(auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleReadOnly,
	})

	req := httptest.NewRequest("POST", "/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for readonly user, got %d", w.Code)
	}
}

// TestTenantIsolation_AdminCanList verifies RBAC enforcement:
// Admin users can list nodes.
func TestTenantIsolation_AdminCanList(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()
	tokenSvc := newTestTokenService()

	fleetSvc := &mockFleetService{}
	handler := NewNodeHandler(fleetSvc, authorizer, auditLog)

	mux := http.NewServeMux()
	authMW := middleware.Auth(tokenSvc, nil)
	handler.Register(mux, authMW)

	tenantID := domain.NewID()
	token, _ := tokenSvc.Issue(auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	})

	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for admin user, got %d: %s", w.Code, w.Body.String())
	}
}

// TestTenantIsolation_Unauthenticated verifies that unauthenticated
// requests to protected endpoints return 401.
func TestTenantIsolation_Unauthenticated(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()
	tokenSvc := newTestTokenService()

	fleetSvc := &mockFleetService{}
	handler := NewNodeHandler(fleetSvc, authorizer, auditLog)

	mux := http.NewServeMux()
	authMW := middleware.Auth(tokenSvc, nil)
	handler.Register(mux, authMW)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/nodes"},
		{"POST", "/api/v1/nodes"},
		{"GET", "/api/v1/nodes/" + domain.NewID().String()},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}
