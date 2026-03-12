package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestRequireResourceOwnerOrAdmin_ReadonlyOwnResource(t *testing.T) {
	nodeID := domain.NewID()
	tenantID := domain.NewID()
	claims := &auth.Claims{
		UserID:   nodeID,
		TenantID: &tenantID,
		Role:     domain.RoleReadOnly,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequireResourceOwnerOrAdmin("id")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/nodes/{id}/config/wireguard", mw(handler))

	req := httptest.NewRequest("GET", "/api/v1/nodes/"+nodeID.String()+"/config/wireguard", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestRequireResourceOwnerOrAdmin_ReadonlyDifferentResource(t *testing.T) {
	nodeID := domain.NewID()
	otherNodeID := domain.NewID()
	tenantID := domain.NewID()
	claims := &auth.Claims{
		UserID:   nodeID,
		TenantID: &tenantID,
		Role:     domain.RoleReadOnly,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.RequireResourceOwnerOrAdmin("id")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/nodes/{id}/config/wireguard", mw(handler))

	req := httptest.NewRequest("GET", "/api/v1/nodes/"+otherNodeID.String()+"/config/wireguard", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireResourceOwnerOrAdmin_AdminBypass(t *testing.T) {
	adminUserID := domain.NewID()
	targetNodeID := domain.NewID()
	tenantID := domain.NewID()
	claims := &auth.Claims{
		UserID:   adminUserID,
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequireResourceOwnerOrAdmin("id")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/nodes/{id}/config/wireguard", mw(handler))

	req := httptest.NewRequest("GET", "/api/v1/nodes/"+targetNodeID.String()+"/config/wireguard", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestRequireResourceOwnerOrAdmin_SuperuserBypass(t *testing.T) {
	superUserID := domain.NewID()
	targetNodeID := domain.NewID()
	claims := &auth.Claims{
		UserID:   superUserID,
		TenantID: nil, // superuser
		Role:     domain.RoleSuperUser,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequireResourceOwnerOrAdmin("id")

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/nodes/{id}/config/wireguard", mw(handler))

	req := httptest.NewRequest("GET", "/api/v1/nodes/"+targetNodeID.String()+"/config/wireguard", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}
