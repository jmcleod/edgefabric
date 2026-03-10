package provisioning

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/crypto"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

const (
	// EnrollmentTokenTTL is the default enrollment token validity period.
	EnrollmentTokenTTL = 1 * time.Hour

	// enrollmentTokenLength is the byte length of the random token.
	enrollmentTokenLength = 32
)

// EnrollmentResult is returned by CompleteEnrollment with the data the
// node agent needs to start polling for configuration.
type EnrollmentResult struct {
	NodeID     domain.ID  `json:"node_id"`
	TenantID   *domain.ID `json:"tenant_id,omitempty"`
	WireGuardIP string    `json:"wireguard_ip"`
}

// GenerateEnrollmentToken creates a one-time enrollment token for a node.
func (p *DefaultProvisioner) GenerateEnrollmentToken(ctx context.Context, tenantID, targetID domain.ID) (*domain.EnrollmentToken, error) {
	tokenStr, err := crypto.GenerateRandomString(enrollmentTokenLength)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	token := &domain.EnrollmentToken{
		ID:         domain.NewID(),
		TenantID:   tenantID,
		TargetType: domain.EnrollmentTargetNode,
		TargetID:   targetID,
		Token:      tokenStr,
		ExpiresAt:  time.Now().UTC().Add(EnrollmentTokenTTL),
	}

	if err := p.tokens.CreateEnrollmentToken(ctx, token); err != nil {
		return nil, fmt.Errorf("store enrollment token: %w", err)
	}
	return token, nil
}

// ValidateEnrollmentToken checks that a token exists, is not expired, and has not been used.
func (p *DefaultProvisioner) ValidateEnrollmentToken(ctx context.Context, tokenStr string) (*domain.EnrollmentToken, error) {
	token, err := p.tokens.GetEnrollmentToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("get enrollment token: %w", err)
	}

	if token.UsedAt != nil {
		return nil, fmt.Errorf("%w: enrollment token already used", storage.ErrConflict)
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, fmt.Errorf("%w: enrollment token expired", storage.ErrConflict)
	}
	return token, nil
}

// CompleteEnrollment validates the token, generates WireGuard keys,
// allocates an overlay IP, marks the token used, and transitions the node to online.
// Returns an EnrollmentResult so the node agent can persist its identity and start
// polling for configuration.
func (p *DefaultProvisioner) CompleteEnrollment(ctx context.Context, tokenStr string) (*EnrollmentResult, error) {
	// Validate token.
	token, err := p.ValidateEnrollmentToken(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	// Get the target node.
	node, err := p.nodes.GetNode(ctx, token.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get target node: %w", err)
	}

	// Generate WireGuard keys if not already assigned.
	_, err = p.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, node.ID)
	if err == storage.ErrNotFound {
		// No peer yet — generate keys + allocate IP.
		kp, err := GenerateWireGuardKeyPair()
		if err != nil {
			return nil, fmt.Errorf("generate WG key pair: %w", err)
		}

		psk, err := GeneratePresharedKey()
		if err != nil {
			return nil, fmt.Errorf("generate preshared key: %w", err)
		}

		encPriv, err := p.secrets.Encrypt(kp.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt WG private key: %w", err)
		}

		encPSK, err := p.secrets.Encrypt(psk)
		if err != nil {
			return nil, fmt.Errorf("encrypt preshared key: %w", err)
		}

		peers, _, err := p.peers.ListWireGuardPeers(ctx, storage.ListParams{Limit: 10000})
		if err != nil {
			return nil, fmt.Errorf("list WG peers: %w", err)
		}
		nodes, _, err := p.nodes.ListNodes(ctx, nil, storage.ListParams{Limit: 10000})
		if err != nil {
			return nil, fmt.Errorf("list nodes: %w", err)
		}

		overlayIP, err := AllocateOverlayIP(p.wgConfig.Subnet, p.wgConfig.Address, peers, nodes)
		if err != nil {
			return nil, fmt.Errorf("allocate overlay IP: %w", err)
		}

		peer := &domain.WireGuardPeer{
			ID:           domain.NewID(),
			OwnerType:    domain.PeerOwnerNode,
			OwnerID:      node.ID,
			PublicKey:     kp.PublicKey,
			PrivateKey:    encPriv,
			PresharedKey:  encPSK,
			AllowedIPs:    []string{overlayIP + "/32"},
			Endpoint:      fmt.Sprintf("%s:%d", node.PublicIP, p.wgConfig.ListenPort),
		}
		if err := p.peers.CreateWireGuardPeer(ctx, peer); err != nil {
			return nil, fmt.Errorf("create WG peer: %w", err)
		}

		node.WireGuardIP = overlayIP
	} else if err != nil {
		return nil, fmt.Errorf("check existing WG peer: %w", err)
	}

	// Mark token as used.
	if err := p.tokens.MarkEnrollmentTokenUsed(ctx, token.ID); err != nil {
		return nil, fmt.Errorf("mark token used: %w", err)
	}

	// Transition node to online.
	node.Status = domain.NodeStatusOnline
	if err := p.nodes.UpdateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("update node status: %w", err)
	}

	return &EnrollmentResult{
		NodeID:      node.ID,
		TenantID:    node.TenantID,
		WireGuardIP: node.WireGuardIP,
	}, nil
}
