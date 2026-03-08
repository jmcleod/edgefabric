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

// NodeGroupHandler handles node group CRUD endpoints.
type NodeGroupHandler struct {
	svc        fleet.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewNodeGroupHandler creates a new node group handler.
func NewNodeGroupHandler(svc fleet.Service, authorizer rbac.Authorizer, audit audit.Logger) *NodeGroupHandler {
	return &NodeGroupHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts node group routes on the mux.
func (h *NodeGroupHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceNodeGroup, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNodeGroup, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceNodeGroup, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceNodeGroup, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceNodeGroup, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/node-groups", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/node-groups", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/node-groups/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("DELETE /api/v1/node-groups/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
	mux.Handle("POST /api/v1/node-groups/{id}/nodes/{nodeId}", middleware.Chain(http.HandlerFunc(h.AddNode), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/node-groups/{id}/nodes/{nodeId}", middleware.Chain(http.HandlerFunc(h.RemoveNode), authMW, requireUpdate))
}

// Create handles POST /api/v1/node-groups.
func (h *NodeGroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req fleet.CreateNodeGroupRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Set tenant from claims if not superuser.
	claims := middleware.ClaimsFromContext(r.Context())
	if claims.TenantID != nil {
		req.TenantID = *claims.TenantID
	}

	g, err := h.svc.CreateNodeGroup(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "node group already exists")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "node_group",
		Details:  map[string]string{"group_id": g.ID.String(), "name": g.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, g)
}

// Get handles GET /api/v1/node-groups/{id}.
func (h *NodeGroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	g, err := h.svc.GetNodeGroup(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get node group")
		return
	}

	// Tenant isolation.
	claims := middleware.ClaimsFromContext(r.Context())
	if claims.TenantID != nil && *claims.TenantID != g.TenantID {
		apiutil.WriteError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	}

	apiutil.JSON(w, http.StatusOK, g)
}

// List handles GET /api/v1/node-groups.
func (h *NodeGroupHandler) List(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)
	claims := middleware.ClaimsFromContext(r.Context())

	// Require tenant scope for non-superuser.
	if claims.TenantID == nil {
		// SuperUser must provide tenant_id query param.
		tenantIDStr := r.URL.Query().Get("tenant_id")
		if tenantIDStr == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "tenant_id query parameter required for superuser")
			return
		}
		tid, err := domain.ParseID(tenantIDStr)
		if err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid tenant_id")
			return
		}
		groups, total, err := h.svc.ListNodeGroups(r.Context(), tid, params)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list node groups")
			return
		}
		apiutil.ListJSON(w, groups, total, params.Offset, params.Limit)
		return
	}

	groups, total, err := h.svc.ListNodeGroups(r.Context(), *claims.TenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list node groups")
		return
	}

	apiutil.ListJSON(w, groups, total, params.Offset, params.Limit)
}

// Delete handles DELETE /api/v1/node-groups/{id}.
func (h *NodeGroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Verify group exists and check tenant isolation.
	g, err := h.svc.GetNodeGroup(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get node group")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims.TenantID != nil && *claims.TenantID != g.TenantID {
		apiutil.WriteError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	}

	_ = g // verified tenant isolation above

	if err := h.svc.DeleteNodeGroup(r.Context(), id); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete node group")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "node_group",
		Details:  map[string]string{"group_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// AddNode handles POST /api/v1/node-groups/{id}/nodes/{nodeId}.
func (h *NodeGroupHandler) AddNode(w http.ResponseWriter, r *http.Request) {
	groupID, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	nodeID, err := apiutil.ParseID(r, "nodeId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.AddNodeToGroup(r.Context(), groupID, nodeID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "group or node not found")
			return
		}
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "node already in group")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to add node to group")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "update",
		Resource: "node_group",
		Details:  map[string]string{"group_id": groupID.String(), "node_id": nodeID.String(), "action": "add_node"},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// RemoveNode handles DELETE /api/v1/node-groups/{id}/nodes/{nodeId}.
func (h *NodeGroupHandler) RemoveNode(w http.ResponseWriter, r *http.Request) {
	groupID, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	nodeID, err := apiutil.ParseID(r, "nodeId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.RemoveNodeFromGroup(r.Context(), groupID, nodeID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to remove node from group")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "update",
		Resource: "node_group",
		Details:  map[string]string{"group_id": groupID.String(), "node_id": nodeID.String(), "action": "remove_node"},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
