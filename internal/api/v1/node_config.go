package v1

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// NodeConfigHandler serves node configuration files for polling-based sync.
// Nodes poll these endpoints to get their desired WireGuard, BGP, DNS, CDN, and route configuration.
// On each successful config fetch, the node's last_config_sync timestamp is updated
// for config drift visibility.
type NodeConfigHandler struct {
	svc        networking.Service
	dnsSvc     dns.Service
	cdnSvc     cdn.Service
	routeSvc   route.Service
	nodeStore  storage.NodeStore
	authorizer rbac.Authorizer
}

// NewNodeConfigHandler creates a new node config handler.
func NewNodeConfigHandler(svc networking.Service, dnsSvc dns.Service, cdnSvc cdn.Service, routeSvc route.Service, nodeStore storage.NodeStore, authorizer rbac.Authorizer) *NodeConfigHandler {
	return &NodeConfigHandler{svc: svc, dnsSvc: dnsSvc, cdnSvc: cdnSvc, routeSvc: routeSvc, nodeStore: nodeStore, authorizer: authorizer}
}

// Register mounts node config routes on the mux.
func (h *NodeConfigHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNode, middleware.TenantFromClaims())
	// Enrollment tokens (readonly, UserID = nodeID) may only access their own config.
	// Admins and superusers bypass this check.
	requireOwner := middleware.RequireResourceOwnerOrAdmin("id")

	mux.Handle("GET /api/v1/nodes/{id}/config/wireguard", middleware.Chain(http.HandlerFunc(h.GetWireGuardConfig), authMW, requireRead, requireOwner))
	mux.Handle("GET /api/v1/nodes/{id}/config/bgp", middleware.Chain(http.HandlerFunc(h.GetBGPConfig), authMW, requireRead, requireOwner))
	mux.Handle("GET /api/v1/nodes/{id}/config/dns", middleware.Chain(http.HandlerFunc(h.GetDNSConfig), authMW, requireRead, requireOwner))
	mux.Handle("GET /api/v1/nodes/{id}/config/cdn", middleware.Chain(http.HandlerFunc(h.GetCDNConfig), authMW, requireRead, requireOwner))
	mux.Handle("GET /api/v1/nodes/{id}/config/routes", middleware.Chain(http.HandlerFunc(h.GetRouteConfig), authMW, requireRead, requireOwner))
}

// GetWireGuardConfig handles GET /api/v1/nodes/{id}/config/wireguard.
// Returns the wg0.conf content as text/plain for direct file deployment.
func (h *NodeConfigHandler) GetWireGuardConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	conf, err := h.svc.GenerateNodeConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found or wireguard not configured")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate wireguard config")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.nodeStore.UpdateNodeConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update node config sync timestamp",
			slog.String("node_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(conf))
}

// GetBGPConfig handles GET /api/v1/nodes/{id}/config/bgp.
// Returns the desired BGP sessions as JSON for the node's BGP reconciliation loop.
func (h *NodeConfigHandler) GetBGPConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sessions, _, err := h.svc.ListBGPSessions(r.Context(), id, storage.ListParams{Limit: 1000})
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list bgp sessions")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.nodeStore.UpdateNodeConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update node config sync timestamp",
			slog.String("node_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	apiutil.JSON(w, http.StatusOK, sessions)
}

// GetDNSConfig handles GET /api/v1/nodes/{id}/config/dns.
// Returns the desired DNS zones and records as JSON for the node's DNS reconciliation loop.
func (h *NodeConfigHandler) GetDNSConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if h.dnsSvc == nil {
		apiutil.WriteError(w, http.StatusNotImplemented, "not_implemented", "dns service not available")
		return
	}

	config, err := h.dnsSvc.GetNodeDNSConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get dns config")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.nodeStore.UpdateNodeConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update node config sync timestamp",
			slog.String("node_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	apiutil.JSON(w, http.StatusOK, config)
}

// GetCDNConfig handles GET /api/v1/nodes/{id}/config/cdn.
// Returns the desired CDN sites and origins as JSON for the node's CDN reconciliation loop.
func (h *NodeConfigHandler) GetCDNConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if h.cdnSvc == nil {
		apiutil.WriteError(w, http.StatusNotImplemented, "not_implemented", "cdn service not available")
		return
	}

	config, err := h.cdnSvc.GetNodeCDNConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get cdn config")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.nodeStore.UpdateNodeConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update node config sync timestamp",
			slog.String("node_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	apiutil.JSON(w, http.StatusOK, config)
}

// GetRouteConfig handles GET /api/v1/nodes/{id}/config/routes.
// Returns the desired route forwarding config as JSON for the node's route reconciliation loop.
func (h *NodeConfigHandler) GetRouteConfig(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if h.routeSvc == nil {
		apiutil.WriteError(w, http.StatusNotImplemented, "not_implemented", "route service not available")
		return
	}

	routeConfig, err := h.routeSvc.GetNodeRouteConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get route config")
		return
	}

	// Record config sync timestamp (best-effort).
	if err := h.nodeStore.UpdateNodeConfigSync(r.Context(), id); err != nil {
		slog.Warn("failed to update node config sync timestamp",
			slog.String("node_id", id.String()),
			slog.String("error", err.Error()),
		)
	}

	apiutil.JSON(w, http.StatusOK, routeConfig)
}
