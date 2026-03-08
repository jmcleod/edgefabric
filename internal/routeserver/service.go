// Package routeserver implements the node-side route forwarding runtime.
// It receives route configuration from the controller via the route
// service's NodeRouteConfig and manages per-route TCP/UDP listeners
// that forward traffic through the WireGuard overlay to gateways.
package routeserver

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/route"
)

// Service is the node-side route forwarding runtime interface.
// Implementations manage local TCP/UDP listeners for each route and
// forward traffic to the corresponding gateway over the overlay network.
type Service interface {
	// Start initializes the route forwarding service.
	// Unlike CDN/DNS, routes don't have a single listenAddr — each route
	// binds to its own EntryIP:EntryPort. Start just marks the service ready.
	Start(ctx context.Context) error

	// Stop shuts down all route listeners gracefully.
	Stop(ctx context.Context) error

	// Reconcile loads the desired route configuration from the controller
	// and starts/stops/restarts per-route listeners to match.
	Reconcile(ctx context.Context, config *route.NodeRouteConfig) error

	// GetStatus returns the current runtime state of the route service.
	GetStatus(ctx context.Context) (*ServerStatus, error)
}

// ServerStatus represents the runtime status of the route forwarding service.
type ServerStatus struct {
	Running        bool   `json:"running"`
	ActiveRoutes   int    `json:"active_routes"`
	TCPListeners   int    `json:"tcp_listeners"`
	UDPListeners   int    `json:"udp_listeners"`
	ConnectionsOpen uint64 `json:"connections_open"`
	BytesForwarded uint64 `json:"bytes_forwarded"`
}
