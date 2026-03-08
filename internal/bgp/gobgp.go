package bgp

import (
	"context"
	"fmt"
	"net"
	"sync"

	apipb "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/server"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// Ensure GoBGPService implements Service at compile time.
var _ Service = (*GoBGPService)(nil)

// GoBGPService implements the BGP Service interface using GoBGP as an
// embedded library. It uses the GoBGP server.BgpServer directly via
// the Go API (not gRPC), running BGP in-process.
type GoBGPService struct {
	mu        sync.Mutex
	server    *server.BgpServer
	running   bool
	routerID  string
	localASN  uint32
	peers     map[string]bool   // peerAddress → exists
	prefixes  map[string][]byte // prefix → path UUID
}

// NewGoBGPService creates a new GoBGP-backed BGP service.
func NewGoBGPService() *GoBGPService {
	return &GoBGPService{
		peers:    make(map[string]bool),
		prefixes: make(map[string][]byte),
	}
}

func (s *GoBGPService) Start(ctx context.Context, routerID string, localASN uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("bgp service already running")
	}

	s.server = server.NewBgpServer()
	go s.server.Serve()

	// Configure the BGP global settings.
	if err := s.server.StartBgp(ctx, &apipb.StartBgpRequest{
		Global: &apipb.Global{
			Asn:        localASN,
			RouterId:   routerID,
			ListenPort: -1, // Don't listen for incoming BGP connections; we initiate.
		},
	}); err != nil {
		return fmt.Errorf("start bgp: %w", err)
	}

	s.running = true
	s.routerID = routerID
	s.localASN = localASN
	return nil
}

func (s *GoBGPService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	if err := s.server.StopBgp(ctx, &apipb.StopBgpRequest{}); err != nil {
		return fmt.Errorf("stop bgp: %w", err)
	}

	s.running = false
	s.peers = make(map[string]bool)
	s.prefixes = make(map[string][]byte)
	return nil
}

func (s *GoBGPService) Reconcile(ctx context.Context, desired []*domain.BGPSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	// Build desired state map.
	desiredPeers := make(map[string]*domain.BGPSession)
	for _, sess := range desired {
		desiredPeers[sess.PeerAddress] = sess
	}

	// Remove peers not in desired state.
	for addr := range s.peers {
		if _, ok := desiredPeers[addr]; !ok {
			if err := s.server.DeletePeer(ctx, &apipb.DeletePeerRequest{
				Address: addr,
			}); err != nil {
				return fmt.Errorf("delete peer %s: %w", addr, err)
			}
			delete(s.peers, addr)
		}
	}

	// Add new peers.
	for addr, sess := range desiredPeers {
		if _, exists := s.peers[addr]; exists {
			continue // Already exists, skip for now (update could be added later).
		}

		peerConf := &apipb.Peer{
			Conf: &apipb.PeerConf{
				NeighborAddress: sess.PeerAddress,
				PeerAsn:         sess.PeerASN,
			},
			Timers: &apipb.Timers{
				Config: &apipb.TimersConfig{
					ConnectRetry:           10,
					HoldTime:               90,
					KeepaliveInterval:      30,
				},
			},
			AfiSafis: []*apipb.AfiSafi{
				{
					Config: &apipb.AfiSafiConfig{
						Family: &apipb.Family{
							Afi:  apipb.Family_AFI_IP,
							Safi: apipb.Family_SAFI_UNICAST,
						},
						Enabled: true,
					},
				},
			},
		}

		if err := s.server.AddPeer(ctx, &apipb.AddPeerRequest{
			Peer: peerConf,
		}); err != nil {
			return fmt.Errorf("add peer %s: %w", addr, err)
		}
		s.peers[addr] = true

		// Announce prefixes for this session.
		for _, prefix := range sess.AnnouncedPrefixes {
			if _, announced := s.prefixes[prefix]; !announced {
				if err := s.announcePrefix(ctx, prefix, s.routerID); err != nil {
					return fmt.Errorf("announce prefix %s: %w", prefix, err)
				}
			}
		}
	}

	return nil
}

func (s *GoBGPService) GetStatus(ctx context.Context) ([]SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, nil
	}

	var states []SessionState
	err := s.server.ListPeer(ctx, &apipb.ListPeerRequest{}, func(p *apipb.Peer) {
		state := SessionState{
			PeerAddress: p.Conf.GetNeighborAddress(),
			PeerASN:     p.Conf.GetPeerAsn(),
			LocalASN:    s.localASN,
			PrefixesOut: len(s.prefixes),
		}

		// Map GoBGP session state to our status string.
		if p.State != nil {
			switch p.State.SessionState {
			case apipb.PeerState_ESTABLISHED:
				state.Status = "established"
				state.UptimeSeconds = p.Timers.GetState().GetUptime().GetSeconds()
			case apipb.PeerState_IDLE:
				state.Status = "idle"
			case apipb.PeerState_CONNECT:
				state.Status = "connect"
			case apipb.PeerState_ACTIVE:
				state.Status = "active"
			case apipb.PeerState_OPENSENT:
				state.Status = "opensent"
			case apipb.PeerState_OPENCONFIRM:
				state.Status = "openconfirm"
			default:
				state.Status = "unknown"
			}
		}

		states = append(states, state)
	})
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}

	return states, nil
}

func (s *GoBGPService) AnnouncePrefix(ctx context.Context, prefix string, nextHop string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	return s.announcePrefix(ctx, prefix, nextHop)
}

func (s *GoBGPService) WithdrawPrefix(ctx context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("bgp service not running")
	}

	uuid, ok := s.prefixes[prefix]
	if !ok {
		return nil // Not announced, nothing to withdraw.
	}

	if err := s.server.DeletePath(ctx, &apipb.DeletePathRequest{
		Uuid: uuid,
	}); err != nil {
		return fmt.Errorf("delete path: %w", err)
	}

	delete(s.prefixes, prefix)
	return nil
}

// announcePrefix adds a BGP path for the given prefix (must be called with lock held).
func (s *GoBGPService) announcePrefix(ctx context.Context, prefix string, nextHop string) error {
	ip, ipNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	prefixLen, _ := ipNet.Mask.Size()

	nlri, err := anypb.New(&apipb.IPAddressPrefix{
		PrefixLen: uint32(prefixLen),
		Prefix:    ip.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal nlri: %w", err)
	}

	origin, err := anypb.New(&apipb.OriginAttribute{
		Origin: 0, // IGP
	})
	if err != nil {
		return fmt.Errorf("marshal origin: %w", err)
	}

	nh, err := anypb.New(&apipb.NextHopAttribute{
		NextHop: nextHop,
	})
	if err != nil {
		return fmt.Errorf("marshal next hop: %w", err)
	}

	resp, err := s.server.AddPath(ctx, &apipb.AddPathRequest{
		Path: &apipb.Path{
			Family: &apipb.Family{
				Afi:  apipb.Family_AFI_IP,
				Safi: apipb.Family_SAFI_UNICAST,
			},
			Nlri:   nlri,
			Pattrs: []*anypb.Any{origin, nh},
		},
	})
	if err != nil {
		return fmt.Errorf("add path: %w", err)
	}

	s.prefixes[prefix] = resp.GetUuid()
	return nil
}
