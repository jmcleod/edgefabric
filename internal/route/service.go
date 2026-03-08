// Package route implements the controller-side route management service.
// It validates route configurations, manages CRUD operations, and provides
// configuration snapshots for nodes and gateways to poll.
package route

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the controller-side route management interface.
type Service interface {
	// Route CRUD.
	CreateRoute(ctx context.Context, req CreateRouteRequest) (*domain.Route, error)
	GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error)
	ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error)
	UpdateRoute(ctx context.Context, id domain.ID, req UpdateRouteRequest) (*domain.Route, error)
	DeleteRoute(ctx context.Context, id domain.ID) error

	// Config sync — returns route config for a specific node.
	GetNodeRouteConfig(ctx context.Context, nodeID domain.ID) (*NodeRouteConfig, error)

	// Config sync — returns route config for a specific gateway.
	GetGatewayRouteConfig(ctx context.Context, gatewayID domain.ID) (*GatewayRouteConfig, error)
}

// CreateRouteRequest is the input for creating a route.
type CreateRouteRequest struct {
	TenantID        domain.ID            `json:"tenant_id"`
	Name            string               `json:"name"`
	Protocol        domain.RouteProtocol `json:"protocol"`
	EntryIP         string               `json:"entry_ip"`
	EntryPort       *int                 `json:"entry_port,omitempty"`
	GatewayID       domain.ID            `json:"gateway_id"`
	DestinationIP   string               `json:"destination_ip"`
	DestinationPort *int                 `json:"destination_port,omitempty"`
	NodeGroupID     *domain.ID           `json:"node_group_id,omitempty"`
}

// UpdateRouteRequest is the input for updating a route.
type UpdateRouteRequest struct {
	Name            *string               `json:"name,omitempty"`
	Protocol        *domain.RouteProtocol `json:"protocol,omitempty"`
	EntryIP         *string               `json:"entry_ip,omitempty"`
	EntryPort       *int                  `json:"entry_port,omitempty"`
	GatewayID       *domain.ID            `json:"gateway_id,omitempty"`
	DestinationIP   *string               `json:"destination_ip,omitempty"`
	DestinationPort *int                  `json:"destination_port,omitempty"`
	NodeGroupID     *domain.ID            `json:"node_group_id,omitempty"`
	Status          *domain.RouteStatus   `json:"status,omitempty"`
	// ClearNodeGroup explicitly removes the node group assignment when true.
	ClearNodeGroup bool `json:"clear_node_group,omitempty"`
}

// NodeRouteConfig is the route configuration snapshot for a node.
// Nodes poll this to reconcile their local route forwarding state.
type NodeRouteConfig struct {
	Routes []RouteWithGateway `json:"routes"`
}

// RouteWithGateway bundles a route with its gateway's WireGuard IP
// so the node knows where to forward traffic over the overlay.
type RouteWithGateway struct {
	Route       *domain.Route `json:"route"`
	GatewayWGIP string        `json:"gateway_wireguard_ip"`
}

// GatewayRouteConfig is the route configuration snapshot for a gateway.
// Gateways poll this to know which routes they should listen for on
// their overlay interface.
type GatewayRouteConfig struct {
	Routes []*domain.Route `json:"routes"`
}
