package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestUserCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a tenant for the user.
	tenant := &domain.Tenant{ID: domain.NewID(), Name: "Test", Slug: "test", Status: domain.TenantStatusActive}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	user := &domain.User{
		ID:           domain.NewID(),
		TenantID:     &tenant.ID,
		Email:        "alice@example.com",
		Name:         "Alice",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehas",
		Role:         domain.RoleAdmin,
		Status:       domain.UserStatusActive,
	}

	// Create.
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Get by ID.
	got, err := store.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "alice@example.com")
	}
	if got.TenantID == nil || *got.TenantID != tenant.ID {
		t.Errorf("tenant_id mismatch")
	}

	// Get by email.
	got2, err := store.GetUserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if got2.ID != user.ID {
		t.Errorf("id mismatch")
	}

	// List all.
	users, total, err := store.ListUsers(ctx, nil, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(users) != 1 {
		t.Errorf("len = %d, want 1", len(users))
	}

	// List by tenant.
	users2, total2, err := store.ListUsers(ctx, &tenant.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list users by tenant: %v", err)
	}
	if total2 != 1 || len(users2) != 1 {
		t.Errorf("tenant-scoped list: total=%d, len=%d", total2, len(users2))
	}

	// Update.
	got.Name = "Alice B"
	if err := store.UpdateUser(ctx, got); err != nil {
		t.Fatalf("update user: %v", err)
	}
	updated, _ := store.GetUser(ctx, user.ID)
	if updated.Name != "Alice B" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Alice B")
	}

	// Delete.
	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	_, err = store.GetUser(ctx, user.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestUserDuplicateEmail(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	u1 := &domain.User{ID: domain.NewID(), Email: "dup@example.com", Name: "U1", PasswordHash: "hash1", Role: domain.RoleSuperUser, Status: domain.UserStatusActive}
	u2 := &domain.User{ID: domain.NewID(), Email: "dup@example.com", Name: "U2", PasswordHash: "hash2", Role: domain.RoleSuperUser, Status: domain.UserStatusActive}

	if err := store.CreateUser(ctx, u1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	err := store.CreateUser(ctx, u2)
	if !errors.Is(err, storage.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists for duplicate email, got %v", err)
	}
}

func TestSuperUserWithoutTenant(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	user := &domain.User{
		ID:           domain.NewID(),
		TenantID:     nil, // Superuser — no tenant.
		Email:        "super@example.com",
		Name:         "Super",
		PasswordHash: "hash",
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusActive,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create superuser: %v", err)
	}

	got, err := store.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get superuser: %v", err)
	}
	if got.TenantID != nil {
		t.Errorf("superuser should have nil tenant_id")
	}
}
