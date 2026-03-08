package cdnserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// Ensure NoopService implements Service at compile time.
var _ Service = (*NoopService)(nil)

// NoopService is a CDN server implementation that accepts all operations
// but never actually listens for HTTP traffic. It tracks state in memory
// for testing and local/demo mode.
type NoopService struct {
	mu         sync.Mutex
	running    bool
	listenAddr string
	siteNames  map[domain.ID]string // siteID → site name
	siteCount  int
}

// NewNoopService creates a new noop CDN service.
func NewNoopService() *NoopService {
	return &NoopService{
		siteNames: make(map[domain.ID]string),
	}
}

func (s *NoopService) Start(_ context.Context, listenAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("cdn server already running")
	}

	s.running = true
	s.listenAddr = listenAddr
	return nil
}

func (s *NoopService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("cdn server not running")
	}

	s.running = false
	s.siteNames = make(map[domain.ID]string)
	s.siteCount = 0
	return nil
}

func (s *NoopService) Reconcile(_ context.Context, config *cdn.NodeCDNConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("cdn server not running")
	}

	// Update site data from config.
	s.siteNames = make(map[domain.ID]string)
	s.siteCount = 0

	if config != nil {
		for _, swo := range config.Sites {
			s.siteNames[swo.Site.ID] = swo.Site.Name
			s.siteCount++
		}
	}

	return nil
}

func (s *NoopService) PurgeCache(_ context.Context, siteID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("cdn server not running")
	}

	// Noop: just verify the site is known.
	if _, ok := s.siteNames[siteID]; !ok {
		return fmt.Errorf("site %s not found", siteID)
	}

	return nil
}

func (s *NoopService) GetStatus(_ context.Context) (*ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &ServerStatus{
		Listening:     s.running,
		ListenAddr:    s.listenAddr,
		SiteCount:     s.siteCount,
		CacheHits:     0,
		CacheMisses:   0,
		CacheEntries:  0,
		RequestsTotal: 0, // Noop never serves requests.
	}, nil
}

// SiteNames returns the current site name map (for testing).
func (s *NoopService) SiteNames() map[domain.ID]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[domain.ID]string, len(s.siteNames))
	for k, v := range s.siteNames {
		result[k] = v
	}
	return result
}

// SiteCount returns the number of tracked sites (for testing).
func (s *NoopService) SiteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.siteCount
}
