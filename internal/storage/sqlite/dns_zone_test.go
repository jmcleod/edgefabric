package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func createTestNodeGroup(t *testing.T, store interface {
	CreateNodeGroup(ctx context.Context, g *domain.NodeGroup) error
}, tenantID domain.ID) domain.ID {
	t.Helper()
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-group-" + domain.NewID().String()[:8],
	}
	if err := store.CreateNodeGroup(context.Background(), group); err != nil {
		t.Fatalf("create test node group: %v", err)
	}
	return group.ID
}

func TestDNSZoneCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	zone := &domain.DNSZone{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "example.com",
	}

	// Create.
	if err := store.CreateDNSZone(ctx, zone); err != nil {
		t.Fatalf("create dns zone: %v", err)
	}
	if zone.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if zone.Status != domain.DNSZoneActive {
		t.Errorf("expected default status active, got %s", zone.Status)
	}
	if zone.Serial != 1 {
		t.Errorf("expected default serial 1, got %d", zone.Serial)
	}
	if zone.TTL != 3600 {
		t.Errorf("expected default TTL 3600, got %d", zone.TTL)
	}

	// Get by ID.
	got, err := store.GetDNSZone(ctx, zone.ID)
	if err != nil {
		t.Fatalf("get dns zone: %v", err)
	}
	if got.Name != "example.com" {
		t.Errorf("expected name example.com, got %s", got.Name)
	}
	if got.TenantID != tenantID {
		t.Errorf("expected tenant_id %s, got %s", tenantID, got.TenantID)
	}
	if got.Serial != 1 {
		t.Errorf("expected serial 1, got %d", got.Serial)
	}

	// List by tenant.
	zones, total, err := store.ListDNSZones(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list dns zones: %v", err)
	}
	if total != 1 || len(zones) != 1 {
		t.Errorf("expected 1 zone, got total=%d len=%d", total, len(zones))
	}

	// Update — change name and TTL.
	got.Name = "updated.example.com"
	got.TTL = 7200
	if err := store.UpdateDNSZone(ctx, got); err != nil {
		t.Fatalf("update dns zone: %v", err)
	}

	updated, err := store.GetDNSZone(ctx, zone.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Name != "updated.example.com" {
		t.Errorf("expected name updated.example.com, got %s", updated.Name)
	}
	if updated.TTL != 7200 {
		t.Errorf("expected TTL 7200, got %d", updated.TTL)
	}

	// Delete.
	if err := store.DeleteDNSZone(ctx, zone.ID); err != nil {
		t.Fatalf("delete dns zone: %v", err)
	}
	_, err = store.GetDNSZone(ctx, zone.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDNSZoneNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetDNSZone(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDNSZoneDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteDNSZone(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDNSZoneIncrementSerial(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	zone := &domain.DNSZone{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "serial.example.com",
	}
	if err := store.CreateDNSZone(ctx, zone); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Initial serial is 1.
	got, _ := store.GetDNSZone(ctx, zone.ID)
	if got.Serial != 1 {
		t.Fatalf("expected initial serial 1, got %d", got.Serial)
	}

	// Increment serial.
	if err := store.IncrementDNSZoneSerial(ctx, zone.ID); err != nil {
		t.Fatalf("increment serial: %v", err)
	}

	got, _ = store.GetDNSZone(ctx, zone.ID)
	if got.Serial != 2 {
		t.Errorf("expected serial 2, got %d", got.Serial)
	}

	// Increment again.
	if err := store.IncrementDNSZoneSerial(ctx, zone.ID); err != nil {
		t.Fatalf("increment serial again: %v", err)
	}

	got, _ = store.GetDNSZone(ctx, zone.ID)
	if got.Serial != 3 {
		t.Errorf("expected serial 3, got %d", got.Serial)
	}

	// Increment on non-existent zone.
	err := store.IncrementDNSZoneSerial(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound for non-existent zone, got %v", err)
	}
}

func TestDNSZoneWithNodeGroup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	groupID := createTestNodeGroup(t, store, tenantID)

	zone := &domain.DNSZone{
		ID:          domain.NewID(),
		TenantID:    tenantID,
		Name:        "grouped.example.com",
		NodeGroupID: &groupID,
	}
	if err := store.CreateDNSZone(ctx, zone); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetDNSZone(ctx, zone.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.NodeGroupID == nil {
		t.Fatal("expected node_group_id to be set")
	}
	if *got.NodeGroupID != groupID {
		t.Errorf("expected node_group_id %s, got %s", groupID, *got.NodeGroupID)
	}
}

func TestDNSZoneTenantIsolation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenant1 := createTestTenant(t, store)
	tenant2 := createTestTenant(t, store)

	zone := &domain.DNSZone{
		ID:       domain.NewID(),
		TenantID: tenant1,
		Name:     "isolated.example.com",
	}
	if err := store.CreateDNSZone(ctx, zone); err != nil {
		t.Fatalf("create: %v", err)
	}

	// List for tenant2 should return 0.
	zones, total, err := store.ListDNSZones(ctx, tenant2, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(zones) != 0 {
		t.Errorf("expected 0 zones for tenant2, got total=%d len=%d", total, len(zones))
	}
}
