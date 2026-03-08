package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// IPAllocationHandler handles IP allocation CRUD endpoints.
type IPAllocationHandler struct {
	svc        networking.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewIPAllocationHandler creates a new IP allocation handler.
func NewIPAllocationHandler(svc networking.Service, authorizer rbac.Authorizer, audit audit.Logger) *IPAllocationHandler {
	return &IPAllocationHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts IP allocation routes on the mux.
func (h *IPAllocationHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceIPAllocation, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceIPAllocation, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceIPAllocation, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceIPAllocation, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceIPAllocation, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/tenants/{tenantId}/ip-allocations", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/tenants/{tenantId}/ip-allocations", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/ip-allocations/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/ip-allocations/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/ip-allocations/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/tenants/{tenantId}/ip-allocations.
func (h *IPAllocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req networking.CreateIPAllocationRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.TenantID = tenantID

	alloc, err := h.svc.CreateIPAllocation(r.Context(), req)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "ip_allocation",
		Details:  map[string]string{"allocation_id": alloc.ID.String(), "prefix": alloc.Prefix},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, alloc)
}

// Get handles GET /api/v1/ip-allocations/{id}.
func (h *IPAllocationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	alloc, err := h.svc.GetIPAllocation(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "ip allocation not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get ip allocation")
		return
	}

	apiutil.JSON(w, http.StatusOK, alloc)
}

// List handles GET /api/v1/tenants/{tenantId}/ip-allocations.
func (h *IPAllocationHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	allocations, total, err := h.svc.ListIPAllocations(r.Context(), tenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list ip allocations")
		return
	}

	apiutil.ListJSON(w, allocations, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/ip-allocations/{id}.
func (h *IPAllocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req networking.UpdateIPAllocationRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	alloc, err := h.svc.UpdateIPAllocation(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "ip allocation not found")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "update",
		Resource: "ip_allocation",
		Details:  map[string]string{"allocation_id": alloc.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, alloc)
}

// Delete handles DELETE /api/v1/ip-allocations/{id}.
func (h *IPAllocationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteIPAllocation(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "ip allocation not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete ip allocation")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "ip_allocation",
		Details:  map[string]string{"allocation_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
