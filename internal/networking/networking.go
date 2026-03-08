package networking

import (
	"context"
	"fmt"
	"net"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Ensure DefaultService implements Service at compile time.
var _ Service = (*DefaultService)(nil)

// DefaultService implements the networking Service interface.
type DefaultService struct {
	nodes    storage.NodeStore
	peers    storage.WireGuardPeerStore
	bgp      storage.BGPSessionStore
	ips      storage.IPAllocationStore
	secrets  *secrets.Store
	wgConfig config.WireGuardHub
}

// NewService creates a new DefaultService.
func NewService(
	nodes storage.NodeStore,
	peers storage.WireGuardPeerStore,
	bgp storage.BGPSessionStore,
	ips storage.IPAllocationStore,
	secrets *secrets.Store,
	wgConfig config.WireGuardHub,
) Service {
	return &DefaultService{
		nodes:    nodes,
		peers:    peers,
		bgp:      bgp,
		ips:      ips,
		secrets:  secrets,
		wgConfig: wgConfig,
	}
}

// --- BGP Session CRUD ---

func (s *DefaultService) CreateBGPSession(ctx context.Context, req CreateBGPSessionRequest) (*domain.BGPSession, error) {
	// Validate node exists.
	if _, err := s.nodes.GetNode(ctx, req.NodeID); err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}
	if req.PeerASN == 0 {
		return nil, fmt.Errorf("peer_asn is required")
	}
	if req.PeerAddress == "" {
		return nil, fmt.Errorf("peer_address is required")
	}
	if ip := net.ParseIP(req.PeerAddress); ip == nil {
		return nil, fmt.Errorf("invalid peer_address: %q", req.PeerAddress)
	}
	if req.LocalASN == 0 {
		return nil, fmt.Errorf("local_asn is required")
	}

	sess := &domain.BGPSession{
		ID:                domain.NewID(),
		NodeID:            req.NodeID,
		PeerASN:           req.PeerASN,
		PeerAddress:       req.PeerAddress,
		LocalASN:          req.LocalASN,
		Status:            domain.BGPSessionConfigured,
		AnnouncedPrefixes: req.AnnouncedPrefixes,
		ImportPolicy:      req.ImportPolicy,
		ExportPolicy:      req.ExportPolicy,
	}

	if err := s.bgp.CreateBGPSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("create bgp session: %w", err)
	}
	return sess, nil
}

func (s *DefaultService) GetBGPSession(ctx context.Context, id domain.ID) (*domain.BGPSession, error) {
	return s.bgp.GetBGPSession(ctx, id)
}

func (s *DefaultService) ListBGPSessions(ctx context.Context, nodeID domain.ID, params storage.ListParams) ([]*domain.BGPSession, int, error) {
	return s.bgp.ListBGPSessions(ctx, nodeID, params)
}

func (s *DefaultService) UpdateBGPSession(ctx context.Context, id domain.ID, req UpdateBGPSessionRequest) (*domain.BGPSession, error) {
	sess, err := s.bgp.GetBGPSession(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.PeerASN != nil {
		sess.PeerASN = *req.PeerASN
	}
	if req.PeerAddress != nil {
		if ip := net.ParseIP(*req.PeerAddress); ip == nil {
			return nil, fmt.Errorf("invalid peer_address: %q", *req.PeerAddress)
		}
		sess.PeerAddress = *req.PeerAddress
	}
	if req.LocalASN != nil {
		sess.LocalASN = *req.LocalASN
	}
	if req.Status != nil {
		sess.Status = *req.Status
	}
	if req.AnnouncedPrefixes != nil {
		sess.AnnouncedPrefixes = req.AnnouncedPrefixes
	}
	if req.ImportPolicy != nil {
		sess.ImportPolicy = *req.ImportPolicy
	}
	if req.ExportPolicy != nil {
		sess.ExportPolicy = *req.ExportPolicy
	}

	if err := s.bgp.UpdateBGPSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("update bgp session: %w", err)
	}
	return sess, nil
}

func (s *DefaultService) DeleteBGPSession(ctx context.Context, id domain.ID) error {
	return s.bgp.DeleteBGPSession(ctx, id)
}

// --- IP Allocation CRUD ---

func (s *DefaultService) CreateIPAllocation(ctx context.Context, req CreateIPAllocationRequest) (*domain.IPAllocation, error) {
	if req.Prefix == "" {
		return nil, fmt.Errorf("prefix is required")
	}
	if _, _, err := net.ParseCIDR(req.Prefix); err != nil {
		return nil, fmt.Errorf("invalid CIDR prefix: %w", err)
	}
	if req.Type == "" {
		return nil, fmt.Errorf("type is required")
	}
	if req.Purpose == "" {
		return nil, fmt.Errorf("purpose is required")
	}

	alloc := &domain.IPAllocation{
		ID:       domain.NewID(),
		TenantID: req.TenantID,
		Prefix:   req.Prefix,
		Type:     req.Type,
		Purpose:  req.Purpose,
		Status:   domain.IPAllocationPending,
	}

	if err := s.ips.CreateIPAllocation(ctx, alloc); err != nil {
		return nil, fmt.Errorf("create ip allocation: %w", err)
	}
	return alloc, nil
}

func (s *DefaultService) GetIPAllocation(ctx context.Context, id domain.ID) (*domain.IPAllocation, error) {
	return s.ips.GetIPAllocation(ctx, id)
}

func (s *DefaultService) ListIPAllocations(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.IPAllocation, int, error) {
	return s.ips.ListIPAllocations(ctx, tenantID, params)
}

func (s *DefaultService) UpdateIPAllocation(ctx context.Context, id domain.ID, req UpdateIPAllocationRequest) (*domain.IPAllocation, error) {
	alloc, err := s.ips.GetIPAllocation(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Status != nil {
		alloc.Status = *req.Status
	}

	if err := s.ips.UpdateIPAllocation(ctx, alloc); err != nil {
		return nil, fmt.Errorf("update ip allocation: %w", err)
	}
	return alloc, nil
}

func (s *DefaultService) DeleteIPAllocation(ctx context.Context, id domain.ID) error {
	return s.ips.DeleteIPAllocation(ctx, id)
}

// --- WireGuard ---

func (s *DefaultService) ListWireGuardPeers(ctx context.Context, params storage.ListParams) ([]*domain.WireGuardPeer, int, error) {
	return s.peers.ListWireGuardPeers(ctx, params)
}

func (s *DefaultService) GenerateHubConfig(ctx context.Context) (string, error) {
	// Get controller's own peer record.
	ctrlPeer, err := s.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerController, domain.ControllerPeerID)
	if err != nil {
		return "", fmt.Errorf("get controller peer: %w", err)
	}

	// Decrypt controller's private key.
	privKey, err := s.secrets.Decrypt(ctrlPeer.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decrypt controller private key: %w", err)
	}

	// List all non-controller peers.
	allPeers, _, err := s.peers.ListWireGuardPeers(ctx, storage.ListParams{Limit: 10000})
	if err != nil {
		return "", fmt.Errorf("list peers: %w", err)
	}

	var peerConfigs []WireGuardPeerConfig
	for _, p := range allPeers {
		if p.OwnerType == domain.PeerOwnerController {
			continue // Skip controller's own peer.
		}

		pc := WireGuardPeerConfig{
			PublicKey:  p.PublicKey,
			AllowedIPs: p.AllowedIPs,
			Endpoint:   p.Endpoint,
		}

		// Decrypt preshared key if present.
		if p.PresharedKey != "" {
			psk, err := s.secrets.Decrypt(p.PresharedKey)
			if err != nil {
				return "", fmt.Errorf("decrypt preshared key for peer %s: %w", p.ID, err)
			}
			pc.PresharedKey = psk
		}

		peerConfigs = append(peerConfigs, pc)
	}

	cfg := &WireGuardConfig{
		PrivateKey: privKey,
		Address:    s.wgConfig.Address,
		ListenPort: s.wgConfig.ListenPort,
		Peers:      peerConfigs,
	}

	return cfg.Render(), nil
}

func (s *DefaultService) GenerateNodeConfig(ctx context.Context, nodeID domain.ID) (string, error) {
	// Get node.
	node, err := s.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("get node: %w", err)
	}

	// Get node's WireGuard peer.
	nodePeer, err := s.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, nodeID)
	if err != nil {
		return "", fmt.Errorf("get node peer: %w", err)
	}

	// Decrypt node's private key.
	nodePrivKey, err := s.secrets.Decrypt(nodePeer.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decrypt node private key: %w", err)
	}

	// Get controller's WireGuard peer (hub).
	ctrlPeer, err := s.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerController, domain.ControllerPeerID)
	if err != nil {
		return "", fmt.Errorf("get controller peer: %w", err)
	}

	// Build hub peer config.
	hubPeer := WireGuardPeerConfig{
		PublicKey:           ctrlPeer.PublicKey,
		AllowedIPs:          []string{s.wgConfig.Subnet},
		Endpoint:            ctrlPeer.Endpoint,
		PersistentKeepalive: 25,
	}

	// Decrypt preshared key if present on the node's peer.
	if nodePeer.PresharedKey != "" {
		psk, err := s.secrets.Decrypt(nodePeer.PresharedKey)
		if err != nil {
			return "", fmt.Errorf("decrypt node preshared key: %w", err)
		}
		hubPeer.PresharedKey = psk
	}

	// Node address is its WireGuard IP as /32.
	nodeAddr := node.WireGuardIP + "/32"
	if node.WireGuardIP == "" {
		return "", fmt.Errorf("node %s has no WireGuard IP assigned", nodeID)
	}

	cfg := &WireGuardConfig{
		PrivateKey: nodePrivKey,
		Address:    nodeAddr,
		Peers:      []WireGuardPeerConfig{hubPeer},
	}

	return cfg.Render(), nil
}

// --- Node Networking State ---

func (s *DefaultService) GetNodeNetworkingState(ctx context.Context, nodeID domain.ID) (*NodeNetworkingState, error) {
	node, err := s.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	state := &NodeNetworkingState{
		NodeID:      nodeID,
		WireGuardIP: node.WireGuardIP,
	}

	// Get WG peer (may not exist yet).
	peer, err := s.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, nodeID)
	if err == nil {
		state.WireGuardPeer = peer
	}

	// Get BGP sessions.
	sessions, _, err := s.bgp.ListBGPSessions(ctx, nodeID, storage.ListParams{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("list bgp sessions: %w", err)
	}
	state.BGPSessions = sessions

	return state, nil
}
