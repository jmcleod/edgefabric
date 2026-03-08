package networking_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/storage"
)

var testWGConfig = config.WireGuardHub{
	ListenPort: 51820,
	Subnet:     "10.100.0.0/16",
	Address:    "10.100.0.1/16",
}

func TestBootstrapControllerPeer_CreatesNew(t *testing.T) {
	_, store, sec := newTestEnv(t)
	ctx := context.Background()

	peer, err := networking.BootstrapControllerPeer(ctx, store, sec, testWGConfig, domain.ControllerPeerID)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if peer.OwnerType != domain.PeerOwnerController {
		t.Errorf("expected owner type controller, got %s", peer.OwnerType)
	}
	if peer.OwnerID != domain.ControllerPeerID {
		t.Errorf("expected owner ID %s, got %s", domain.ControllerPeerID, peer.OwnerID)
	}
	if peer.PublicKey == "" {
		t.Error("expected non-empty public key")
	}
	if peer.PrivateKey == "" {
		t.Error("expected non-empty (encrypted) private key")
	}
	if len(peer.AllowedIPs) == 0 {
		t.Error("expected at least one allowed IP")
	}

	// Verify we can decrypt the private key.
	plainPriv, err := sec.Decrypt(peer.PrivateKey)
	if err != nil {
		t.Fatalf("decrypt private key: %v", err)
	}
	if plainPriv == "" {
		t.Error("expected non-empty plaintext private key")
	}

	// Verify it's stored.
	stored, err := store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerController, domain.ControllerPeerID)
	if err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if stored.PublicKey != peer.PublicKey {
		t.Error("stored public key should match")
	}
}

func TestBootstrapControllerPeer_Idempotent(t *testing.T) {
	_, store, sec := newTestEnv(t)
	ctx := context.Background()

	// First call creates.
	first, err := networking.BootstrapControllerPeer(ctx, store, sec, testWGConfig, domain.ControllerPeerID)
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}

	// Second call returns the same peer.
	second, err := networking.BootstrapControllerPeer(ctx, store, sec, testWGConfig, domain.ControllerPeerID)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected same peer ID on second call: %s vs %s", first.ID, second.ID)
	}
	if first.PublicKey != second.PublicKey {
		t.Error("expected same public key on second call")
	}

	// Verify only one peer exists.
	peers, total, err := store.ListWireGuardPeers(ctx, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(peers) != 1 {
		t.Errorf("expected exactly 1 peer, got total=%d len=%d", total, len(peers))
	}
}
