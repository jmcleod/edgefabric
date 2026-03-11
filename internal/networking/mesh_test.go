package networking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

// meshTestWGConfig is a mesh-topology WireGuard config for tests.
var meshTestWGConfig = config.WireGuardHub{
	ListenPort: 51820,
	Subnet:     "10.100.0.0/16",
	Address:    "10.100.0.1/16",
	Topology:   "mesh",
}

func newMeshTestEnv(t *testing.T) (networking.Service, *sqlite.SQLiteStore, *secrets.Store) {
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

	svc := networking.NewService(store, store, store, store, store, sec, meshTestWGConfig)
	return svc, store, sec
}

func createTestNodeWithIP(t *testing.T, store *sqlite.SQLiteStore, tenantID *domain.ID, name, publicIP, wgIP string) *domain.Node {
	t.Helper()
	node := &domain.Node{
		ID:          domain.NewID(),
		TenantID:    tenantID,
		Name:        name,
		Hostname:    name + ".example.com",
		PublicIP:    publicIP,
		WireGuardIP: wgIP,
		SSHPort:     22,
		SSHUser:     "root",
		Status:      domain.NodeStatusOnline,
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create node %s: %v", name, err)
	}
	return node
}

func createWGPeerForNode(t *testing.T, store *sqlite.SQLiteStore, sec *secrets.Store, node *domain.Node) {
	t.Helper()
	kp, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	encPriv, err := sec.Encrypt(kp.PrivateKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	peer := &domain.WireGuardPeer{
		ID:         domain.NewID(),
		OwnerType:  domain.PeerOwnerNode,
		OwnerID:    node.ID,
		PublicKey:   kp.PublicKey,
		PrivateKey:  encPriv,
		AllowedIPs:  []string{node.WireGuardIP + "/32"},
		Endpoint:    node.PublicIP + ":51820",
	}
	if err := store.CreateWireGuardPeer(context.Background(), peer); err != nil {
		t.Fatalf("create WG peer for %s: %v", node.Name, err)
	}
}

func createControllerPeer(t *testing.T, store *sqlite.SQLiteStore, sec *secrets.Store) {
	t.Helper()
	kp, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	encPriv, err := sec.Encrypt(kp.PrivateKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	peer := &domain.WireGuardPeer{
		ID:         domain.ControllerPeerID,
		OwnerType:  domain.PeerOwnerController,
		OwnerID:    domain.ControllerPeerID,
		PublicKey:   kp.PublicKey,
		PrivateKey:  encPriv,
		AllowedIPs:  []string{"10.100.0.1/32"},
		Endpoint:    "controller.example.com:51820",
	}
	if err := store.CreateWireGuardPeer(context.Background(), peer); err != nil {
		t.Fatalf("create controller peer: %v", err)
	}
}

func createNodeGroup(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID, name string, nodeIDs ...domain.ID) *domain.NodeGroup {
	t.Helper()
	ctx := context.Background()
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     name,
	}
	if err := store.CreateNodeGroup(ctx, group); err != nil {
		t.Fatalf("create node group: %v", err)
	}
	for _, nid := range nodeIDs {
		if err := store.AddNodeToGroup(ctx, group.ID, nid); err != nil {
			t.Fatalf("add node to group: %v", err)
		}
	}
	return group
}

func TestGenerateMeshConfig_SingleGroup(t *testing.T) {
	svc, store, sec := newMeshTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	// Create controller peer.
	createControllerPeer(t, store, sec)

	// Create 3 nodes with WireGuard IPs.
	node1 := createTestNodeWithIP(t, store, &tenantID, "node1", "203.0.113.1", "10.100.0.2")
	node2 := createTestNodeWithIP(t, store, &tenantID, "node2", "203.0.113.2", "10.100.0.3")
	node3 := createTestNodeWithIP(t, store, &tenantID, "node3", "203.0.113.3", "10.100.0.4")

	// Create WG peers for all nodes.
	createWGPeerForNode(t, store, sec, node1)
	createWGPeerForNode(t, store, sec, node2)
	createWGPeerForNode(t, store, sec, node3)

	// Put all 3 nodes in a single group.
	createNodeGroup(t, store, tenantID, "mesh-group", node1.ID, node2.ID, node3.ID)

	// Generate config for node1 — should get hub + node2 + node3 as peers.
	cfgStr, err := svc.GenerateNodeConfig(ctx, node1.ID)
	if err != nil {
		t.Fatalf("generate node config: %v", err)
	}

	// Should have 3 peers: hub + 2 mesh peers.
	peerCount := strings.Count(cfgStr, "[Peer]")
	if peerCount != 3 {
		t.Errorf("expected 3 peer sections (hub + 2 mesh), got %d", peerCount)
	}

	// Should have a ListenPort (mesh nodes must listen).
	if !strings.Contains(cfgStr, "ListenPort = 51820") {
		t.Error("mesh node config should have ListenPort")
	}

	// Should contain hub AllowedIPs (the full subnet).
	if !strings.Contains(cfgStr, "10.100.0.0/16") {
		t.Error("expected hub AllowedIPs for full subnet")
	}

	// Should contain mesh peer /32 AllowedIPs.
	if !strings.Contains(cfgStr, "10.100.0.3/32") {
		t.Error("expected mesh peer node2 AllowedIPs /32")
	}
	if !strings.Contains(cfgStr, "10.100.0.4/32") {
		t.Error("expected mesh peer node3 AllowedIPs /32")
	}

	// Mesh peers should have PersistentKeepalive.
	keepaliveCount := strings.Count(cfgStr, "PersistentKeepalive = 25")
	if keepaliveCount < 3 {
		t.Errorf("expected at least 3 PersistentKeepalive entries (hub + mesh peers), got %d", keepaliveCount)
	}
}

func TestGenerateMeshConfig_NoGroups(t *testing.T) {
	svc, store, sec := newMeshTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	// Create controller peer.
	createControllerPeer(t, store, sec)

	// Create a node with NO group membership.
	node := createTestNodeWithIP(t, store, &tenantID, "lonely-node", "203.0.113.10", "10.100.0.10")
	createWGPeerForNode(t, store, sec, node)

	cfgStr, err := svc.GenerateNodeConfig(ctx, node.ID)
	if err != nil {
		t.Fatalf("generate node config: %v", err)
	}

	// Should have only 1 peer: the hub. No mesh peers.
	peerCount := strings.Count(cfgStr, "[Peer]")
	if peerCount != 1 {
		t.Errorf("expected 1 peer section (hub only), got %d", peerCount)
	}

	// Hub-only node should NOT have a ListenPort (no incoming connections).
	// (listenPort will be 0 when no mesh peers exist)
	if strings.Contains(cfgStr, "ListenPort") {
		t.Error("hub-only node should not have ListenPort when no mesh peers")
	}
}

func TestGenerateMeshConfig_HubAlwaysIncluded(t *testing.T) {
	svc, store, sec := newMeshTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	// Create controller peer.
	createControllerPeer(t, store, sec)

	// Create 2 nodes in a group.
	node1 := createTestNodeWithIP(t, store, &tenantID, "node-a", "203.0.113.20", "10.100.0.20")
	node2 := createTestNodeWithIP(t, store, &tenantID, "node-b", "203.0.113.21", "10.100.0.21")
	createWGPeerForNode(t, store, sec, node1)
	createWGPeerForNode(t, store, sec, node2)
	createNodeGroup(t, store, tenantID, "group-a", node1.ID, node2.ID)

	cfgStr, err := svc.GenerateNodeConfig(ctx, node1.ID)
	if err != nil {
		t.Fatalf("generate node config: %v", err)
	}

	// Should have 2 peers: hub + 1 mesh peer.
	peerCount := strings.Count(cfgStr, "[Peer]")
	if peerCount != 2 {
		t.Errorf("expected 2 peer sections (hub + 1 mesh), got %d", peerCount)
	}

	// Hub endpoint should always be present.
	if !strings.Contains(cfgStr, "Endpoint = controller.example.com:51820") {
		t.Error("hub endpoint should always be present in mesh config")
	}

	// Hub's AllowedIPs should be the full subnet.
	if !strings.Contains(cfgStr, "10.100.0.0/16") {
		t.Error("hub AllowedIPs should cover the full overlay subnet")
	}
}
