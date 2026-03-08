// Package gatewayrt implements the gateway-side route forwarding runtime.
// It receives route configuration from the controller via the route
// service's GatewayRouteConfig and manages per-route TCP/UDP listeners
// bound to the gateway's WireGuard overlay IP that forward traffic
// to private network destinations.
package gatewayrt

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/route"
)

// Service is the gateway-side route forwarding runtime interface.
// Implementations listen on the WireGuard overlay IP for incoming
// traffic from nodes and forward it to private destinations.
type Service interface {
	// Start initializes the gateway route forwarding service.
	Start(ctx context.Context) error

	// Stop shuts down all route listeners gracefully.
	Stop(ctx context.Context) error

	// Reconcile loads the desired route configuration from the controller
	// and starts/stops/restarts per-route listeners to match.
	Reconcile(ctx context.Context, config *route.GatewayRouteConfig) error

	// GetStatus returns the current runtime state of the gateway route service.
	GetStatus(ctx context.Context) (*ServerStatus, error)
}

// ServerStatus represents the runtime status of the gateway route forwarding service.
type ServerStatus struct {
	Running         bool   `json:"running"`
	ActiveRoutes    int    `json:"active_routes"`
	TCPListeners    int    `json:"tcp_listeners"`
	UDPListeners    int    `json:"udp_listeners"`
	ConnectionsOpen uint64 `json:"connections_open"`
	BytesForwarded  uint64 `json:"bytes_forwarded"`
}
