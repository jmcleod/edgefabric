package middleware

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

// RequirePermission returns middleware that enforces an RBAC check for the
// given action and resource. It expects claims to already be in the context
// (i.e., the Auth middleware must run first).
//
// tenantIDFunc optionally extracts a tenant ID from the request for scoped checks.
// Pass nil for endpoints that are not tenant-scoped (e.g., superuser-only).
func RequirePermission(authorizer rbac.Authorizer, action rbac.Action, resource rbac.Resource, tenantIDFunc func(*http.Request) *domain.ID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			var tenantID *domain.ID
			if tenantIDFunc != nil {
				tenantID = tenantIDFunc(r)
			}

			if err := authorizer.Authorize(r.Context(), *claims, action, resource, tenantID); err != nil {
				apiutil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantFromClaims returns a function that extracts the tenant ID from the
// authenticated claims. Useful for endpoints where the tenant scope comes
// from the caller's identity rather than a path parameter.
func TenantFromClaims() func(*http.Request) *domain.ID {
	return func(r *http.Request) *domain.ID {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			return nil
		}
		return claims.TenantID
	}
}
