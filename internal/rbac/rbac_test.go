package rbac_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

func TestSuperUserAllowedEverything(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	claims := auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleSuperUser,
	}

	resources := []rbac.Resource{
		rbac.ResourceTenant, rbac.ResourceUser, rbac.ResourceNode,
		rbac.ResourceSSHKey, rbac.ResourceAPIKey,
	}
	actions := []rbac.Action{
		rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate,
		rbac.ActionDelete, rbac.ActionList,
	}

	for _, res := range resources {
		for _, act := range actions {
			if err := authorizer.Authorize(context.Background(), claims, act, res, nil); err != nil {
				t.Errorf("superuser should be allowed %s on %s, got %v", act, res, err)
			}
		}
	}
}

func TestReadOnlyCanOnlyReadAndList(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	tenantID := domain.NewID()
	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleReadOnly,
	}

	// Read and list should be allowed.
	for _, act := range []rbac.Action{rbac.ActionRead, rbac.ActionList} {
		if err := authorizer.Authorize(context.Background(), claims, act, rbac.ResourceNode, &tenantID); err != nil {
			t.Errorf("readonly should be allowed %s, got %v", act, err)
		}
	}

	// Create, update, delete should be forbidden.
	for _, act := range []rbac.Action{rbac.ActionCreate, rbac.ActionUpdate, rbac.ActionDelete} {
		if err := authorizer.Authorize(context.Background(), claims, act, rbac.ResourceNode, &tenantID); err == nil {
			t.Errorf("readonly should NOT be allowed %s on node", act)
		}
	}
}

func TestAdminCanCRUDWithinTenant(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	tenantID := domain.NewID()
	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	}

	// Admin should be able to CRUD nodes in their tenant.
	for _, act := range []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionList} {
		if err := authorizer.Authorize(context.Background(), claims, act, rbac.ResourceNode, &tenantID); err != nil {
			t.Errorf("admin should be allowed %s on node in own tenant, got %v", act, err)
		}
	}
}

func TestAdminCannotAccessOtherTenant(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	tenantID := domain.NewID()
	otherTenant := domain.NewID()
	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	}

	// Admin should NOT access another tenant's resources.
	if err := authorizer.Authorize(context.Background(), claims, rbac.ActionRead, rbac.ResourceNode, &otherTenant); err == nil {
		t.Error("admin should NOT access other tenant's nodes")
	}
}

func TestAdminCannotMutateGlobalResources(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	tenantID := domain.NewID()
	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	}

	// Admin can read/list SSH keys and tenants.
	for _, res := range []rbac.Resource{rbac.ResourceSSHKey, rbac.ResourceTenant} {
		if err := authorizer.Authorize(context.Background(), claims, rbac.ActionRead, res, nil); err != nil {
			t.Errorf("admin should read %s, got %v", res, err)
		}
		if err := authorizer.Authorize(context.Background(), claims, rbac.ActionList, res, nil); err != nil {
			t.Errorf("admin should list %s, got %v", res, err)
		}
	}

	// Admin cannot create/update/delete SSH keys or tenants.
	for _, act := range []rbac.Action{rbac.ActionCreate, rbac.ActionUpdate, rbac.ActionDelete} {
		for _, res := range []rbac.Resource{rbac.ResourceSSHKey, rbac.ResourceTenant} {
			if err := authorizer.Authorize(context.Background(), claims, act, res, nil); err == nil {
				t.Errorf("admin should NOT %s %s", act, res)
			}
		}
	}
}

func TestNonSuperUserWithoutTenantIsForbidden(t *testing.T) {
	authorizer := rbac.NewAuthorizer()
	claims := auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleAdmin,
		// No TenantID — this is invalid for non-superuser.
	}

	if err := authorizer.Authorize(context.Background(), claims, rbac.ActionRead, rbac.ResourceNode, nil); err == nil {
		t.Error("non-superuser without tenant should be forbidden")
	}
}
