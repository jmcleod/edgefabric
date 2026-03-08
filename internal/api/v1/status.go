package v1

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
	"github.com/jmcleod/edgefabric/pkg/version"
)

// StatusHandler provides a dashboard/status endpoint with fleet overview.
type StatusHandler struct {
	tenantSvc       tenant.Service
	userSvc         user.Service
	fleetSvc        fleet.Service
	dnsSvc          dns.Service
	cdnSvc          cdn.Service
	routeSvc        route.Service
	schemaVersioner storage.SchemaVersioner
	authorizer      rbac.Authorizer
}

// NewStatusHandler creates a new status handler.
func NewStatusHandler(
	tenantSvc tenant.Service,
	userSvc user.Service,
	fleetSvc fleet.Service,
	dnsSvc dns.Service,
	cdnSvc cdn.Service,
	routeSvc route.Service,
	schemaVersioner storage.SchemaVersioner,
	authorizer rbac.Authorizer,
) *StatusHandler {
	return &StatusHandler{
		tenantSvc:       tenantSvc,
		userSvc:         userSvc,
		fleetSvc:        fleetSvc,
		dnsSvc:          dnsSvc,
		cdnSvc:          cdnSvc,
		routeSvc:        routeSvc,
		schemaVersioner: schemaVersioner,
		authorizer:      authorizer,
	}
}

// Register mounts status routes on the mux.
func (h *StatusHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNode, middleware.TenantFromClaims())
	mux.Handle("GET /api/v1/status", middleware.Chain(http.HandlerFunc(h.Status), authMW, requireRead))
}

// statusResponse is the dashboard status overview.
type statusResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`

	TenantCount int `json:"tenant_count,omitempty"` // Only for superuser.
	UserCount   int `json:"user_count"`

	NodeCount     int            `json:"node_count"`
	NodesByStatus map[string]int `json:"nodes_by_status"`

	GatewayCount     int            `json:"gateway_count"`
	GatewaysByStatus map[string]int `json:"gateways_by_status"`

	StaleNodeCount    int `json:"stale_node_count"`
	StaleGatewayCount int `json:"stale_gateway_count"`

	RouteCount    int `json:"route_count"`
	DNSZoneCount  int `json:"dns_zone_count"`
	CDNSiteCount  int `json:"cdn_site_count"`
	SchemaVersion int `json:"schema_version"`
}

// Status handles GET /api/v1/status — returns a fleet overview.
func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())

	resp := statusResponse{
		Version:          version.Version,
		Commit:           version.Commit,
		BuildTime:        version.BuildTime,
		NodesByStatus:    make(map[string]int),
		GatewaysByStatus: make(map[string]int),
	}

	// Determine tenant scope. Superuser sees everything; tenant users see their own.
	var tenantFilter *domain.ID
	if claims.TenantID != nil {
		tenantFilter = claims.TenantID
	}

	// Get tenant count (superuser only — single query using returned total).
	if claims.TenantID == nil {
		_, total, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 1})
		if err != nil {
			slog.Warn("status: failed to list tenants", slog.String("error", err.Error()))
		} else {
			resp.TenantCount = total
		}
	}

	// User count.
	_, userTotal, err := h.userSvc.List(r.Context(), tenantFilter, storage.ListParams{Limit: 1})
	if err != nil {
		slog.Warn("status: failed to list users", slog.String("error", err.Error()))
	} else {
		resp.UserCount = userTotal
	}

	// Node count + breakdown by status + stale config detection.
	staleThreshold := time.Now().UTC().Add(-5 * time.Minute)
	nodes, nodeTotal, err := h.fleetSvc.ListNodes(r.Context(), tenantFilter, storage.ListParams{Limit: 200})
	if err != nil {
		slog.Warn("status: failed to list nodes", slog.String("error", err.Error()))
	} else {
		resp.NodeCount = nodeTotal
		for _, n := range nodes {
			resp.NodesByStatus[string(n.Status)]++
			// A node is "stale" if it's online but hasn't synced config recently.
			if n.Status == domain.NodeStatusOnline {
				if n.LastConfigSync == nil || n.LastConfigSync.Before(staleThreshold) {
					resp.StaleNodeCount++
				}
			}
		}
	}

	// Gateway count + breakdown by status.
	// ListGateways requires a non-pointer tenantID. For superuser (no tenant filter),
	// iterate over tenants to aggregate. For tenant-scoped users, query directly.
	if tenantFilter != nil {
		gateways, gwTotal, err := h.fleetSvc.ListGateways(r.Context(), *tenantFilter, storage.ListParams{Limit: 200})
		if err != nil {
			slog.Warn("status: failed to list gateways", slog.String("error", err.Error()))
		} else {
			resp.GatewayCount = gwTotal
			for _, gw := range gateways {
				resp.GatewaysByStatus[string(gw.Status)]++
				if gw.Status == domain.GatewayStatusOnline {
					if gw.LastConfigSync == nil || gw.LastConfigSync.Before(staleThreshold) {
						resp.StaleGatewayCount++
					}
				}
			}
		}
	} else {
		// Superuser: iterate tenants.
		tenants, _, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 200})
		if err != nil {
			slog.Warn("status: failed to list tenants for gateway aggregation", slog.String("error", err.Error()))
		} else {
			for _, t := range tenants {
				gateways, gwTotal, err := h.fleetSvc.ListGateways(r.Context(), t.ID, storage.ListParams{Limit: 200})
				if err != nil {
					slog.Warn("status: failed to list gateways for tenant",
						slog.String("tenant_id", t.ID.String()),
						slog.String("error", err.Error()),
					)
					continue
				}
				resp.GatewayCount += gwTotal
				for _, gw := range gateways {
					resp.GatewaysByStatus[string(gw.Status)]++
					if gw.Status == domain.GatewayStatusOnline {
						if gw.LastConfigSync == nil || gw.LastConfigSync.Before(staleThreshold) {
							resp.StaleGatewayCount++
						}
					}
				}
			}
		}
	}

	// Route count.
	if h.routeSvc != nil {
		if tenantFilter != nil {
			_, routeTotal, err := h.routeSvc.ListRoutes(r.Context(), *tenantFilter, storage.ListParams{Limit: 1})
			if err != nil {
				slog.Warn("status: failed to list routes", slog.String("error", err.Error()))
			} else {
				resp.RouteCount = routeTotal
			}
		} else {
			tenants, _, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 200})
			if err == nil {
				for _, t := range tenants {
					_, rTotal, err := h.routeSvc.ListRoutes(r.Context(), t.ID, storage.ListParams{Limit: 1})
					if err == nil {
						resp.RouteCount += rTotal
					}
				}
			}
		}
	}

	// DNS zone count.
	if h.dnsSvc != nil {
		if tenantFilter != nil {
			_, zoneTotal, err := h.dnsSvc.ListZones(r.Context(), *tenantFilter, storage.ListParams{Limit: 1})
			if err != nil {
				slog.Warn("status: failed to list DNS zones", slog.String("error", err.Error()))
			} else {
				resp.DNSZoneCount = zoneTotal
			}
		} else {
			tenants, _, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 200})
			if err == nil {
				for _, t := range tenants {
					_, zTotal, err := h.dnsSvc.ListZones(r.Context(), t.ID, storage.ListParams{Limit: 1})
					if err == nil {
						resp.DNSZoneCount += zTotal
					}
				}
			}
		}
	}

	// CDN site count.
	if h.cdnSvc != nil {
		if tenantFilter != nil {
			_, siteTotal, err := h.cdnSvc.ListSites(r.Context(), *tenantFilter, storage.ListParams{Limit: 1})
			if err != nil {
				slog.Warn("status: failed to list CDN sites", slog.String("error", err.Error()))
			} else {
				resp.CDNSiteCount = siteTotal
			}
		} else {
			tenants, _, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 200})
			if err == nil {
				for _, t := range tenants {
					_, sTotal, err := h.cdnSvc.ListSites(r.Context(), t.ID, storage.ListParams{Limit: 1})
					if err == nil {
						resp.CDNSiteCount += sTotal
					}
				}
			}
		}
	}

	// Schema version.
	if h.schemaVersioner != nil {
		sv, err := h.schemaVersioner.SchemaVersion(r.Context())
		if err != nil {
			slog.Warn("status: failed to get schema version", slog.String("error", err.Error()))
		} else {
			resp.SchemaVersion = sv
		}
	}

	apiutil.JSON(w, http.StatusOK, resp)
}
