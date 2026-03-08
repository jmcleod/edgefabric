package routeserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

// Ensure NoopService implements Service at compile time.
var _ Service = (*NoopService)(nil)

// NoopService is a route forwarding implementation that accepts all operations
// but never actually listens for TCP/UDP traffic. It tracks state in memory
// for testing and local/demo mode.
type NoopService struct {
	mu         sync.Mutex
	running    bool
	routeNames map[domain.ID]string // routeID → route name
	routeCount int
}

// NewNoopService creates a new noop route service.
func NewNoopService() *NoopService {
	return &NoopService{
		routeNames: make(map[domain.ID]string),
	}
}

func (s *NoopService) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("route service already running")
	}

	s.running = true
	return nil
}

func (s *NoopService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("route service not running")
	}

	s.running = false
	s.routeNames = make(map[domain.ID]string)
	s.routeCount = 0
	return nil
}

func (s *NoopService) Reconcile(_ context.Context, config *route.NodeRouteConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("route service not running")
	}

	// Update route data from config.
	s.routeNames = make(map[domain.ID]string)
	s.routeCount = 0

	if config != nil {
		for _, rwg := range config.Routes {
			s.routeNames[rwg.Route.ID] = rwg.Route.Name
			s.routeCount++
		}
	}

	return nil
}

func (s *NoopService) GetStatus(_ context.Context) (*ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &ServerStatus{
		Running:         s.running,
		ActiveRoutes:    s.routeCount,
		TCPListeners:    0,
		UDPListeners:    0,
		ConnectionsOpen: 0,
		BytesForwarded:  0,
	}, nil
}

// RouteNames returns the current route name map (for testing).
func (s *NoopService) RouteNames() map[domain.ID]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[domain.ID]string, len(s.routeNames))
	for k, v := range s.routeNames {
		result[k] = v
	}
	return result
}

// RouteCount returns the number of tracked routes (for testing).
func (s *NoopService) RouteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.routeCount
}
