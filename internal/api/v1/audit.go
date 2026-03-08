package v1

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

// AuditHandler handles audit event list endpoints.
type AuditHandler struct {
	audit      audit.Logger
	authorizer rbac.Authorizer
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(audit audit.Logger, authorizer rbac.Authorizer) *AuditHandler {
	return &AuditHandler{audit: audit, authorizer: authorizer}
}

// Register mounts audit routes on the mux.
func (h *AuditHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceAuditEvent, middleware.TenantFromClaims())
	mux.Handle("GET /api/v1/audit-events", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
}

// List handles GET /api/v1/audit-events.
// SuperUsers see all events; tenant-scoped users see only their tenant's events.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	params := apiutil.ParseListParams(r)

	// Non-superusers see only their tenant's events.
	tenantFilter := claims.TenantID

	events, total, err := h.audit.List(r.Context(), tenantFilter, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list audit events")
		return
	}

	apiutil.ListJSON(w, events, total, params.Offset, params.Limit)
}
