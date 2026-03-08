package networking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// TestNetworkingFullFlow exercises the complete networking lifecycle:
// bootstrap controller peer → create node → generate WG keys →
// create BGP sessions → create IP allocations → generate hub/node configs →
// verify node networking state.
func TestNetworkingFullFlow(t *testing.T) {
	svc, store, sec := newTestEnv(t)
	ctx := context.Background()

	// Step 1: Bootstrap controller WireGuard peer.
	ctrlPeer, err := networking.BootstrapControllerPeer(ctx, store, sec, testWGConfig, domain.ControllerPeerID)
	if err != nil {
		t.Fatalf("bootstrap controller peer: %v", err)
	}
	if ctrlPeer.OwnerType != domain.PeerOwnerController {
		t.Errorf("expected controller owner type, got %s", ctrlPeer.OwnerType)
	}

	// Step 2: Create a node with a tenant.
	tenantID := createTestTenant(t, store)
	node := createTestNode(t, store, &tenantID)

	// Step 3: Generate WireGuard keys for the node (mimics provisioning step).
	kp, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("generate WG keys: %v", err)
	}
	psk, err := provisioning.GeneratePresharedKey()
	if err != nil {
		t.Fatalf("generate PSK: %v", err)
	}
	encPriv, err := sec.Encrypt(kp.PrivateKey)
	if err != nil {
		t.Fatalf("encrypt private key: %v", err)
	}
	encPSK, err := sec.Encrypt(psk)
	if err != nil {
		t.Fatalf("encrypt PSK: %v", err)
	}

	nodePeer := &domain.WireGuardPeer{
		ID:           domain.NewID(),
		OwnerType:    domain.PeerOwnerNode,
		OwnerID:      node.ID,
		PublicKey:     kp.PublicKey,
		PrivateKey:    encPriv,
		PresharedKey:  encPSK,
		AllowedIPs:    []string{"10.100.0.2/32"},
		Endpoint:      "203.0.113.1:51820",
	}
	if err := store.CreateWireGuardPeer(ctx, nodePeer); err != nil {
		t.Fatalf("create node peer: %v", err)
	}

	// Assign overlay IP to node.
	node.WireGuardIP = "10.100.0.2"
	if err := store.UpdateNode(ctx, node); err != nil {
		t.Fatalf("update node WG IP: %v", err)
	}

	// Step 4: Create BGP sessions for the node.
	sess1, err := svc.CreateBGPSession(ctx, networking.CreateBGPSessionRequest{
		NodeID:            node.ID,
		PeerASN:           65001,
		PeerAddress:       "192.0.2.1",
		LocalASN:          65000,
		AnnouncedPrefixes: []string{"10.0.0.0/24", "10.1.0.0/24"},
	})
	if err != nil {
		t.Fatalf("create BGP session 1: %v", err)
	}

	sess2, err := svc.CreateBGPSession(ctx, networking.CreateBGPSessionRequest{
		NodeID:      node.ID,
		PeerASN:     65002,
		PeerAddress: "192.0.2.2",
		LocalASN:    65000,
	})
	if err != nil {
		t.Fatalf("create BGP session 2: %v", err)
	}

	// Step 5: Create IP allocations for the tenant.
	alloc, err := svc.CreateIPAllocation(ctx, networking.CreateIPAllocationRequest{
		TenantID: tenantID,
		Prefix:   "10.0.0.0/24",
		Type:     domain.IPAllocationAnycast,
		Purpose:  domain.IPPurposeDNS,
	})
	if err != nil {
		t.Fatalf("create IP allocation: %v", err)
	}

	// Step 6: Generate hub WireGuard config.
	hubConf, err := svc.GenerateHubConfig(ctx)
	if err != nil {
		t.Fatalf("generate hub config: %v", err)
	}

	// Verify hub config contains expected sections.
	if !strings.Contains(hubConf, "[Interface]") {
		t.Error("hub config missing [Interface]")
	}
	if !strings.Contains(hubConf, "[Peer]") {
		t.Error("hub config missing [Peer]")
	}
	if !strings.Contains(hubConf, "ListenPort = 51820") {
		t.Error("hub config missing ListenPort")
	}
	if !strings.Contains(hubConf, nodePeer.PublicKey) {
		t.Error("hub config missing node's public key")
	}

	// Step 7: Generate node WireGuard config.
	nodeConf, err := svc.GenerateNodeConfig(ctx, node.ID)
	if err != nil {
		t.Fatalf("generate node config: %v", err)
	}

	if !strings.Contains(nodeConf, "[Interface]") {
		t.Error("node config missing [Interface]")
	}
	if !strings.Contains(nodeConf, "Address = 10.100.0.2/32") {
		t.Error("node config missing correct address")
	}
	if !strings.Contains(nodeConf, ctrlPeer.PublicKey) {
		t.Error("node config missing controller's public key")
	}
	if !strings.Contains(nodeConf, "PersistentKeepalive = 25") {
		t.Error("node config missing PersistentKeepalive")
	}

	// Step 8: Verify node networking state.
	state, err := svc.GetNodeNetworkingState(ctx, node.ID)
	if err != nil {
		t.Fatalf("get networking state: %v", err)
	}

	if state.NodeID != node.ID {
		t.Errorf("expected node ID %s, got %s", node.ID, state.NodeID)
	}
	if state.WireGuardIP != "10.100.0.2" {
		t.Errorf("expected WG IP 10.100.0.2, got %s", state.WireGuardIP)
	}
	if state.WireGuardPeer == nil {
		t.Fatal("expected WireGuard peer in state")
	}
	if state.WireGuardPeer.PublicKey != kp.PublicKey {
		t.Error("WG peer public key mismatch")
	}
	if len(state.BGPSessions) != 2 {
		t.Errorf("expected 2 BGP sessions, got %d", len(state.BGPSessions))
	}

	// Step 9: Verify list operations.
	peers, total, err := svc.ListWireGuardPeers(ctx, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if total != 2 { // controller + node
		t.Errorf("expected 2 total peers, got %d", total)
	}
	if len(peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(peers))
	}

	sessions, total, err := svc.ListBGPSessions(ctx, node.ID, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("list BGP sessions: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 BGP sessions, got %d", total)
	}

	allocations, total, err := svc.ListIPAllocations(ctx, tenantID, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("list IP allocations: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 IP allocation, got %d", total)
	}

	// Step 10: Update and delete to complete lifecycle.
	_, err = svc.UpdateBGPSession(ctx, sess2.ID, networking.UpdateBGPSessionRequest{
		AnnouncedPrefixes: []string{"10.2.0.0/24"},
	})
	if err != nil {
		t.Fatalf("update BGP session: %v", err)
	}

	if err := svc.DeleteBGPSession(ctx, sess1.ID); err != nil {
		t.Fatalf("delete BGP session: %v", err)
	}

	if err := svc.DeleteIPAllocation(ctx, alloc.ID); err != nil {
		t.Fatalf("delete IP allocation: %v", err)
	}

	// Verify only 1 session remains.
	sessions, total, err = svc.ListBGPSessions(ctx, node.ID, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 session after delete, got %d", total)
	}
	if len(sessions) > 0 && len(sessions[0].AnnouncedPrefixes) != 1 {
		t.Errorf("expected 1 announced prefix after update, got %d", len(sessions[0].AnnouncedPrefixes))
	}

	// Suppress unused variable warnings.
	_ = allocations
}
