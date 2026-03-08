package user_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
	"github.com/jmcleod/edgefabric/internal/user"
)

const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func newTestStore(t *testing.T) (*sqlite.SQLiteStore, auth.Service) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	secretStore, err := secrets.NewStore(testKey)
	if err != nil {
		t.Fatalf("secret store: %v", err)
	}

	authSvc := auth.NewService(store, store, secretStore, "test")
	return store, authSvc
}

func TestUserServiceCreate(t *testing.T) {
	store, authSvc := newTestStore(t)
	svc := user.NewService(store, authSvc)
	ctx := context.Background()

	// Create a tenant for the user.
	tenant := &domain.Tenant{ID: domain.NewID(), Name: "Test", Slug: "test-user", Status: domain.TenantStatusActive}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	u, err := svc.Create(ctx, user.CreateRequest{
		TenantID: &tenant.ID,
		Email:    "alice@example.com",
		Name:     "Alice",
		Password: "strongpassword",
		Role:     domain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", u.Email, "alice@example.com")
	}
	if u.PasswordHash == "strongpassword" {
		t.Error("password should be hashed, not plaintext")
	}
	if u.PasswordHash == "" {
		t.Error("password hash should not be empty")
	}
	if u.Status != domain.UserStatusActive {
		t.Errorf("status = %q, want %q", u.Status, domain.UserStatusActive)
	}
}

func TestUserServiceCreateSuperUser(t *testing.T) {
	store, authSvc := newTestStore(t)
	svc := user.NewService(store, authSvc)

	u, err := svc.Create(context.Background(), user.CreateRequest{
		TenantID: nil, // Superuser — no tenant.
		Email:    "super@example.com",
		Name:     "Super Admin",
		Password: "supersecure123",
		Role:     domain.RoleSuperUser,
	})
	if err != nil {
		t.Fatalf("create superuser: %v", err)
	}
	if u.TenantID != nil {
		t.Error("superuser should have nil tenant_id")
	}
}

func TestUserServiceCreateValidation(t *testing.T) {
	store, authSvc := newTestStore(t)
	svc := user.NewService(store, authSvc)
	ctx := context.Background()

	tenantID := domain.NewID()

	tests := []struct {
		name string
		req  user.CreateRequest
	}{
		{"empty email", user.CreateRequest{Email: "", Name: "A", Password: "12345678", Role: domain.RoleAdmin, TenantID: &tenantID}},
		{"invalid email", user.CreateRequest{Email: "not-an-email", Name: "A", Password: "12345678", Role: domain.RoleAdmin, TenantID: &tenantID}},
		{"empty name", user.CreateRequest{Email: "a@b.com", Name: "", Password: "12345678", Role: domain.RoleAdmin, TenantID: &tenantID}},
		{"empty password", user.CreateRequest{Email: "a@b.com", Name: "A", Password: "", Role: domain.RoleAdmin, TenantID: &tenantID}},
		{"short password", user.CreateRequest{Email: "a@b.com", Name: "A", Password: "short", Role: domain.RoleAdmin, TenantID: &tenantID}},
		{"empty role", user.CreateRequest{Email: "a@b.com", Name: "A", Password: "12345678", Role: "", TenantID: &tenantID}},
		{"superuser with tenant", user.CreateRequest{Email: "a@b.com", Name: "A", Password: "12345678", Role: domain.RoleSuperUser, TenantID: &tenantID}},
		{"admin without tenant", user.CreateRequest{Email: "a@b.com", Name: "A", Password: "12345678", Role: domain.RoleAdmin, TenantID: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tt.req)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestUserServiceEmailNormalization(t *testing.T) {
	store, authSvc := newTestStore(t)
	svc := user.NewService(store, authSvc)

	u, err := svc.Create(context.Background(), user.CreateRequest{
		Email:    "  Alice@Example.COM  ",
		Name:     "Alice",
		Password: "password123",
		Role:     domain.RoleSuperUser,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", u.Email, "alice@example.com")
	}
}

func TestUserServiceUpdate(t *testing.T) {
	store, authSvc := newTestStore(t)
	svc := user.NewService(store, authSvc)
	ctx := context.Background()

	u, err := svc.Create(ctx, user.CreateRequest{
		Email:    "update@example.com",
		Name:     "Original",
		Password: "password123",
		Role:     domain.RoleSuperUser,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Updated"
	updated, err := svc.Update(ctx, u.ID, user.UpdateRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Updated")
	}
}
