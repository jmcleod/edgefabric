package dns_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// TestIntegrationFullDNSFlow exercises the complete DNS flow from zone creation
// through record management, serial tracking, and node config sync.
func TestIntegrationFullDNSFlow(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// 1. Create tenant.
	tenantID := createTestTenant(t, store)

	// 2. Create node group and add a node.
	groupID := createTestNodeGroup(t, store, tenantID)
	nodeID := createTestNode(t, store, tenantID)

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// 3. Create DNS zone assigned to the group.
	ttl := 600
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID:    tenantID,
		Name:        "integration-test.example.com",
		TTL:         ttl,
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	if zone.Serial != 1 {
		t.Errorf("expected initial serial 1, got %d", zone.Serial)
	}
	if zone.TTL != 600 {
		t.Errorf("expected TTL 600, got %d", zone.TTL)
	}

	// 4. Create various record types.
	priority10 := 10
	weight60 := 60
	port5060 := 5060

	records := []dns.CreateRecordRequest{
		{ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.1"},
		{ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.2"},
		{ZoneID: zone.ID, Name: "ipv6", Type: domain.DNSRecordTypeAAAA, Value: "2001:db8::1"},
		{ZoneID: zone.ID, Name: "mail", Type: domain.DNSRecordTypeMX, Value: "mx1.example.com", Priority: &priority10},
		{ZoneID: zone.ID, Name: "alias", Type: domain.DNSRecordTypeCNAME, Value: "www.integration-test.example.com"},
		{ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeTXT, Value: "v=spf1 include:_spf.example.com ~all"},
		{ZoneID: zone.ID, Name: "_sip._tcp", Type: domain.DNSRecordTypeSRV, Value: "sip.example.com", Priority: &priority10, Weight: &weight60, Port: &port5060},
	}

	var createdRecords []*domain.DNSRecord
	for _, req := range records {
		rec, err := svc.CreateRecord(ctx, req)
		if err != nil {
			t.Fatalf("create record %s/%s: %v", req.Name, req.Type, err)
		}
		createdRecords = append(createdRecords, rec)
	}

	// 5. Verify serial increments after record creation.
	got, err := svc.GetZone(ctx, zone.ID)
	if err != nil {
		t.Fatalf("get zone: %v", err)
	}
	expectedSerial := uint32(1 + len(records)) // 1 initial + 7 record creates
	if got.Serial != expectedSerial {
		t.Errorf("expected serial %d after creates, got %d", expectedSerial, got.Serial)
	}

	// 6. Get node DNS config and verify.
	config, err := svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node dns config: %v", err)
	}
	if len(config.Zones) != 1 {
		t.Fatalf("expected 1 zone in config, got %d", len(config.Zones))
	}
	configZone := config.Zones[0]
	if configZone.Zone.Name != "integration-test.example.com" {
		t.Errorf("expected zone name integration-test.example.com, got %s", configZone.Zone.Name)
	}
	if len(configZone.Records) != len(records) {
		t.Errorf("expected %d records in config, got %d", len(records), len(configZone.Records))
	}
	if configZone.Zone.Serial != expectedSerial {
		t.Errorf("expected config serial %d, got %d", expectedSerial, configZone.Zone.Serial)
	}

	// 7. Update a record → serial increments.
	newValue := "198.51.100.1"
	_, err = svc.UpdateRecord(ctx, createdRecords[0].ID, dns.UpdateRecordRequest{Value: &newValue})
	if err != nil {
		t.Fatalf("update record: %v", err)
	}

	got, _ = svc.GetZone(ctx, zone.ID)
	if got.Serial != expectedSerial+1 {
		t.Errorf("expected serial %d after update, got %d", expectedSerial+1, got.Serial)
	}

	// 8. Delete a record → serial increments.
	if err := svc.DeleteRecord(ctx, createdRecords[1].ID); err != nil {
		t.Fatalf("delete record: %v", err)
	}

	got, _ = svc.GetZone(ctx, zone.ID)
	if got.Serial != expectedSerial+2 {
		t.Errorf("expected serial %d after delete, got %d", expectedSerial+2, got.Serial)
	}

	// 9. Verify updated config reflects changes.
	config, err = svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get updated node dns config: %v", err)
	}
	if len(config.Zones[0].Records) != len(records)-1 {
		t.Errorf("expected %d records after delete, got %d", len(records)-1, len(config.Zones[0].Records))
	}

	// 10. Delete zone → cascade deletes records.
	if err := svc.DeleteZone(ctx, zone.ID); err != nil {
		t.Fatalf("delete zone: %v", err)
	}

	_, err = svc.GetZone(ctx, zone.ID)
	if err == nil {
		t.Error("expected zone to be deleted")
	}

	// Node config should now be empty.
	config, err = svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get config after zone delete: %v", err)
	}
	if len(config.Zones) != 0 {
		t.Errorf("expected 0 zones after delete, got %d", len(config.Zones))
	}
}

// TestIntegrationMultipleZonesPerNode verifies that a node in multiple groups
// gets zones from all assigned groups.
func TestIntegrationMultipleZonesPerNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	group1ID := createTestNodeGroup(t, store, tenantID)
	group2ID := createTestNodeGroup(t, store, tenantID)

	// Add node to both groups.
	if err := store.AddNodeToGroup(ctx, group1ID, nodeID); err != nil {
		t.Fatalf("add node to group1: %v", err)
	}
	if err := store.AddNodeToGroup(ctx, group2ID, nodeID); err != nil {
		t.Fatalf("add node to group2: %v", err)
	}

	// Create a zone per group.
	_, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID, Name: "zone1.example.com", NodeGroupID: &group1ID,
	})
	if err != nil {
		t.Fatalf("create zone1: %v", err)
	}
	_, err = svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID, Name: "zone2.example.com", NodeGroupID: &group2ID,
	})
	if err != nil {
		t.Fatalf("create zone2: %v", err)
	}

	config, err := svc.GetNodeDNSConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node dns config: %v", err)
	}
	if len(config.Zones) != 2 {
		t.Errorf("expected 2 zones from multiple groups, got %d", len(config.Zones))
	}
}

// TestIntegrationZoneUpdate verifies updating zone properties.
func TestIntegrationZoneUpdate(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "update-test.example.com",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	// Update zone name.
	newName := "updated.example.com"
	updated, err := svc.UpdateZone(ctx, zone.ID, dns.UpdateZoneRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update zone: %v", err)
	}
	if updated.Name != "updated.example.com" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}

	// Update zone TTL.
	newTTL := 1200
	updated, err = svc.UpdateZone(ctx, zone.ID, dns.UpdateZoneRequest{TTL: &newTTL})
	if err != nil {
		t.Fatalf("update zone TTL: %v", err)
	}
	if updated.TTL != 1200 {
		t.Errorf("expected TTL 1200, got %d", updated.TTL)
	}

	// Disable zone.
	disabled := domain.DNSZoneDisabled
	updated, err = svc.UpdateZone(ctx, zone.ID, dns.UpdateZoneRequest{Status: &disabled})
	if err != nil {
		t.Fatalf("disable zone: %v", err)
	}
	if updated.Status != domain.DNSZoneDisabled {
		t.Errorf("expected disabled status, got %s", updated.Status)
	}
}

// TestIntegrationValidationEdgeCases tests various validation edge cases.
func TestIntegrationValidationEdgeCases(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	// Bad zone names.
	badNames := []string{"", "bad zone!", "-leading-dash.com", ".leading-dot.com"}
	for _, name := range badNames {
		_, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
			TenantID: tenantID,
			Name:     name,
		})
		if err == nil {
			t.Errorf("expected error for bad zone name %q", name)
		}
	}

	// Create zone for record validation.
	zone, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenantID,
		Name:     "validation.example.com",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}

	// Bad A record (not an IP).
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "not-an-ip",
	})
	if err == nil {
		t.Error("expected error for bad A record value")
	}

	// IPv6 in A record.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "::1",
	})
	if err == nil {
		t.Error("expected error for IPv6 in A record")
	}

	// IPv4 in AAAA record.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeAAAA, Value: "1.2.3.4",
	})
	if err == nil {
		t.Error("expected error for IPv4 in AAAA record")
	}

	// MX without priority.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeMX, Value: "mail.example.com",
	})
	if err == nil {
		t.Error("expected error for MX without priority")
	}

	// SRV without port.
	priority := 10
	weight := 60
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "_sip._tcp", Type: domain.DNSRecordTypeSRV, Value: "sip.example.com",
		Priority: &priority, Weight: &weight,
	})
	if err == nil {
		t.Error("expected error for SRV without port")
	}

	// CNAME exclusivity: create A then try CNAME at same name.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "excl", Type: domain.DNSRecordTypeA, Value: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create A for exclusivity test: %v", err)
	}
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "excl", Type: domain.DNSRecordTypeCNAME, Value: "other.example.com",
	})
	if err == nil {
		t.Error("expected CNAME exclusivity error")
	}

	// CAA bad format.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeCAA, Value: "bad-format",
	})
	if err == nil {
		t.Error("expected error for bad CAA format")
	}

	// Record for nonexistent zone.
	_, err = svc.CreateRecord(ctx, dns.CreateRecordRequest{
		ZoneID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4",
	})
	if err == nil {
		t.Error("expected error for nonexistent zone")
	}
}

// TestIntegrationTenantIsolation ensures zones from one tenant don't leak
// to nodes of another tenant.
func TestIntegrationTenantIsolation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// Create two tenants.
	tenant1ID := createTestTenant(t, store)
	tenant2ID := createTestTenant(t, store)

	// Tenant1 setup.
	node1ID := createTestNode(t, store, tenant1ID)
	group1ID := createTestNodeGroup(t, store, tenant1ID)
	if err := store.AddNodeToGroup(ctx, group1ID, node1ID); err != nil {
		t.Fatalf("add node1 to group1: %v", err)
	}

	// Tenant2 setup.
	node2ID := createTestNode(t, store, tenant2ID)
	group2ID := createTestNodeGroup(t, store, tenant2ID)
	if err := store.AddNodeToGroup(ctx, group2ID, node2ID); err != nil {
		t.Fatalf("add node2 to group2: %v", err)
	}

	// Create zones per tenant.
	_, err := svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenant1ID, Name: "tenant1.example.com", NodeGroupID: &group1ID,
	})
	if err != nil {
		t.Fatalf("create tenant1 zone: %v", err)
	}
	_, err = svc.CreateZone(ctx, dns.CreateZoneRequest{
		TenantID: tenant2ID, Name: "tenant2.example.com", NodeGroupID: &group2ID,
	})
	if err != nil {
		t.Fatalf("create tenant2 zone: %v", err)
	}

	// Node1 should only see tenant1's zone.
	config1, err := svc.GetNodeDNSConfig(ctx, node1ID)
	if err != nil {
		t.Fatalf("get config for node1: %v", err)
	}
	if len(config1.Zones) != 1 || config1.Zones[0].Zone.Name != "tenant1.example.com" {
		t.Errorf("node1 should only see tenant1 zone, got %d zones", len(config1.Zones))
	}

	// Node2 should only see tenant2's zone.
	config2, err := svc.GetNodeDNSConfig(ctx, node2ID)
	if err != nil {
		t.Fatalf("get config for node2: %v", err)
	}
	if len(config2.Zones) != 1 || config2.Zones[0].Zone.Name != "tenant2.example.com" {
		t.Errorf("node2 should only see tenant2 zone, got %d zones", len(config2.Zones))
	}

	// Verify zone listing is tenant-scoped.
	zones1, total1, err := svc.ListZones(ctx, tenant1ID, storage.ListParams{Limit: 50})
	if err != nil {
		t.Fatalf("list zones tenant1: %v", err)
	}
	if total1 != 1 || len(zones1) != 1 {
		t.Errorf("expected 1 zone for tenant1, got total=%d len=%d", total1, len(zones1))
	}

	zones2, total2, err := svc.ListZones(ctx, tenant2ID, storage.ListParams{Limit: 50})
	if err != nil {
		t.Fatalf("list zones tenant2: %v", err)
	}
	if total2 != 1 || len(zones2) != 1 {
		t.Errorf("expected 1 zone for tenant2, got total=%d len=%d", total2, len(zones2))
	}
}
