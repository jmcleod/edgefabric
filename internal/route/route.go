package route

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService is the controller-side route management service.
type DefaultService struct {
	routes     storage.RouteStore
	gateways   storage.GatewayStore
	nodeGroups storage.NodeGroupStore
	nodes      storage.NodeStore
}

// NewService creates a new route service.
func NewService(
	routes storage.RouteStore,
	gateways storage.GatewayStore,
	nodeGroups storage.NodeGroupStore,
	nodes storage.NodeStore,
) *DefaultService {
	return &DefaultService{
		routes:     routes,
		gateways:   gateways,
		nodeGroups: nodeGroups,
		nodes:      nodes,
	}
}

// --- Route CRUD ---

func (s *DefaultService) CreateRoute(ctx context.Context, req CreateRouteRequest) (*domain.Route, error) {
	r := &domain.Route{
		ID:              domain.NewID(),
		TenantID:        req.TenantID,
		Name:            req.Name,
		Protocol:        req.Protocol,
		EntryIP:         req.EntryIP,
		EntryPort:       req.EntryPort,
		GatewayID:       req.GatewayID,
		DestinationIP:   req.DestinationIP,
		DestinationPort: req.DestinationPort,
		NodeGroupID:     req.NodeGroupID,
	}

	if err := validateRoute(r); err != nil {
		return nil, fmt.Errorf("invalid route: %w", err)
	}

	// Verify the gateway exists.
	if _, err := s.gateways.GetGateway(ctx, req.GatewayID); err != nil {
		return nil, fmt.Errorf("gateway lookup: %w", err)
	}

	if err := s.routes.CreateRoute(ctx, r); err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}
	return r, nil
}

func (s *DefaultService) GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error) {
	return s.routes.GetRoute(ctx, id)
}

func (s *DefaultService) ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error) {
	return s.routes.ListRoutes(ctx, tenantID, params)
}

func (s *DefaultService) UpdateRoute(ctx context.Context, id domain.ID, req UpdateRouteRequest) (*domain.Route, error) {
	r, err := s.routes.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		r.Name = *req.Name
	}
	if req.Protocol != nil {
		r.Protocol = *req.Protocol
	}
	if req.EntryIP != nil {
		r.EntryIP = *req.EntryIP
	}
	if req.EntryPort != nil {
		r.EntryPort = req.EntryPort
	}
	if req.GatewayID != nil {
		r.GatewayID = *req.GatewayID
	}
	if req.DestinationIP != nil {
		r.DestinationIP = *req.DestinationIP
	}
	if req.DestinationPort != nil {
		r.DestinationPort = req.DestinationPort
	}
	if req.ClearNodeGroup {
		r.NodeGroupID = nil
	} else if req.NodeGroupID != nil {
		r.NodeGroupID = req.NodeGroupID
	}
	if req.Status != nil {
		r.Status = *req.Status
	}

	if err := validateRoute(r); err != nil {
		return nil, fmt.Errorf("invalid route: %w", err)
	}

	// Verify gateway exists if changed.
	if req.GatewayID != nil {
		if _, err := s.gateways.GetGateway(ctx, *req.GatewayID); err != nil {
			return nil, fmt.Errorf("gateway lookup: %w", err)
		}
	}

	if err := s.routes.UpdateRoute(ctx, r); err != nil {
		return nil, fmt.Errorf("update route: %w", err)
	}
	return r, nil
}

func (s *DefaultService) DeleteRoute(ctx context.Context, id domain.ID) error {
	return s.routes.DeleteRoute(ctx, id)
}

// --- Config sync ---

// GetNodeRouteConfig returns the complete route configuration for a node.
// It finds all routes assigned to node groups that the node belongs to,
// and includes the gateway WireGuard IP for each route.
func (s *DefaultService) GetNodeRouteConfig(ctx context.Context, nodeID domain.ID) (*NodeRouteConfig, error) {
	// Verify node exists.
	if _, err := s.nodes.GetNode(ctx, nodeID); err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	// Get all node groups this node belongs to.
	groups, err := s.nodeGroups.ListNodeGroups_ByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list node groups: %w", err)
	}

	// For each group, find routes assigned to it.
	config := &NodeRouteConfig{}
	seenRoutes := make(map[domain.ID]bool)
	gwCache := make(map[domain.ID]string) // gatewayID -> WireGuardIP

	for _, group := range groups {
		routes, _, err := s.routes.ListRoutes(ctx, group.TenantID, storage.ListParams{Limit: 10000})
		if err != nil {
			return nil, fmt.Errorf("list routes for group %s: %w", group.ID, err)
		}

		for _, r := range routes {
			// Only include routes assigned to this group and active.
			if r.NodeGroupID == nil || *r.NodeGroupID != group.ID {
				continue
			}
			if r.Status != domain.RouteStatusActive {
				continue
			}
			if seenRoutes[r.ID] {
				continue
			}
			seenRoutes[r.ID] = true

			// Look up gateway WireGuard IP (cached).
			gwIP, ok := gwCache[r.GatewayID]
			if !ok {
				gw, err := s.gateways.GetGateway(ctx, r.GatewayID)
				if err != nil {
					return nil, fmt.Errorf("get gateway %s: %w", r.GatewayID, err)
				}
				gwIP = gw.WireGuardIP
				gwCache[r.GatewayID] = gwIP
			}

			config.Routes = append(config.Routes, RouteWithGateway{
				Route:       r,
				GatewayWGIP: gwIP,
			})
		}
	}

	return config, nil
}

// GetGatewayRouteConfig returns the route configuration for a gateway.
// It returns all active routes that reference this gateway.
func (s *DefaultService) GetGatewayRouteConfig(ctx context.Context, gatewayID domain.ID) (*GatewayRouteConfig, error) {
	// Verify gateway exists.
	if _, err := s.gateways.GetGateway(ctx, gatewayID); err != nil {
		return nil, fmt.Errorf("get gateway: %w", err)
	}

	routes, err := s.routes.ListRoutesByGateway(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list routes by gateway: %w", err)
	}

	return &GatewayRouteConfig{Routes: routes}, nil
}
