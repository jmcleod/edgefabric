// Package fleet manages node and gateway inventory and health.
package fleet

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the fleet management interface.
type Service interface {
	// Nodes
	CreateNode(ctx context.Context, req CreateNodeRequest) (*domain.Node, error)
	GetNode(ctx context.Context, id domain.ID) (*domain.Node, error)
	ListNodes(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Node, int, error)
	UpdateNode(ctx context.Context, id domain.ID, req UpdateNodeRequest) (*domain.Node, error)
	DeleteNode(ctx context.Context, id domain.ID) error
	RecordNodeHeartbeat(ctx context.Context, id domain.ID) error

	// Gateways
	CreateGateway(ctx context.Context, req CreateGatewayRequest) (*domain.Gateway, error)
	GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error)
	ListGateways(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Gateway, int, error)
	UpdateGateway(ctx context.Context, id domain.ID, req UpdateGatewayRequest) (*domain.Gateway, error)
	DeleteGateway(ctx context.Context, id domain.ID) error
	RecordGatewayHeartbeat(ctx context.Context, id domain.ID) error

	// Node Groups
	CreateNodeGroup(ctx context.Context, req CreateNodeGroupRequest) (*domain.NodeGroup, error)
	GetNodeGroup(ctx context.Context, id domain.ID) (*domain.NodeGroup, error)
	ListNodeGroups(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.NodeGroup, int, error)
	DeleteNodeGroup(ctx context.Context, id domain.ID) error
	AddNodeToGroup(ctx context.Context, groupID, nodeID domain.ID) error
	RemoveNodeFromGroup(ctx context.Context, groupID, nodeID domain.ID) error

	// SSH Keys (global resource — SuperUser only for mutations)
	CreateSSHKey(ctx context.Context, k *domain.SSHKey) error
	GetSSHKey(ctx context.Context, id domain.ID) (*domain.SSHKey, error)
	ListSSHKeys(ctx context.Context, params storage.ListParams) ([]*domain.SSHKey, int, error)
	DeleteSSHKey(ctx context.Context, id domain.ID) error
}

// CreateNodeRequest holds the input for creating a node.
type CreateNodeRequest struct {
	TenantID *domain.ID `json:"tenant_id,omitempty"`
	Name     string     `json:"name"`
	Hostname string     `json:"hostname"`
	PublicIP string     `json:"public_ip"`
	Region   string     `json:"region,omitempty"`
	Provider string     `json:"provider,omitempty"`
	SSHPort  int        `json:"ssh_port"`
	SSHUser  string     `json:"ssh_user"`
}

// UpdateNodeRequest holds the input for updating a node.
type UpdateNodeRequest struct {
	Name     *string            `json:"name,omitempty"`
	TenantID *domain.ID         `json:"tenant_id,omitempty"`
	Status   *domain.NodeStatus `json:"status,omitempty"`
	Region   *string            `json:"region,omitempty"`
}

// CreateGatewayRequest holds the input for creating a gateway.
type CreateGatewayRequest struct {
	TenantID domain.ID `json:"tenant_id"`
	Name     string    `json:"name"`
	PublicIP string    `json:"public_ip,omitempty"`
}

// UpdateGatewayRequest holds the input for updating a gateway.
type UpdateGatewayRequest struct {
	Name   *string              `json:"name,omitempty"`
	Status *domain.GatewayStatus `json:"status,omitempty"`
}

// CreateNodeGroupRequest holds the input for creating a node group.
type CreateNodeGroupRequest struct {
	TenantID    domain.ID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
}
