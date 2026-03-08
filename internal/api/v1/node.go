package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// NodeHandler handles node CRUD endpoints.
type NodeHandler struct {
	svc        fleet.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewNodeHandler creates a new node handler.
func NewNodeHandler(svc fleet.Service, authorizer rbac.Authorizer, audit audit.Logger) *NodeHandler {
	return &NodeHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts node routes on the mux.
func (h *NodeHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceNode, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNode, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceNode, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceNode, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceNode, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/nodes", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/nodes", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/nodes/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/nodes/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/nodes/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
	mux.Handle("POST /api/v1/nodes/{id}/heartbeat", middleware.Chain(http.HandlerFunc(h.Heartbeat), authMW, requireUpdate))
}

// Create handles POST /api/v1/nodes.
func (h *NodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req fleet.CreateNodeRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	n, err := h.svc.CreateNode(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "node already exists")
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
		Resource: "node",
		Details:  map[string]string{"node_id": n.ID.String(), "name": n.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, n)
}

// Get handles GET /api/v1/nodes/{id}.
func (h *NodeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	n, err := h.svc.GetNode(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get node")
		return
	}

	// Tenant isolation: non-superuser can only see nodes in their tenant.
	claims := middleware.ClaimsFromContext(r.Context())
	if claims.TenantID != nil && n.TenantID != nil && *claims.TenantID != *n.TenantID {
		apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}

	apiutil.JSON(w, http.StatusOK, n)
}

// List handles GET /api/v1/nodes.
func (h *NodeHandler) List(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)
	claims := middleware.ClaimsFromContext(r.Context())

	// Non-superuser sees only their tenant's nodes.
	var tenantFilter *domain.ID
	if claims.TenantID != nil {
		tenantFilter = claims.TenantID
	}

	nodes, total, err := h.svc.ListNodes(r.Context(), tenantFilter, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list nodes")
		return
	}

	apiutil.ListJSON(w, nodes, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/nodes/{id}.
func (h *NodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req fleet.UpdateNodeRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	n, err := h.svc.UpdateNode(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
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
		Resource: "node",
		Details:  map[string]string{"node_id": n.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, n)
}

// Delete handles DELETE /api/v1/nodes/{id}.
func (h *NodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteNode(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete node")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "node",
		Details:  map[string]string{"node_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// Heartbeat handles POST /api/v1/nodes/{id}/heartbeat.
func (h *NodeHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.RecordNodeHeartbeat(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to record heartbeat")
		return
	}

	apiutil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
