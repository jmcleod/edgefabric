package networking

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the networking orchestration interface on the controller.
// It manages WireGuard configuration, BGP sessions, IP allocations,
// and aggregated networking state for nodes.
type Service interface {
	// WireGuard config generation.
	GenerateHubConfig(ctx context.Context) (string, error)
	GenerateNodeConfig(ctx context.Context, nodeID domain.ID) (string, error)

	// BGP session management (controller-side desired state).
	CreateBGPSession(ctx context.Context, req CreateBGPSessionRequest) (*domain.BGPSession, error)
	GetBGPSession(ctx context.Context, id domain.ID) (*domain.BGPSession, error)
	ListBGPSessions(ctx context.Context, nodeID domain.ID, params storage.ListParams) ([]*domain.BGPSession, int, error)
	UpdateBGPSession(ctx context.Context, id domain.ID, req UpdateBGPSessionRequest) (*domain.BGPSession, error)
	DeleteBGPSession(ctx context.Context, id domain.ID) error

	// IP allocation management.
	CreateIPAllocation(ctx context.Context, req CreateIPAllocationRequest) (*domain.IPAllocation, error)
	GetIPAllocation(ctx context.Context, id domain.ID) (*domain.IPAllocation, error)
	ListIPAllocations(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.IPAllocation, int, error)
	UpdateIPAllocation(ctx context.Context, id domain.ID, req UpdateIPAllocationRequest) (*domain.IPAllocation, error)
	DeleteIPAllocation(ctx context.Context, id domain.ID) error

	// WireGuard peer listing (read-only view for API).
	ListWireGuardPeers(ctx context.Context, params storage.ListParams) ([]*domain.WireGuardPeer, int, error)

	// Node networking state (aggregated view).
	GetNodeNetworkingState(ctx context.Context, nodeID domain.ID) (*NodeNetworkingState, error)
}

// CreateBGPSessionRequest is the input for creating a BGP session.
type CreateBGPSessionRequest struct {
	NodeID            domain.ID `json:"node_id"`
	PeerASN           uint32    `json:"peer_asn"`
	PeerAddress       string    `json:"peer_address"`
	LocalASN          uint32    `json:"local_asn"`
	AnnouncedPrefixes []string  `json:"announced_prefixes,omitempty"`
	ImportPolicy      string    `json:"import_policy,omitempty"`
	ExportPolicy      string    `json:"export_policy,omitempty"`
}

// UpdateBGPSessionRequest is the input for updating a BGP session.
type UpdateBGPSessionRequest struct {
	PeerASN           *uint32                  `json:"peer_asn,omitempty"`
	PeerAddress       *string                  `json:"peer_address,omitempty"`
	LocalASN          *uint32                  `json:"local_asn,omitempty"`
	Status            *domain.BGPSessionStatus `json:"status,omitempty"`
	AnnouncedPrefixes []string                 `json:"announced_prefixes,omitempty"`
	ImportPolicy      *string                  `json:"import_policy,omitempty"`
	ExportPolicy      *string                  `json:"export_policy,omitempty"`
}

// CreateIPAllocationRequest is the input for creating an IP allocation.
type CreateIPAllocationRequest struct {
	TenantID domain.ID                  `json:"tenant_id"`
	Prefix   string                     `json:"prefix"`
	Type     domain.IPAllocationType    `json:"type"`
	Purpose  domain.IPAllocationPurpose `json:"purpose"`
}

// UpdateIPAllocationRequest is the input for updating an IP allocation.
type UpdateIPAllocationRequest struct {
	Status *domain.IPAllocationStatus `json:"status,omitempty"`
}

// NodeNetworkingState aggregates all networking info for a node.
type NodeNetworkingState struct {
	NodeID        domain.ID              `json:"node_id"`
	WireGuardIP   string                 `json:"wireguard_ip"`
	WireGuardPeer *domain.WireGuardPeer  `json:"wireguard_peer,omitempty"`
	BGPSessions   []*domain.BGPSession   `json:"bgp_sessions"`
}
