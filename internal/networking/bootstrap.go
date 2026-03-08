package networking

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// BootstrapControllerPeer ensures the controller has a WireGuard peer record.
// This is called once at controller startup and is idempotent — if the peer
// already exists, it returns the existing record.
func BootstrapControllerPeer(
	ctx context.Context,
	peers storage.WireGuardPeerStore,
	sec *secrets.Store,
	wgConfig config.WireGuardHub,
	controllerID domain.ID,
) (*domain.WireGuardPeer, error) {
	// Check if controller peer already exists.
	existing, err := peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerController, controllerID)
	if err == nil {
		return existing, nil // Already bootstrapped.
	}
	if err != storage.ErrNotFound {
		return nil, fmt.Errorf("check controller peer: %w", err)
	}

	// Generate WireGuard key pair.
	kp, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate wireguard keys: %w", err)
	}

	// Generate preshared key.
	psk, err := provisioning.GeneratePresharedKey()
	if err != nil {
		return nil, fmt.Errorf("generate preshared key: %w", err)
	}

	// Encrypt private key and PSK.
	encPriv, err := sec.Encrypt(kp.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}
	encPSK, err := sec.Encrypt(psk)
	if err != nil {
		return nil, fmt.Errorf("encrypt preshared key: %w", err)
	}

	// Build endpoint from config.
	endpoint := ""
	if wgConfig.ListenPort > 0 {
		endpoint = fmt.Sprintf(":%d", wgConfig.ListenPort)
	}

	peer := &domain.WireGuardPeer{
		ID:           domain.NewID(),
		OwnerType:    domain.PeerOwnerController,
		OwnerID:      controllerID,
		PublicKey:     kp.PublicKey,
		PrivateKey:    encPriv,
		PresharedKey:  encPSK,
		AllowedIPs:    []string{wgConfig.Subnet},
		Endpoint:      endpoint,
	}

	if err := peers.CreateWireGuardPeer(ctx, peer); err != nil {
		return nil, fmt.Errorf("create controller peer: %w", err)
	}

	return peer, nil
}
