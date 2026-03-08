package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestWireGuardPeerCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)

	peer := &domain.WireGuardPeer{
		ID:        domain.NewID(),
		OwnerType: domain.PeerOwnerNode,
		OwnerID:   nodeID,
		PublicKey:  "test-public-key-base64",
		PrivateKey: "encrypted-private-key-data",
		PresharedKey: "encrypted-preshared-key",
		AllowedIPs: []string{"10.100.0.2/32", "192.168.1.0/24"},
		Endpoint:   "203.0.113.1:51820",
	}

	// Create.
	if err := store.CreateWireGuardPeer(ctx, peer); err != nil {
		t.Fatalf("create wireguard peer: %v", err)
	}
	if peer.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Get by ID.
	got, err := store.GetWireGuardPeer(ctx, peer.ID)
	if err != nil {
		t.Fatalf("get wireguard peer: %v", err)
	}
	if got.PublicKey != "test-public-key-base64" {
		t.Errorf("expected public key test-public-key-base64, got %s", got.PublicKey)
	}
	if got.PrivateKey != "encrypted-private-key-data" {
		t.Error("private key should be stored and retrievable")
	}
	if got.PresharedKey != "encrypted-preshared-key" {
		t.Error("preshared key should be stored and retrievable")
	}
	if len(got.AllowedIPs) != 2 {
		t.Errorf("expected 2 allowed IPs, got %d", len(got.AllowedIPs))
	}
	if got.Endpoint != "203.0.113.1:51820" {
		t.Errorf("expected endpoint 203.0.113.1:51820, got %s", got.Endpoint)
	}

	// Get by owner.
	got, err = store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, nodeID)
	if err != nil {
		t.Fatalf("get by owner: %v", err)
	}
	if got.ID != peer.ID {
		t.Errorf("expected peer ID %s, got %s", peer.ID, got.ID)
	}

	// List.
	peers, total, err := store.ListWireGuardPeers(ctx, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list wireguard peers: %v", err)
	}
	if total != 1 || len(peers) != 1 {
		t.Errorf("expected 1 peer, got total=%d len=%d", total, len(peers))
	}

	// Update — change public key and allowed IPs.
	got.PublicKey = "updated-public-key"
	got.AllowedIPs = []string{"10.100.0.2/32"}
	if err := store.UpdateWireGuardPeer(ctx, got); err != nil {
		t.Fatalf("update wireguard peer: %v", err)
	}

	updated, err := store.GetWireGuardPeer(ctx, peer.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.PublicKey != "updated-public-key" {
		t.Errorf("expected updated-public-key, got %s", updated.PublicKey)
	}
	if len(updated.AllowedIPs) != 1 {
		t.Errorf("expected 1 allowed IP after update, got %d", len(updated.AllowedIPs))
	}

	// Delete.
	if err := store.DeleteWireGuardPeer(ctx, peer.ID); err != nil {
		t.Fatalf("delete wireguard peer: %v", err)
	}
	_, err = store.GetWireGuardPeer(ctx, peer.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestWireGuardPeerNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetWireGuardPeer(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWireGuardPeerByOwnerNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
