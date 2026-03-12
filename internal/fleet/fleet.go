package fleet

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService implements the fleet Service interface.
type DefaultService struct {
	nodes    storage.NodeStore
	groups   storage.NodeGroupStore
	gateways storage.GatewayStore
	sshKeys  storage.SSHKeyStore
	eventBus *events.Bus // nil-safe; events are published only when set.
}

// NewService creates a new fleet management service.
func NewService(nodes storage.NodeStore, groups storage.NodeGroupStore, gateways storage.GatewayStore, sshKeys storage.SSHKeyStore, opts ...Option) Service {
	s := &DefaultService{
		nodes:    nodes,
		groups:   groups,
		gateways: gateways,
		sshKeys:  sshKeys,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures the fleet service.
type Option func(*DefaultService)

// WithEventBus enables event publishing on status transitions.
func WithEventBus(bus *events.Bus) Option {
	return func(s *DefaultService) {
		s.eventBus = bus
	}
}

// --- Nodes ---

func (s *DefaultService) CreateNode(ctx context.Context, req CreateNodeRequest) (*domain.Node, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Hostname == "" {
		return nil, fmt.Errorf("hostname is required")
	}
	if req.PublicIP == "" {
		return nil, fmt.Errorf("public_ip is required")
	}
	if ip := net.ParseIP(req.PublicIP); ip == nil {
		return nil, fmt.Errorf("invalid public_ip: %q", req.PublicIP)
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}

	n := &domain.Node{
		ID:       domain.NewID(),
		TenantID: req.TenantID,
		Name:     req.Name,
		Hostname: req.Hostname,
		PublicIP:  req.PublicIP,
		Region:   req.Region,
		Provider: req.Provider,
		SSHPort:  req.SSHPort,
		SSHUser:  req.SSHUser,
		Status:   domain.NodeStatusPending,
	}

	if err := s.nodes.CreateNode(ctx, n); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return n, nil
}

func (s *DefaultService) GetNode(ctx context.Context, id domain.ID) (*domain.Node, error) {
	return s.nodes.GetNode(ctx, id)
}

func (s *DefaultService) ListNodes(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Node, int, error) {
	return s.nodes.ListNodes(ctx, tenantID, params)
}

func (s *DefaultService) UpdateNode(ctx context.Context, id domain.ID, req UpdateNodeRequest) (*domain.Node, error) {
	n, err := s.nodes.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		n.Name = *req.Name
	}
	if req.TenantID != nil {
		n.TenantID = req.TenantID
	}
	if req.Status != nil {
		n.Status = *req.Status
	}
	if req.Region != nil {
		n.Region = *req.Region
	}

	if err := s.nodes.UpdateNode(ctx, n); err != nil {
		return nil, fmt.Errorf("update node: %w", err)
	}
	return n, nil
}

func (s *DefaultService) DeleteNode(ctx context.Context, id domain.ID) error {
	return s.nodes.DeleteNode(ctx, id)
}

func (s *DefaultService) RecordNodeHeartbeat(ctx context.Context, id domain.ID) error {
	// Read previous status to detect transitions.
	var previousStatus domain.NodeStatus
	if s.eventBus != nil {
		node, err := s.nodes.GetNode(ctx, id)
		if err == nil {
			previousStatus = node.Status
		}
	}

	if err := s.nodes.UpdateNodeHeartbeat(ctx, id); err != nil {
		return err
	}

	// Publish event if the node just came online.
	if s.eventBus != nil && previousStatus != domain.NodeStatusOnline {
		s.eventBus.Publish(ctx, events.Event{
			Type:      events.NodeStatusChanged,
			Timestamp: time.Now().UTC(),
			Severity:  events.SeverityInfo,
			Resource:  "node/" + id.String(),
			Details: map[string]string{
				"previous_status": string(previousStatus),
				"new_status":      string(domain.NodeStatusOnline),
			},
		})
	}

	return nil
}

// --- Gateways ---

func (s *DefaultService) CreateGateway(ctx context.Context, req CreateGatewayRequest) (*domain.Gateway, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	g := &domain.Gateway{
		ID:        domain.NewID(),
		TenantID:  req.TenantID,
		Name:      req.Name,
		PublicIP:   req.PublicIP,
		Status:    domain.GatewayStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.gateways.CreateGateway(ctx, g); err != nil {
		return nil, fmt.Errorf("create gateway: %w", err)
	}
	return g, nil
}

func (s *DefaultService) GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error) {
	return s.gateways.GetGateway(ctx, id)
}

func (s *DefaultService) ListGateways(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Gateway, int, error) {
	return s.gateways.ListGateways(ctx, tenantID, params)
}

func (s *DefaultService) UpdateGateway(ctx context.Context, id domain.ID, req UpdateGatewayRequest) (*domain.Gateway, error) {
	g, err := s.gateways.GetGateway(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		g.Name = *req.Name
	}
	if req.Status != nil {
		g.Status = *req.Status
	}

	if err := s.gateways.UpdateGateway(ctx, g); err != nil {
		return nil, fmt.Errorf("update gateway: %w", err)
	}
	return g, nil
}

func (s *DefaultService) DeleteGateway(ctx context.Context, id domain.ID) error {
	return s.gateways.DeleteGateway(ctx, id)
}

func (s *DefaultService) RecordGatewayHeartbeat(ctx context.Context, id domain.ID) error {
	// Read previous status to detect transitions.
	var previousStatus domain.GatewayStatus
	if s.eventBus != nil {
		gw, err := s.gateways.GetGateway(ctx, id)
		if err == nil {
			previousStatus = gw.Status
		}
	}

	if err := s.gateways.UpdateGatewayHeartbeat(ctx, id); err != nil {
		return err
	}

	// Publish event if the gateway just came online.
	if s.eventBus != nil && previousStatus != domain.GatewayStatusOnline {
		s.eventBus.Publish(ctx, events.Event{
			Type:      events.GatewayStatusChanged,
			Timestamp: time.Now().UTC(),
			Severity:  events.SeverityInfo,
			Resource:  "gateway/" + id.String(),
			Details: map[string]string{
				"previous_status": string(previousStatus),
				"new_status":      string(domain.GatewayStatusOnline),
			},
		})
	}

	return nil
}

// --- Node Groups ---

func (s *DefaultService) CreateNodeGroup(ctx context.Context, req CreateNodeGroupRequest) (*domain.NodeGroup, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	g := &domain.NodeGroup{
		ID:          domain.NewID(),
		TenantID:    req.TenantID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.groups.CreateNodeGroup(ctx, g); err != nil {
		return nil, fmt.Errorf("create node group: %w", err)
	}
	return g, nil
}

func (s *DefaultService) GetNodeGroup(ctx context.Context, id domain.ID) (*domain.NodeGroup, error) {
	return s.groups.GetNodeGroup(ctx, id)
}

func (s *DefaultService) ListNodeGroups(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.NodeGroup, int, error) {
	return s.groups.ListNodeGroups(ctx, tenantID, params)
}

func (s *DefaultService) AddNodeToGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	// Verify group exists.
	if _, err := s.groups.GetNodeGroup(ctx, groupID); err != nil {
		return fmt.Errorf("get group: %w", err)
	}
	// Verify node exists.
	if _, err := s.nodes.GetNode(ctx, nodeID); err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	return s.groups.AddNodeToGroup(ctx, groupID, nodeID)
}

func (s *DefaultService) DeleteNodeGroup(ctx context.Context, id domain.ID) error {
	return s.groups.DeleteNodeGroup(ctx, id)
}

func (s *DefaultService) RemoveNodeFromGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	return s.groups.RemoveNodeFromGroup(ctx, groupID, nodeID)
}

// --- SSH Keys ---

func (s *DefaultService) CreateSSHKey(ctx context.Context, k *domain.SSHKey) error {
	return s.sshKeys.CreateSSHKey(ctx, k)
}

func (s *DefaultService) GetSSHKey(ctx context.Context, id domain.ID) (*domain.SSHKey, error) {
	return s.sshKeys.GetSSHKey(ctx, id)
}

func (s *DefaultService) ListSSHKeys(ctx context.Context, params storage.ListParams) ([]*domain.SSHKey, int, error) {
	return s.sshKeys.ListSSHKeys(ctx, params)
}

func (s *DefaultService) DeleteSSHKey(ctx context.Context, id domain.ID) error {
	return s.sshKeys.DeleteSSHKey(ctx, id)
}
