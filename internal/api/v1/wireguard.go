package v1

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

// WireGuardHandler handles WireGuard peer listing endpoints.
type WireGuardHandler struct {
	svc        networking.Service
	authorizer rbac.Authorizer
}

// NewWireGuardHandler creates a new WireGuard handler.
func NewWireGuardHandler(svc networking.Service, authorizer rbac.Authorizer) *WireGuardHandler {
	return &WireGuardHandler{svc: svc, authorizer: authorizer}
}

// Register mounts WireGuard routes on the mux.
func (h *WireGuardHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	// WireGuard peer listing is superuser-only (no tenant scope → nil tenantIDFunc).
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceNode, nil)

	mux.Handle("GET /api/v1/wireguard/peers", middleware.Chain(http.HandlerFunc(h.ListPeers), authMW, requireList))
}

// ListPeers handles GET /api/v1/wireguard/peers.
func (h *WireGuardHandler) ListPeers(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)

	peers, total, err := h.svc.ListWireGuardPeers(r.Context(), params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list wireguard peers")
		return
	}

	apiutil.ListJSON(w, peers, total, params.Offset, params.Limit)
}
