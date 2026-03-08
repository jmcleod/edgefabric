package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// NodeNetworkingHandler handles the node networking state endpoint.
type NodeNetworkingHandler struct {
	svc        networking.Service
	authorizer rbac.Authorizer
}

// NewNodeNetworkingHandler creates a new node networking handler.
func NewNodeNetworkingHandler(svc networking.Service, authorizer rbac.Authorizer) *NodeNetworkingHandler {
	return &NodeNetworkingHandler{svc: svc, authorizer: authorizer}
}

// Register mounts node networking routes on the mux.
func (h *NodeNetworkingHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNode, middleware.TenantFromClaims())

	mux.Handle("GET /api/v1/nodes/{id}/networking", middleware.Chain(http.HandlerFunc(h.GetState), authMW, requireRead))
}

// GetState handles GET /api/v1/nodes/{id}/networking.
func (h *NodeNetworkingHandler) GetState(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	state, err := h.svc.GetNodeNetworkingState(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get node networking state")
		return
	}

	apiutil.JSON(w, http.StatusOK, state)
}
