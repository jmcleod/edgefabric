// Package rbac implements role-based access control.
package rbac

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// Action represents a permission action.
type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionList   Action = "list"
)

// Resource represents a resource type for RBAC.
type Resource string

const (
	ResourceTenant       Resource = "tenant"
	ResourceUser         Resource = "user"
	ResourceNode         Resource = "node"
	ResourceNodeGroup    Resource = "node_group"
	ResourceGateway      Resource = "gateway"
	ResourceIPAllocation Resource = "ip_allocation"
	ResourceBGPSession   Resource = "bgp_session"
	ResourceDNSZone      Resource = "dns_zone"
	ResourceDNSRecord    Resource = "dns_record"
	ResourceCDNSite      Resource = "cdn_site"
	ResourceCDNOrigin    Resource = "cdn_origin"
	ResourceRoute        Resource = "route"
	ResourceSSHKey       Resource = "ssh_key"
	ResourceTLSCert      Resource = "tls_certificate"
	ResourceAuditEvent   Resource = "audit_event"
	ResourceAPIKey       Resource = "api_key"
)

// Authorizer checks whether a set of claims permits an action on a resource.
type Authorizer interface {
	Authorize(ctx context.Context, claims auth.Claims, action Action, resource Resource, tenantID *domain.ID) error
}

// ErrForbidden is returned when authorization fails.
var ErrForbidden = fmt.Errorf("forbidden")

// DefaultAuthorizer implements basic RBAC rules.
type DefaultAuthorizer struct{}

// NewAuthorizer creates a default RBAC authorizer.
func NewAuthorizer() Authorizer {
	return &DefaultAuthorizer{}
}

// Authorize checks permissions based on role and tenant scope.
func (a *DefaultAuthorizer) Authorize(_ context.Context, claims auth.Claims, action Action, resource Resource, tenantID *domain.ID) error {
	// SuperUser can do everything.
	if claims.Role == domain.RoleSuperUser {
		return nil
	}

	// ReadOnly can only read/list.
	if claims.Role == domain.RoleReadOnly {
		if action != ActionRead && action != ActionList {
			return ErrForbidden
		}
	}

	// Non-superuser must have a tenant scope.
	if claims.TenantID == nil {
		return ErrForbidden
	}

	// Ensure tenant-scoped users only access their own tenant's resources.
	if tenantID != nil && *claims.TenantID != *tenantID {
		return ErrForbidden
	}

	// Global resources (ssh_key, tenant management) require SuperUser.
	switch resource {
	case ResourceTenant, ResourceSSHKey:
		if action != ActionRead && action != ActionList {
			return ErrForbidden
		}
	}

	return nil
}
