package dns_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

func newTestEnv(t *testing.T) (dns.Service, *sqlite.SQLiteStore) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := dns.NewService(store, store, store, store)
	return svc, store
}

func createTestTenant(t *testing.T, store *sqlite.SQLiteStore) domain.ID {
	t.Helper()
	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "test-tenant-" + domain.NewID().String()[:8],
		Slug: "test-" + domain.NewID().String()[:8],
	}
	if err := store.CreateTenant(context.Background(), tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant.ID
}

func createTestNodeGroup(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-group-" + domain.NewID().String()[:8],
	}
	if err := store.CreateNodeGroup(context.Background(), group); err != nil {
		t.Fatalf("create node group: %v", err)
	}
	return group.ID
}

func createTestNode(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: &tenantID,
		Name:     "test-node-" + domain.NewID().String()[:8],
		Hostname: "node.example.com",
		PublicIP: "203.0.113.1",
		SSHPort:  22,
		SSHUser:  "root",
		Status:   domain.NodeStatusOnline,
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.ID
}

func TestCreateZone_HappyPath(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "example.com",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	if zone.Name != "example.com" {
		t.Errorf("expected name example.com, got %s", zone.Name)
	}
	if zone.Status != domain.DNSZoneActive {
		t.Errorf("expected active status, got %s", zone.Status)
	}
	if zone.Serial != 1 {
		t.Errorf("expected serial 1, got %d", zone.Serial)
	}

	// Verify via Get.
	got, err := svc.GetZone(ctx, zone.ID)
	if err != nil {
		t.Fatalf("get zone: %v", err)
	}
	if got.Name != "example.com" {
		t.Errorf("expected name example.com, got %s", got.Name)
	}
}

func TestCreateZone_Validation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	tests := []struct {
		name string
		req  dns.CreateZoneRequest
	}{
		{"empty name", dns.CreateZoneRequest{TenantID: tenantID, Name: ""}},
		{"invalid name", dns.CreateZoneRequest{TenantID: tenantID, Name: "not a valid zone!"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateZone(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCreateRecord_HappyPath(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "example.com",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	record, err := svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "www",
		Type:   domain.DNSRecordTypeA,
		Value:  "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if record.Name != "www" {
		t.Errorf("expected name www, got %s", record.Name)
	}

	// Verify via Get.
	got, err := svc.GetRecord(ctx, record.ID)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if got.Value != "203.0.113.10" {
		t.Errorf("expected value 203.0.113.10, got %s", got.Value)
	}
}

func TestCreateRecord_Validation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "example.com",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	tests := []struct {
		name string
		req  dns.CreateRecordRequest
	}{
		{"bad A record value", dns.CreateRecordRequest{ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "not-an-ip"}},
		{"IPv6 in A record", dns.CreateRecordRequest{ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "2001:db8::1"}},
		{"IPv4 in AAAA record", dns.CreateRecordRequest{ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeAAAA, Value: "1.2.3.4"}},
		{"MX missing priority", dns.CreateRecordRequest{ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeMX, Value: "mail.example.com"}},
		{"nonexistent zone", dns.CreateRecordRequest{ZoneID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateRecord(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestRecordMutationIncrementsSerial(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zone, _ := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "serial-test.example.com",
	})

	// Serial starts at 1.
	got, _ := svc.GetZone(ctx, zone.ID)
	if got.Serial != 1 {
		t.Fatalf("expected initial serial 1, got %d", got.Serial)
	}

	// Create record → serial becomes 2.
	record, err := svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "www",
		Type:   domain.DNSRecordTypeA,
		Value:  "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}

	got, _ = svc.GetZone(ctx, zone.ID)
	if got.Serial != 2 {
		t.Errorf("expected serial 2 after create, got %d", got.Serial)
	}

	// Update record → serial becomes 3.
	newValue := "5.6.7.8"
	_, err = svc.UpdateRecord(ctx, record.ID, dns.UpdateRecordRequest{Value: &newValue})
	if err != nil {
		t.Fatalf("update record: %v", err)
	}

	got, _ = svc.GetZone(ctx, zone.ID)
	if got.Serial != 3 {
		t.Errorf("expected serial 3 after update, got %d", got.Serial)
	}

	// Delete record → serial becomes 4.
	if err := svc.DeleteRecord(ctx, record.ID); err != nil {
		t.Fatalf("delete record: %v", err)
	}

	got, _ = svc.GetZone(ctx, zone.ID)
	if got.Serial != 4 {
		t.Errorf("expected serial 4 after delete, got %d", got.Serial)
	}
}

func TestCNAMEExclusivity(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zone, _ := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "cname-test.example.com",
	})

	// Create an A record at "www".
	_, err := svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "www",
		Type:   domain.DNSRecordTypeA,
		Value:  "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create A record: %v", err)
	}

	// Try to create a CNAME at "www" — should fail.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "www",
		Type:   domain.DNSRecordTypeCNAME,
		Value:  "other.example.com",
	})
	if err == nil {
		t.Error("expected CNAME exclusivity error")
	}

	// Create CNAME at "alias" — should succeed.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "alias",
		Type:   domain.DNSRecordTypeCNAME,
		Value:  "other.example.com",
	})
	if err != nil {
		t.Fatalf("expected CNAME at different name to succeed: %v", err)
	}

	// Try to create A at "alias" — should fail (CNAME exists).
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID,
		Name:   "alias",
		Type:   domain.DNSRecordTypeA,
		Value:  "5.6.7.8",
	})
	if err == nil {
		t.Error("expected error: can't add A where CNAME exists")
	}
}

func TestGetNodeDNSConfig(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	// Add node to group.
	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create a zone assigned to the group.
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID:    tenantID,
		Name:        "example.com",
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	// Create records.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create A record: %v", err)
	}
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeTXT, Value: "v=spf1 ~all",
	})
	if err != nil {
		t.Fatalf("create TXT record: %v", err)
	}

	// Get node DNS config.
	config, err := svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node dns config: %v", err)
	}
	if len(config.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(config.Zones))
	}
	if config.Zones[0].Zone.Name != "example.com" {
		t.Errorf("expected zone example.com, got %s", config.Zones[0].Zone.Name)
	}
	if len(config.Zones[0].Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(config.Zones[0].Records))
	}
}

func TestGetNodeDNSConfig_NoZonesForUnassignedNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	// Create a zone assigned to the group, but don't add the node.
	_, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID:    tenantID,
		Name:        "not-for-this-node.example.com",
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	config, err := svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node dns config: %v", err)
	}
	if len(config.Zones) != 0 {
		t.Errorf("expected 0 zones for unassigned node, got %d", len(config.Zones))
	}
}

func TestGetNodeDNSConfig_DisabledZoneExcluded(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create zone and disable it.
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID:    tenantID,
		Name:        "disabled.example.com",
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	disabled := domain.DNSZoneDisabled
	_, err = svc.UpdateZone(ctx, zone.ID, dns.UpdateZoneRequest{Status: &disabled})
	if err != nil {
		t.Fatalf("disable zone: %v", err)
	}

	config, err := svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node dns config: %v", err)
	}
	if len(config.Zones) != 0 {
		t.Errorf("expected 0 zones (disabled excluded), got %d", len(config.Zones))
	}
}

func TestListZones(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	for i := 0; i < 3; i++ {
		_, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
			TenantID: tenantID,
			Name:     fmt.Sprintf("zone%d.example.com", i),
		})
		if err != nil {
			t.Fatalf("create zone %d: %v", i, err)
		}
	}

	zones, total, err := svc.ListZones(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list zones: %v", err)
	}
	if total != 3 || len(zones) != 3 {
		t.Errorf("expected 3 zones, got total=%d len=%d", total, len(zones))
	}
}

func TestListRecords(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	zone, _ := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "records-test.example.com",
	})

	for _, name := range []string{"www", "mail", "ftp"} {
		_, err := svc.CreateRecord(ctx, dns.CreateRecordRequest{
			ZoneID: zone.ID, Name: name, Type: domain.DNSRecordTypeA, Value: "1.2.3.4",
		})
		if err != nil {
			t.Fatalf("create record %s: %v", name, err)
		}
	}

	records, total, err := svc.ListRecords(ctx, zone.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if total != 3 || len(records) != 3 {
		t.Errorf("expected 3 records, got total=%d len=%d", total, len(records))
	}
}
