package apiutil

import (
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// CanAccessTenant checks whether the given claims allow access to a resource
// with the specified tenant ID (pointer form, for domain types like Node
// where TenantID is *domain.ID). Superusers (nil TenantID in claims) always
// pass. Returns true if access is permitted.
func CanAccessTenant(claims *auth.Claims, resourceTenantID *domain.ID) bool {
	if claims.TenantID == nil {
		return true // superuser
	}
	if resourceTenantID == nil {
		return true // resource not yet assigned to a tenant
	}
	return *claims.TenantID == *resourceTenantID
}

// CanAccessTenantVal is like CanAccessTenant but takes a non-pointer tenant ID,
// for domain types like Gateway where TenantID is domain.ID.
func CanAccessTenantVal(claims *auth.Claims, resourceTenantID domain.ID) bool {
	if claims.TenantID == nil {
		return true // superuser
	}
	return *claims.TenantID == resourceTenantID
}
