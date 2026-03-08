package v1

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
	"github.com/jmcleod/edgefabric/pkg/version"
)

// StatusHandler provides a dashboard/status endpoint with fleet overview.
type StatusHandler struct {
	tenantSvc  tenant.Service
	userSvc    user.Service
	fleetSvc   fleet.Service
	authorizer rbac.Authorizer
}

// NewStatusHandler creates a new status handler.
func NewStatusHandler(tenantSvc tenant.Service, userSvc user.Service, fleetSvc fleet.Service, authorizer rbac.Authorizer) *StatusHandler {
	return &StatusHandler{
		tenantSvc:  tenantSvc,
		userSvc:    userSvc,
		fleetSvc:   fleetSvc,
		authorizer: authorizer,
	}
}

// Register mounts status routes on the mux.
func (h *StatusHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceNode, middleware.TenantFromClaims())
	mux.Handle("GET /api/v1/status", middleware.Chain(http.HandlerFunc(h.Status), authMW, requireRead))
}

// statusResponse is the dashboard status overview.
type statusResponse struct {
	Version     string      `json:"version"`
	TenantCount int         `json:"tenant_count,omitempty"` // Only for superuser
	UserCount   int         `json:"user_count"`
	NodeCount   int         `json:"node_count"`
	NodesByStatus map[string]int `json:"nodes_by_status"`
}

// Status handles GET /api/v1/status — returns a fleet overview.
func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())

	resp := statusResponse{
		Version:       version.Version,
		NodesByStatus: make(map[string]int),
	}

	// Get tenant count (superuser only).
	if claims.TenantID == nil {
		tenants, _, err := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 1})
		if err == nil {
			_ = tenants // We only need the total.
			_, total, _ := h.tenantSvc.List(r.Context(), storage.ListParams{Limit: 0})
			resp.TenantCount = total
		}
	}

	// Get user and node counts (scoped to tenant for non-superuser).
	var tenantFilter *domain.ID
	if claims.TenantID != nil {
		tenantFilter = claims.TenantID
	}

	_, userTotal, err := h.userSvc.List(r.Context(), tenantFilter, storage.ListParams{Limit: 1})
	if err == nil {
		resp.UserCount = userTotal
	}

	nodes, nodeTotal, err := h.fleetSvc.ListNodes(r.Context(), tenantFilter, storage.ListParams{Limit: 200})
	if err == nil {
		resp.NodeCount = nodeTotal
		for _, n := range nodes {
			resp.NodesByStatus[string(n.Status)]++
		}
	}

	apiutil.JSON(w, http.StatusOK, resp)
}
