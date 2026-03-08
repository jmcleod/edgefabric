package dnsserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmcleod/edgefabric/internal/dns"
)

// Ensure NoopService implements Service at compile time.
var _ Service = (*NoopService)(nil)

// NoopService is a DNS server implementation that accepts all operations
// but never actually listens for DNS queries. It tracks state in memory
// for testing and local/demo mode.
type NoopService struct {
	mu          sync.Mutex
	running     bool
	listenAddr  string
	zoneSerials map[string]uint32 // zone name → serial
	zoneCount   int
}

// NewNoopService creates a new noop DNS service.
func NewNoopService() *NoopService {
	return &NoopService{
		zoneSerials: make(map[string]uint32),
	}
}

func (s *NoopService) Start(_ context.Context, listenAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("dns server already running")
	}

	s.running = true
	s.listenAddr = listenAddr
	return nil
}

func (s *NoopService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("dns server not running")
	}

	s.running = false
	s.zoneSerials = make(map[string]uint32)
	s.zoneCount = 0
	return nil
}

func (s *NoopService) Reconcile(_ context.Context, config *dns.NodeDNSConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("dns server not running")
	}

	// Update zone data from config.
	s.zoneSerials = make(map[string]uint32)
	s.zoneCount = 0

	if config != nil {
		for _, zwr := range config.Zones {
			s.zoneSerials[zwr.Zone.Name] = zwr.Zone.Serial
			s.zoneCount++
		}
	}

	return nil
}

func (s *NoopService) GetStatus(_ context.Context) (*ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	serials := make(map[string]uint32, len(s.zoneSerials))
	for k, v := range s.zoneSerials {
		serials[k] = v
	}

	return &ServerStatus{
		Listening:    s.running,
		ListenAddr:   s.listenAddr,
		ZoneCount:    s.zoneCount,
		ZoneSerials:  serials,
		QueriesTotal: 0, // Noop never serves queries.
	}, nil
}

// ZoneSerials returns the current zone serial map (for testing).
func (s *NoopService) ZoneSerials() map[string]uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]uint32, len(s.zoneSerials))
	for k, v := range s.zoneSerials {
		result[k] = v
	}
	return result
}

// ZoneCount returns the number of tracked zones (for testing).
func (s *NoopService) ZoneCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.zoneCount
}
