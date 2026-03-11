package v1

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/rbac"

	dto "github.com/prometheus/client_model/go"
)

// TenantUsageHandler serves per-tenant usage metrics.
type TenantUsageHandler struct {
	metrics    *observability.Metrics
	authorizer rbac.Authorizer
}

// NewTenantUsageHandler creates a new tenant usage handler.
func NewTenantUsageHandler(metrics *observability.Metrics, authorizer rbac.Authorizer) *TenantUsageHandler {
	return &TenantUsageHandler{metrics: metrics, authorizer: authorizer}
}

// Register mounts tenant usage routes on the mux.
func (h *TenantUsageHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceTenant, middleware.TenantFromClaims())
	mux.Handle("GET /api/v1/tenants/{id}/usage", middleware.Chain(http.HandlerFunc(h.GetUsage), authMW, requireRead))
}

// tenantUsageResponse is the JSON response for tenant usage metrics.
type tenantUsageResponse struct {
	HTTPRequests        float64 `json:"http_requests"`
	CDNBandwidthBytes   float64 `json:"cdn_bandwidth_bytes"`
	DNSQueries          float64 `json:"dns_queries"`
	RouteBytesForwarded float64 `json:"route_bytes_forwarded"`
}

// GetUsage handles GET /api/v1/tenants/{id}/usage.
func (h *TenantUsageHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "tenant id required")
		return
	}

	// Gather all metrics from the registry and sum by tenant_id label.
	families, err := h.metrics.Registry.Gather()
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to gather metrics")
		return
	}

	resp := tenantUsageResponse{}
	for _, mf := range families {
		name := mf.GetName()
		sum := sumByTenantLabel(mf.GetMetric(), tenantID)
		switch name {
		case "edgefabric_tenant_http_requests_total":
			resp.HTTPRequests = sum
		case "edgefabric_tenant_cdn_bandwidth_bytes_total":
			resp.CDNBandwidthBytes = sum
		case "edgefabric_tenant_dns_queries_total":
			resp.DNSQueries = sum
		case "edgefabric_tenant_route_bytes_forwarded_total":
			resp.RouteBytesForwarded = sum
		}
	}

	apiutil.JSON(w, http.StatusOK, resp)
}

// sumByTenantLabel sums counter values across all metrics where tenant_id matches.
func sumByTenantLabel(metrics []*dto.Metric, tenantID string) float64 {
	var total float64
	for _, m := range metrics {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "tenant_id" && lp.GetValue() == tenantID {
				if c := m.GetCounter(); c != nil {
					total += c.GetValue()
				}
				break
			}
		}
	}
	return total
}
