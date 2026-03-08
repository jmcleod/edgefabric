// Package middleware provides HTTP middleware for the EdgeFabric API.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/auth"
)

// claimsKey is the context key for authenticated claims.
type claimsKey struct{}

// ClaimsFromContext extracts auth claims from request context.
// Returns nil if no claims are present (unauthenticated request).
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey{}).(*auth.Claims)
	return c
}

// ContextWithClaims returns a new context with the given claims attached.
// This is primarily intended for testing; production code uses the Auth middleware.
func ContextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// Auth returns middleware that authenticates requests via Bearer token or API key.
//
// Authentication methods:
//   - Bearer token: "Authorization: Bearer <token>" — verified by the token service
//   - API key: "Authorization: Bearer ef_<key>" — prefix "ef_" triggers API key auth
//
// Security: unauthenticated requests receive 401; the middleware does NOT enforce
// authorization — use RequirePermission for that.
func Auth(tokenSvc *auth.TokenService, authSvc auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}

			if !strings.HasPrefix(header, "Bearer ") {
				apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid authorization scheme")
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == "" {
				apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "empty token")
				return
			}

			var claims *auth.Claims

			// API keys are prefixed with "ef_" to distinguish from session tokens.
			if strings.HasPrefix(token, "ef_") {
				apiKey, err := authSvc.AuthenticateAPIKey(r.Context(), token)
				if err != nil {
					apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
					return
				}
				claims = &auth.Claims{
					UserID:   apiKey.UserID,
					TenantID: &apiKey.TenantID,
					Role:     apiKey.Role,
					APIKeyID: &apiKey.ID,
				}
			} else {
				c, err := tokenSvc.Verify(token)
				if err != nil {
					apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
					return
				}
				claims = c
			}

			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
