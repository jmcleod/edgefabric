package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestAPIKeyCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create prerequisite tenant and user.
	tenant := &domain.Tenant{ID: domain.NewID(), Name: "Test", Slug: "test-ak", Status: domain.TenantStatusActive}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: domain.NewID(), TenantID: &tenant.ID, Email: "key@example.com", Name: "Key User", PasswordHash: "hash", Role: domain.RoleAdmin, Status: domain.UserStatusActive}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	apiKey := &domain.APIKey{
		ID:        domain.NewID(),
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Name:      "test-key",
		KeyHash:   "$2a$12$fakehash",
		KeyPrefix: "ef_12345",
		Role:      domain.RoleAdmin,
	}

	// Create.
	if err := store.CreateAPIKey(ctx, apiKey); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	// Get by ID.
	got, err := store.GetAPIKey(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if got.Name != "test-key" {
		t.Errorf("name = %q, want %q", got.Name, "test-key")
	}

	// Get by prefix.
	got2, err := store.GetAPIKeyByPrefix(ctx, "ef_12345")
	if err != nil {
		t.Fatalf("get api key by prefix: %v", err)
	}
	if got2.ID != apiKey.ID {
		t.Errorf("id mismatch from prefix lookup")
	}

	// List by tenant.
	keys, total, err := store.ListAPIKeys(ctx, tenant.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if total != 1 || len(keys) != 1 {
		t.Errorf("list: total=%d, len=%d", total, len(keys))
	}

	// Update last used.
	if err := store.UpdateAPIKeyLastUsed(ctx, apiKey.ID); err != nil {
		t.Fatalf("update last used: %v", err)
	}
	updated, _ := store.GetAPIKey(ctx, apiKey.ID)
	if updated.LastUsedAt == nil {
		t.Errorf("last_used_at should be set after UpdateAPIKeyLastUsed")
	}

	// Delete.
	if err := store.DeleteAPIKey(ctx, apiKey.ID); err != nil {
		t.Fatalf("delete api key: %v", err)
	}
	_, err = store.GetAPIKey(ctx, apiKey.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
