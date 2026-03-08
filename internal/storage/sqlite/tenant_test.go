package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestTenantCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{
		ID:     domain.NewID(),
		Name:   "Acme Corp",
		Slug:   "acme-corp",
		Status: domain.TenantStatusActive,
	}

	// Create.
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// Get by ID.
	got, err := store.GetTenant(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("name = %q, want %q", got.Name, "Acme Corp")
	}
	if got.Slug != "acme-corp" {
		t.Errorf("slug = %q, want %q", got.Slug, "acme-corp")
	}

	// Get by slug.
	got2, err := store.GetTenantBySlug(ctx, "acme-corp")
	if err != nil {
		t.Fatalf("get tenant by slug: %v", err)
	}
	if got2.ID != tenant.ID {
		t.Errorf("id mismatch")
	}

	// List.
	tenants, total, err := store.ListTenants(ctx, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(tenants) != 1 {
		t.Errorf("len = %d, want 1", len(tenants))
	}

	// Update.
	got.Name = "Acme Inc"
	if err := store.UpdateTenant(ctx, got); err != nil {
		t.Fatalf("update tenant: %v", err)
	}
	updated, _ := store.GetTenant(ctx, tenant.ID)
	if updated.Name != "Acme Inc" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Acme Inc")
	}

	// Delete.
	if err := store.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	_, err = store.GetTenant(ctx, tenant.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTenantDuplicateSlug(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	t1 := &domain.Tenant{ID: domain.NewID(), Name: "T1", Slug: "same-slug", Status: domain.TenantStatusActive}
	t2 := &domain.Tenant{ID: domain.NewID(), Name: "T2", Slug: "same-slug", Status: domain.TenantStatusActive}

	if err := store.CreateTenant(ctx, t1); err != nil {
		t.Fatalf("create t1: %v", err)
	}
	err := store.CreateTenant(ctx, t2)
	if !errors.Is(err, storage.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists for duplicate slug, got %v", err)
	}
}

func TestTenantNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetTenant(ctx, domain.NewID())
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = store.GetTenantBySlug(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound for slug, got %v", err)
	}
}
