package v1

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// GatewayConfigHandler serves gateway configuration files for polling-based sync.
// Gateways poll these endpoints to get their desired route forwarding configuration.
// On each successful config fetch, the gateway's last_config_sync timestamp is updated
// for config drift visibility.
type GatewayConfigHandler struct {
	routeSvc     route.Service
	gatewayStore storage.GatewayStore
	authorizer   rbac.Authorizer
}

// NewGatewayConfigHandler creates a new gateway config handler.
func NewGatewayConfigHandler(routeSvc route.Service, gatewayStore storage.GatewayStore, authorizer rbac.Authorizer) *GatewayConfigHandler {
	return &GatewayConfigHandler{routeSvc: routeSvc, gatewayStore: gatewayStore, authorizer: authorizer}
}

// Register mounts gateway config routes on the mux.
func (h *GatewayConfigHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceGateway, middleware.TenantFromClaims())
	// Enrollment tokens (readonly, UserID = gatewayID) may only access their own config.
	requireOwner := middleware.RequireResourceOwnerOrAdmin("id")

	mux.Handle("GET /api/v1/gateways/{id}/config/routes", middleware.Chain(http.HandlerFunc(h.GetRouteConfig), authMW, requireRead, requireOwner))
}

// GetRouteConfig handles GET /api/v1/gateways/{id}/config/routes.
// Returns the desired route forwarding config as JSON for the gateway's route reconciliation loop.
func (h *GatewayConfigHandler) GetRouteConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	config, err := h.routeSvc.GetGatewayRouteConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "gateway not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get route config")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.gatewayStore.UpdateGatewayConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update gateway config sync timestamp",
			slog.String("gateway_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	apiutil.JSON(w, http.StatusOK, config)
}
