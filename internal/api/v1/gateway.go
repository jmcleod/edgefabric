package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// GatewayHandler handles gateway CRUD endpoints.
type GatewayHandler struct {
	svc        fleet.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewGatewayHandler creates a new gateway handler.
func NewGatewayHandler(svc fleet.Service, authorizer rbac.Authorizer, audit audit.Logger) *GatewayHandler {
	return &GatewayHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts gateway routes on the mux.
func (h *GatewayHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceGateway, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceGateway, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceGateway, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceGateway, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceGateway, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/gateways", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/gateways", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/gateways/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/gateways/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/gateways/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
	mux.Handle("POST /api/v1/gateways/{id}/heartbeat", middleware.Chain(http.HandlerFunc(h.Heartbeat), authMW, requireUpdate))
}

// Create handles POST /api/v1/gateways.
func (h *GatewayHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req fleet.CreateGatewayRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	gw, err := h.svc.CreateGateway(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "gateway already exists")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "gateway",
		Details:  map[string]string{"gateway_id": gw.ID.String(), "name": gw.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, gw)
}

// Get handles GET /api/v1/gateways/{id}.
func (h *GatewayHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	gw, err := h.svc.GetGateway(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "gateway not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get gateway")
		return
	}

	apiutil.JSON(w, http.StatusOK, gw)
}

// List handles GET /api/v1/gateways.
func (h *GatewayHandler) List(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)
	claims := middleware.ClaimsFromContext(r.Context())

	// Use tenant from claims for non-superuser.
	tenantID := claims.TenantID
	if tenantID == nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "tenant_id required")
		return
	}

	gateways, total, err := h.svc.ListGateways(r.Context(), *tenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list gateways")
		return
	}

	apiutil.ListJSON(w, gateways, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/gateways/{id}.
func (h *GatewayHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req fleet.UpdateGatewayRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	gw, err := h.svc.UpdateGateway(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "gateway not found")
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
		Resource: "gateway",
		Details:  map[string]string{"gateway_id": gw.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, gw)
}

// Delete handles DELETE /api/v1/gateways/{id}.
func (h *GatewayHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteGateway(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "gateway not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete gateway")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "gateway",
		Details:  map[string]string{"gateway_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// Heartbeat handles POST /api/v1/gateways/{id}/heartbeat.
func (h *GatewayHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.RecordGatewayHeartbeat(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "gateway not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to record heartbeat")
		return
	}

	apiutil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
