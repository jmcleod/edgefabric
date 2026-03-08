package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func createTestDNSZone(t *testing.T, store interface {
	CreateDNSZone(ctx context.Context, z *domain.DNSZone) error
}, tenantID domain.ID) domain.ID {
	t.Helper()
	zone := &domain.DNSZone{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-" + domain.NewID().String()[:8] + ".example.com",
	}
	if err := store.CreateDNSZone(context.Background(), zone); err != nil {
		t.Fatalf("create test dns zone: %v", err)
	}
	return zone.ID
}

func TestDNSRecordCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zoneID := createTestDNSZone(t, store, tenantID)

	ttl := 300
	record := &domain.DNSRecord{
		ID:     domain.NewID(),
		ZoneID: zoneID,
		Name:   "www",
		Type:   domain.DNSRecordTypeA,
		Value:  "203.0.113.10",
		TTL:    &ttl,
	}

	// Create.
	if err := store.CreateDNSRecord(ctx, record); err != nil {
		t.Fatalf("create dns record: %v", err)
	}
	if record.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Get by ID.
	got, err := store.GetDNSRecord(ctx, record.ID)
	if err != nil {
		t.Fatalf("get dns record: %v", err)
	}
	if got.Name != "www" {
		t.Errorf("expected name www, got %s", got.Name)
	}
	if got.Type != domain.DNSRecordTypeA {
		t.Errorf("expected type A, got %s", got.Type)
	}
	if got.Value != "203.0.113.10" {
		t.Errorf("expected value 203.0.113.10, got %s", got.Value)
	}
	if got.TTL == nil || *got.TTL != 300 {
		t.Errorf("expected TTL 300, got %v", got.TTL)
	}
	if got.ZoneID != zoneID {
		t.Errorf("expected zone_id %s, got %s", zoneID, got.ZoneID)
	}

	// List by zone.
	records, total, err := store.ListDNSRecords(ctx, zoneID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list dns records: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Errorf("expected 1 record, got total=%d len=%d", total, len(records))
	}

	// Update — change value.
	got.Value = "203.0.113.20"
	newTTL := 600
	got.TTL = &newTTL
	if err := store.UpdateDNSRecord(ctx, got); err != nil {
		t.Fatalf("update dns record: %v", err)
	}

	updated, err := store.GetDNSRecord(ctx, record.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Value != "203.0.113.20" {
		t.Errorf("expected value 203.0.113.20, got %s", updated.Value)
	}
	if updated.TTL == nil || *updated.TTL != 600 {
		t.Errorf("expected TTL 600, got %v", updated.TTL)
	}

	// Delete.
	if err := store.DeleteDNSRecord(ctx, record.ID); err != nil {
		t.Fatalf("delete dns record: %v", err)
	}
	_, err = store.GetDNSRecord(ctx, record.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDNSRecordNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetDNSRecord(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDNSRecordDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteDNSRecord(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteZoneCascadesRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zoneID := createTestDNSZone(t, store, tenantID)

	// Create two records in the zone.
	for _, name := range []string{"www", "mail"} {
		record := &domain.DNSRecord{
			ID:     domain.NewID(),
			ZoneID: zoneID,
			Name:   name,
			Type:   domain.DNSRecordTypeA,
			Value:  "203.0.113.10",
		}
		if err := store.CreateDNSRecord(ctx, record); err != nil {
			t.Fatalf("create record %s: %v", name, err)
		}
	}

	// Verify records exist.
	records, total, err := store.ListDNSRecords(ctx, zoneID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(records) != 2 {
		t.Fatalf("expected 2 records, got total=%d len=%d", total, len(records))
	}

	// Delete the zone — should cascade delete records.
	if err := store.DeleteDNSZone(ctx, zoneID); err != nil {
		t.Fatalf("delete zone: %v", err)
	}

	// Records should be gone.
	records, total, err = store.ListDNSRecords(ctx, zoneID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list after zone delete: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Errorf("expected 0 records after zone delete, got total=%d len=%d", total, len(records))
	}
}

func TestDNSRecordWithMXPriority(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zoneID := createTestDNSZone(t, store, tenantID)

	priority := 10
	record := &domain.DNSRecord{
		ID:       domain.NewID(),
		ZoneID:   zoneID,
		Name:     "@",
		Type:     domain.DNSRecordTypeMX,
		Value:    "mail.example.com",
		Priority: &priority,
	}
	if err := store.CreateDNSRecord(ctx, record); err != nil {
		t.Fatalf("create MX record: %v", err)
	}

	got, err := store.GetDNSRecord(ctx, record.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Priority == nil || *got.Priority != 10 {
		t.Errorf("expected priority 10, got %v", got.Priority)
	}
}

func TestDNSRecordWithSRV(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zoneID := createTestDNSZone(t, store, tenantID)

	priority := 10
	weight := 60
	port := 5060
	record := &domain.DNSRecord{
		ID:       domain.NewID(),
		ZoneID:   zoneID,
		Name:     "_sip._tcp",
		Type:     domain.DNSRecordTypeSRV,
		Value:    "sipserver.example.com",
		Priority: &priority,
		Weight:   &weight,
		Port:     &port,
	}
	if err := store.CreateDNSRecord(ctx, record); err != nil {
		t.Fatalf("create SRV record: %v", err)
	}

	got, err := store.GetDNSRecord(ctx, record.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Priority == nil || *got.Priority != 10 {
		t.Errorf("expected priority 10, got %v", got.Priority)
	}
	if got.Weight == nil || *got.Weight != 60 {
		t.Errorf("expected weight 60, got %v", got.Weight)
	}
	if got.Port == nil || *got.Port != 5060 {
		t.Errorf("expected port 5060, got %v", got.Port)
	}
}
