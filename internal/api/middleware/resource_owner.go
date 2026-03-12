package middleware

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// RequireResourceOwnerOrAdmin returns middleware that restricts readonly
// (enrollment) tokens to only access their own resource. The middleware
// compares claims.UserID (which holds the node/gateway ID for enrollment
// tokens) against the {paramName} path parameter.
//
// Superusers (TenantID == nil) and admin-role users bypass this check so
// human operators can still inspect any node/gateway config.
func RequireResourceOwnerOrAdmin(paramName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			// Superusers and admins can access any resource.
			if claims.TenantID == nil || claims.Role == domain.RoleAdmin || claims.Role == domain.RoleSuperUser {
				next.ServeHTTP(w, r)
				return
			}

			// For readonly tokens (enrollment tokens), enforce ownership.
			pathID, err := apiutil.ParseID(r, paramName)
			if err != nil {
				apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}

			if claims.UserID != pathID {
				apiutil.WriteError(w, http.StatusForbidden, "forbidden", "not authorized for this resource")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
