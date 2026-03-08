package bgp

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// Ensure NoopService implements Service at compile time.
var _ Service = (*NoopService)(nil)

// NoopService is a BGP service implementation that accepts all operations
// but never actually peers with anything. It tracks state in memory for
// testing and local/demo mode.
type NoopService struct {
	mu        sync.Mutex
	running   bool
	routerID  string
	localASN  uint32
	sessions  map[string]*noopSession // keyed by "peerASN:peerAddress"
	prefixes  map[string]string       // prefix → nextHop
}

type noopSession struct {
	sessionID   domain.ID
	peerASN     uint32
	peerAddress string
	localASN    uint32
}

// NewNoopService creates a new noop BGP service.
func NewNoopService() *NoopService {
	return &NoopService{
		sessions: make(map[string]*noopSession),
		prefixes: make(map[string]string),
	}
}

func (s *NoopService) Start(_ context.Context, routerID string, localASN uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("bgp service already running")
	}

	s.running = true
	s.routerID = routerID
	s.localASN = localASN
	return nil
}

func (s *NoopService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	s.running = false
	s.sessions = make(map[string]*noopSession)
	s.prefixes = make(map[string]string)
	return nil
}

func (s *NoopService) Reconcile(_ context.Context, desired []*domain.BGPSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	// Build desired state map.
	desiredMap := make(map[string]*domain.BGPSession)
	for _, sess := range desired {
		key := sessionKey(sess.PeerASN, sess.PeerAddress)
		desiredMap[key] = sess
	}

	// Remove sessions not in desired state.
	for key := range s.sessions {
		if _, ok := desiredMap[key]; !ok {
			delete(s.sessions, key)
		}
	}

	// Add/update sessions from desired state.
	for key, sess := range desiredMap {
		s.sessions[key] = &noopSession{
			sessionID:   sess.ID,
			peerASN:     sess.PeerASN,
			peerAddress: sess.PeerAddress,
			localASN:    sess.LocalASN,
		}

		// Reconcile announced prefixes.
		for _, prefix := range sess.AnnouncedPrefixes {
			if _, exists := s.prefixes[prefix]; !exists {
				s.prefixes[prefix] = s.routerID // Use routerID as default next-hop.
			}
		}
	}

	return nil
}

func (s *NoopService) GetStatus(_ context.Context) ([]SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var states []SessionState
	for _, sess := range s.sessions {
		status := "idle"
		if s.running {
			status = "established" // Noop pretends all sessions are established.
		}
		states = append(states, SessionState{
			SessionID:   sess.sessionID,
			PeerASN:     sess.peerASN,
			PeerAddress: sess.peerAddress,
			LocalASN:    sess.localASN,
			Status:      status,
			PrefixesOut: len(s.prefixes),
		})
	}
	return states, nil
}

func (s *NoopService) AnnouncePrefix(_ context.Context, prefix string, nextHop string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	s.prefixes[prefix] = nextHop
	return nil
}

func (s *NoopService) WithdrawPrefix(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	delete(s.prefixes, prefix)
	return nil
}

// sessionKey creates a unique key for a BGP session.
func sessionKey(peerASN uint32, peerAddress string) string {
	return fmt.Sprintf("%d:%s", peerASN, peerAddress)
}

// AnnouncedPrefixes returns the current set of announced prefixes (for testing).
func (s *NoopService) AnnouncedPrefixes() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]string, len(s.prefixes))
	for k, v := range s.prefixes {
		result[k] = v
	}
	return result
}

// SessionCount returns the number of tracked sessions (for testing).
func (s *NoopService) SessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}
