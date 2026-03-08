package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestIPAllocationCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	alloc := &domain.IPAllocation{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Prefix:   "203.0.113.0/24",
		Type:     domain.IPAllocationAnycast,
		Purpose:  domain.IPPurposeDNS,
	}

	// Create.
	if err := store.CreateIPAllocation(ctx, alloc); err != nil {
		t.Fatalf("create ip allocation: %v", err)
	}
	if alloc.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if alloc.Status != domain.IPAllocationPending {
		t.Errorf("expected default status pending, got %s", alloc.Status)
	}

	// Get by ID.
	got, err := store.GetIPAllocation(ctx, alloc.ID)
	if err != nil {
		t.Fatalf("get ip allocation: %v", err)
	}
	if got.Prefix != "203.0.113.0/24" {
		t.Errorf("expected prefix 203.0.113.0/24, got %s", got.Prefix)
	}
	if got.Type != domain.IPAllocationAnycast {
		t.Errorf("expected type anycast, got %s", got.Type)
	}
	if got.Purpose != domain.IPPurposeDNS {
		t.Errorf("expected purpose dns, got %s", got.Purpose)
	}
	if got.TenantID != tenantID {
		t.Errorf("expected tenant_id %s, got %s", tenantID, got.TenantID)
	}

	// List by tenant.
	allocations, total, err := store.ListIPAllocations(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list ip allocations: %v", err)
	}
	if total != 1 || len(allocations) != 1 {
		t.Errorf("expected 1 allocation, got total=%d len=%d", total, len(allocations))
	}

	// Update — change status to active.
	got.Status = domain.IPAllocationActive
	if err := store.UpdateIPAllocation(ctx, got); err != nil {
		t.Fatalf("update ip allocation: %v", err)
	}

	updated, err := store.GetIPAllocation(ctx, alloc.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Status != domain.IPAllocationActive {
		t.Errorf("expected status active, got %s", updated.Status)
	}

	// Delete.
	if err := store.DeleteIPAllocation(ctx, alloc.ID); err != nil {
		t.Fatalf("delete ip allocation: %v", err)
	}
	_, err = store.GetIPAllocation(ctx, alloc.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestIPAllocationNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetIPAllocation(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIPAllocationDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteIPAllocation(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIPAllocationTenantIsolation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenant1 := createTestTenant(t, store)
	tenant2 := createTestTenant(t, store)

	// Create allocation for tenant1.
	alloc := &domain.IPAllocation{
		ID:       domain.NewID(),
		TenantID: tenant1,
		Prefix:   "203.0.113.0/24",
		Type:     domain.IPAllocationAnycast,
		Purpose:  domain.IPPurposeCDN,
	}
	if err := store.CreateIPAllocation(ctx, alloc); err != nil {
		t.Fatalf("create: %v", err)
	}

	// List for tenant2 should return 0.
	allocations, total, err := store.ListIPAllocations(ctx, tenant2, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(allocations) != 0 {
		t.Errorf("expected 0 allocations for tenant2, got total=%d len=%d", total, len(allocations))
	}
}
