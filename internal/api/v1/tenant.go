// Package v1 implements the EdgeFabric REST API v1 handlers.
package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/tenant"
)

// TenantHandler handles tenant CRUD endpoints.
type TenantHandler struct {
	svc        tenant.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewTenantHandler creates a new tenant handler.
func NewTenantHandler(svc tenant.Service, authorizer rbac.Authorizer, audit audit.Logger) *TenantHandler {
	return &TenantHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts tenant routes on the mux.
func (h *TenantHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceTenant, nil)
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceTenant, nil)
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceTenant, nil)
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceTenant, nil)
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceTenant, nil)

	mux.Handle("POST /api/v1/tenants", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/tenants", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/tenants/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/tenants/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/tenants/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/tenants.
func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req tenant.CreateRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	t, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "tenant slug already exists")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "tenant",
		Details:  map[string]string{"tenant_id": t.ID.String(), "slug": t.Slug},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, t)
}

// Get handles GET /api/v1/tenants/{id}.
func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	t, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "tenant not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get tenant")
		return
	}

	apiutil.JSON(w, http.StatusOK, t)
}

// List handles GET /api/v1/tenants.
func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)

	tenants, total, err := h.svc.List(r.Context(), params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list tenants")
		return
	}

	apiutil.ListJSON(w, tenants, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/tenants/{id}.
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req tenant.UpdateRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	t, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "tenant not found")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "update",
		Resource: "tenant",
		Details:  map[string]string{"tenant_id": t.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, t)
}

// Delete handles DELETE /api/v1/tenants/{id}.
func (h *TenantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "tenant not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete tenant")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "tenant",
		Details:  map[string]string{"tenant_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
