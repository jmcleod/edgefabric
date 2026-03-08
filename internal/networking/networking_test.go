package networking_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

// testKey is a base64-encoded 32-byte AES key for tests.
const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 zero bytes

func newTestEnv(t *testing.T) (networking.Service, *sqlite.SQLiteStore, *secrets.Store) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sec, err := secrets.NewStore(testKey)
	if err != nil {
		t.Fatalf("create secrets: %v", err)
	}

	wgCfg := config.WireGuardHub{
		ListenPort: 51820,
		Subnet:     "10.100.0.0/16",
		Address:    "10.100.0.1/16",
	}

	svc := networking.NewService(store, store, store, store, sec, wgCfg)
	return svc, store, sec
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

func createTestNode(t *testing.T, store *sqlite.SQLiteStore, tenantID *domain.ID) *domain.Node {
	t.Helper()
	node := &domain.Node{
		ID:          domain.NewID(),
		TenantID:    tenantID,
		Name:        "test-node-" + domain.NewID().String()[:8],
		Hostname:    "node.example.com",
		PublicIP:    "203.0.113.1",
		WireGuardIP: "10.100.0.5",
		SSHPort:     22,
		SSHUser:     "root",
		Status:      domain.NodeStatusOnline,
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node
}

func TestCreateBGPSession_Validation(t *testing.T) {
	svc, store, _ := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	node := createTestNode(t, store, &tenantID)

	tests := []struct {
		name string
		req  networking.CreateBGPSessionRequest
	}{
		{"missing peer_asn", networking.CreateBGPSessionRequest{NodeID: node.ID, PeerAddress: "1.2.3.4", LocalASN: 65000}},
		{"missing peer_address", networking.CreateBGPSessionRequest{NodeID: node.ID, PeerASN: 65001, LocalASN: 65000}},
		{"invalid peer_address", networking.CreateBGPSessionRequest{NodeID: node.ID, PeerASN: 65001, PeerAddress: "not-an-ip", LocalASN: 65000}},
		{"missing local_asn", networking.CreateBGPSessionRequest{NodeID: node.ID, PeerASN: 65001, PeerAddress: "1.2.3.4"}},
		{"missing node", networking.CreateBGPSessionRequest{NodeID: domain.NewID(), PeerASN: 65001, PeerAddress: "1.2.3.4", LocalASN: 65000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateBGPSession(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCreateBGPSession_HappyPath(t *testing.T) {
	svc, store, _ := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	node := createTestNode(t, store, &tenantID)

	sess, err := svc.CreateBGPSession(ctx, networking.CreateBGPSessionRequest{
		NodeID:            node.ID,
		PeerASN:           65001,
		PeerAddress:       "198.51.100.1",
		LocalASN:          65000,
		AnnouncedPrefixes: []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sess.Status != domain.BGPSessionConfigured {
		t.Errorf("expected configured status, got %s", sess.Status)
	}

	// Verify via Get.
	got, err := svc.GetBGPSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PeerASN != 65001 {
		t.Errorf("expected peer_asn 65001, got %d", got.PeerASN)
	}
}

func TestCreateIPAllocation_Validation(t *testing.T) {
	svc, store, _ := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	tests := []struct {
		name string
		req  networking.CreateIPAllocationRequest
	}{
		{"missing prefix", networking.CreateIPAllocationRequest{TenantID: tenantID, Type: domain.IPAllocationAnycast, Purpose: domain.IPPurposeDNS}},
		{"invalid prefix", networking.CreateIPAllocationRequest{TenantID: tenantID, Prefix: "not-a-cidr", Type: domain.IPAllocationAnycast, Purpose: domain.IPPurposeDNS}},
		{"missing type", networking.CreateIPAllocationRequest{TenantID: tenantID, Prefix: "203.0.113.0/24", Purpose: domain.IPPurposeDNS}},
		{"missing purpose", networking.CreateIPAllocationRequest{TenantID: tenantID, Prefix: "203.0.113.0/24", Type: domain.IPAllocationAnycast}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateIPAllocation(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCreateIPAllocation_HappyPath(t *testing.T) {
	svc, store, _ := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	alloc, err := svc.CreateIPAllocation(ctx, networking.CreateIPAllocationRequest{
		TenantID: tenantID,
		Prefix:   "203.0.113.0/24",
		Type:     domain.IPAllocationAnycast,
		Purpose:  domain.IPPurposeDNS,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if alloc.Status != domain.IPAllocationPending {
		t.Errorf("expected pending status, got %s", alloc.Status)
	}

	// Verify via list.
	allocations, total, err := svc.ListIPAllocations(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(allocations) != 1 {
		t.Errorf("expected 1 allocation, got total=%d len=%d", total, len(allocations))
	}
}

func TestGetNodeNetworkingState(t *testing.T) {
	svc, store, _ := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	node := createTestNode(t, store, &tenantID)

	// Create a BGP session for the node.
	_, err := svc.CreateBGPSession(ctx, networking.CreateBGPSessionRequest{
		NodeID:      node.ID,
		PeerASN:     65001,
		PeerAddress: "198.51.100.1",
		LocalASN:    65000,
	})
	if err != nil {
		t.Fatalf("create bgp session: %v", err)
	}

	state, err := svc.GetNodeNetworkingState(ctx, node.ID)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	if state.NodeID != node.ID {
		t.Errorf("expected node_id %s, got %s", node.ID, state.NodeID)
	}
	if state.WireGuardIP != "10.100.0.5" {
		t.Errorf("expected WireGuardIP 10.100.0.5, got %s", state.WireGuardIP)
	}
	if len(state.BGPSessions) != 1 {
		t.Errorf("expected 1 BGP session, got %d", len(state.BGPSessions))
	}
}
