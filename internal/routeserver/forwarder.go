package routeserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

// Ensure ForwarderService implements Service at compile time.
var _ Service = (*ForwarderService)(nil)

// routeRuntime tracks the state of a single active route.
type routeRuntime struct {
	route       *domain.Route
	gatewayWGIP string
	tcpListener net.Listener
	udpConn     net.PacketConn
	cancel      context.CancelFunc
}

// ForwarderService is a route forwarding implementation that manages
// per-route TCP/UDP listeners. Each route binds to its own EntryIP:EntryPort
// and forwards traffic through the WireGuard overlay to the gateway.
type ForwarderService struct {
	mu     sync.Mutex
	logger *slog.Logger

	running bool
	routes  map[domain.ID]*routeRuntime

	// Counters.
	connectionsOpen atomic.Uint64
	bytesForwarded  atomic.Uint64
}

// NewForwarderService creates a new route forwarder service.
func NewForwarderService(logger *slog.Logger) *ForwarderService {
	return &ForwarderService{
		logger: logger,
		routes: make(map[domain.ID]*routeRuntime),
	}
}

func (s *ForwarderService) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("route forwarder already running")
	}

	s.running = true
	s.logger.Info("route forwarder started")
	return nil
}

func (s *ForwarderService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("route forwarder not running")
	}

	// Stop all active route listeners.
	for id, rt := range s.routes {
		s.stopRouteLocked(rt)
		delete(s.routes, id)
	}

	s.running = false
	s.logger.Info("route forwarder stopped")
	return nil
}

func (s *ForwarderService) Reconcile(_ context.Context, config *route.NodeRouteConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("route forwarder not running")
	}

	// Build desired set from config.
	desired := make(map[domain.ID]route.RouteWithGateway)
	if config != nil {
		for _, rwg := range config.Routes {
			desired[rwg.Route.ID] = rwg
		}
	}

	// Stop routes that are no longer desired.
	for id, rt := range s.routes {
		if _, ok := desired[id]; !ok {
			s.logger.Info("stopping removed route",
				slog.String("route_id", id.String()),
				slog.String("route_name", rt.route.Name),
			)
			s.stopRouteLocked(rt)
			delete(s.routes, id)
		}
	}

	// Start new routes or restart changed routes.
	for id, rwg := range desired {
		existing, ok := s.routes[id]
		if ok && !s.routeChanged(existing, rwg) {
			continue // Already running with same config.
		}

		// Stop existing if changed.
		if ok {
			s.logger.Info("restarting changed route",
				slog.String("route_id", id.String()),
				slog.String("route_name", rwg.Route.Name),
			)
			s.stopRouteLocked(existing)
			delete(s.routes, id)
		}

		// Start new route.
		rt, err := s.startRoute(rwg)
		if err != nil {
			s.logger.Error("failed to start route",
				slog.String("route_id", id.String()),
				slog.String("route_name", rwg.Route.Name),
				slog.String("error", err.Error()),
			)
			continue
		}
		s.routes[id] = rt
	}

	return nil
}

func (s *ForwarderService) GetStatus(_ context.Context) (*ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tcpListeners, udpListeners int
	for _, rt := range s.routes {
		if rt.tcpListener != nil {
			tcpListeners++
		}
		if rt.udpConn != nil {
			udpListeners++
		}
	}

	return &ServerStatus{
		Running:         s.running,
		ActiveRoutes:    len(s.routes),
		TCPListeners:    tcpListeners,
		UDPListeners:    udpListeners,
		ConnectionsOpen: s.connectionsOpen.Load(),
		BytesForwarded:  s.bytesForwarded.Load(),
	}, nil
}

// routeChanged checks if the route config has changed in a way that requires restart.
func (s *ForwarderService) routeChanged(existing *routeRuntime, desired route.RouteWithGateway) bool {
	r := existing.route
	d := desired.Route

	if r.EntryIP != d.EntryIP {
		return true
	}
	if !intPtrEqual(r.EntryPort, d.EntryPort) {
		return true
	}
	if r.Protocol != d.Protocol {
		return true
	}
	if existing.gatewayWGIP != desired.GatewayWGIP {
		return true
	}
	if r.DestinationIP != d.DestinationIP {
		return true
	}
	if !intPtrEqual(r.DestinationPort, d.DestinationPort) {
		return true
	}
	return false
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// startRoute creates listeners for a single route.
func (s *ForwarderService) startRoute(rwg route.RouteWithGateway) (*routeRuntime, error) {
	r := rwg.Route
	ctx, cancel := context.WithCancel(context.Background())

	rt := &routeRuntime{
		route:       r,
		gatewayWGIP: rwg.GatewayWGIP,
		cancel:      cancel,
	}

	protocol := r.Protocol

	switch protocol {
	case domain.RouteProtocolTCP:
		if err := s.startTCPListener(ctx, rt); err != nil {
			cancel()
			return nil, err
		}
	case domain.RouteProtocolUDP:
		if err := s.startUDPListener(ctx, rt); err != nil {
			cancel()
			return nil, err
		}
	case domain.RouteProtocolAll:
		// Start both TCP and UDP.
		if err := s.startTCPListener(ctx, rt); err != nil {
			cancel()
			return nil, err
		}
		if err := s.startUDPListener(ctx, rt); err != nil {
			// Clean up TCP listener.
			if rt.tcpListener != nil {
				rt.tcpListener.Close()
			}
			cancel()
			return nil, err
		}
	case domain.RouteProtocolICMP:
		// ICMP requires raw sockets — deferred to v2.
		s.logger.Warn("ICMP route forwarding not yet supported, skipping",
			slog.String("route_id", r.ID.String()),
			slog.String("route_name", r.Name),
		)
		cancel()
		return nil, fmt.Errorf("icmp forwarding not supported in v1")
	default:
		cancel()
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	s.logger.Info("route started",
		slog.String("route_id", r.ID.String()),
		slog.String("route_name", r.Name),
		slog.String("protocol", string(protocol)),
		slog.String("entry", fmt.Sprintf("%s:%d", r.EntryIP, safePort(r.EntryPort))),
		slog.String("gateway_wg_ip", rwg.GatewayWGIP),
	)

	return rt, nil
}

// startTCPListener binds a TCP listener on EntryIP:EntryPort and starts
// accepting connections. Each connection is forwarded to the gateway.
func (s *ForwarderService) startTCPListener(ctx context.Context, rt *routeRuntime) error {
	addr := fmt.Sprintf("%s:%d", rt.route.EntryIP, safePort(rt.route.EntryPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen tcp %s: %w", addr, err)
	}
	rt.tcpListener = ln

	go s.tcpAcceptLoop(ctx, ln, rt)
	return nil
}

// tcpAcceptLoop accepts TCP connections and spawns relay goroutines.
func (s *ForwarderService) tcpAcceptLoop(ctx context.Context, ln net.Listener, rt *routeRuntime) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return // Clean shutdown.
			default:
				s.logger.Error("tcp accept error",
					slog.String("route_id", rt.route.ID.String()),
					slog.String("error", err.Error()),
				)
				return
			}
		}

		s.connectionsOpen.Add(1)
		go s.tcpRelay(ctx, conn, rt)
	}
}

// tcpRelay connects to the gateway and bidirectionally copies data.
func (s *ForwarderService) tcpRelay(ctx context.Context, clientConn net.Conn, rt *routeRuntime) {
	defer func() {
		clientConn.Close()
		s.connectionsOpen.Add(^uint64(0)) // Decrement.
	}()

	// Connect to gateway on overlay network.
	// Node sends to GatewayWGIP:EntryPort — the gateway listens there.
	gwAddr := fmt.Sprintf("%s:%d", rt.gatewayWGIP, safePort(rt.route.EntryPort))
	gwConn, err := net.DialTimeout("tcp", gwAddr, 10*time.Second)
	if err != nil {
		s.logger.Error("tcp dial gateway failed",
			slog.String("route_id", rt.route.ID.String()),
			slog.String("gateway_addr", gwAddr),
			slog.String("error", err.Error()),
		)
		return
	}
	defer gwConn.Close()

	// Bidirectional relay.
	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(gwConn, clientConn)
		s.bytesForwarded.Add(uint64(n))
		done <- struct{}{}
	}()
	go func() {
		n, _ := io.Copy(clientConn, gwConn)
		s.bytesForwarded.Add(uint64(n))
		done <- struct{}{}
	}()

	// Wait for either direction to finish, or context to cancel.
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// startUDPListener binds a UDP listener on EntryIP:EntryPort and starts
// forwarding packets to the gateway, maintaining a session map for return traffic.
func (s *ForwarderService) startUDPListener(ctx context.Context, rt *routeRuntime) error {
	addr := fmt.Sprintf("%s:%d", rt.route.EntryIP, safePort(rt.route.EntryPort))
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", addr, err)
	}
	rt.udpConn = conn

	go s.udpReadLoop(ctx, conn, rt)
	return nil
}

// udpSession tracks a single client→gateway UDP session.
type udpSession struct {
	upstreamConn net.Conn
	lastActivity atomic.Int64 // Unix timestamp.
}

// udpReadLoop reads packets from clients and forwards them to the gateway.
// It maintains a session map for return traffic routing.
func (s *ForwarderService) udpReadLoop(ctx context.Context, conn net.PacketConn, rt *routeRuntime) {
	var sessions sync.Map // clientAddr string → *udpSession

	buf := make([]byte, 65535)
	gwAddr := fmt.Sprintf("%s:%d", rt.gatewayWGIP, safePort(rt.route.EntryPort))

	for {
		select {
		case <-ctx.Done():
			// Clean up all sessions.
			sessions.Range(func(key, value any) bool {
				if sess, ok := value.(*udpSession); ok {
					sess.upstreamConn.Close()
				}
				return true
			})
			return
		default:
		}

		// Set read deadline so we can check context periodically.
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, clientAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Read timeout, check context.
			}
			select {
			case <-ctx.Done():
				return
			default:
				s.logger.Error("udp read error",
					slog.String("route_id", rt.route.ID.String()),
					slog.String("error", err.Error()),
				)
				return
			}
		}

		clientKey := clientAddr.String()
		s.bytesForwarded.Add(uint64(n))

		// Get or create session.
		sessIface, loaded := sessions.Load(clientKey)
		if !loaded {
			// New session: connect to gateway.
			gwConn, err := net.DialTimeout("udp", gwAddr, 10*time.Second)
			if err != nil {
				s.logger.Error("udp dial gateway failed",
					slog.String("route_id", rt.route.ID.String()),
					slog.String("gateway_addr", gwAddr),
					slog.String("error", err.Error()),
				)
				continue
			}

			sess := &udpSession{upstreamConn: gwConn}
			sess.lastActivity.Store(time.Now().Unix())
			sessions.Store(clientKey, sess)
			sessIface = sess

			// Start reverse goroutine: gateway → client.
			go s.udpReverseRelay(ctx, conn, gwConn, clientAddr, &sessions, clientKey, rt)
		}

		sess := sessIface.(*udpSession)
		sess.lastActivity.Store(time.Now().Unix())

		// Forward packet to gateway.
		if _, err := sess.upstreamConn.Write(buf[:n]); err != nil {
			s.logger.Debug("udp forward error",
				slog.String("route_id", rt.route.ID.String()),
				slog.String("error", err.Error()),
			)
			sess.upstreamConn.Close()
			sessions.Delete(clientKey)
		}
	}
}

// udpReverseRelay reads packets from the gateway and sends them back to the client.
// Sessions have a 30-second idle timeout.
func (s *ForwarderService) udpReverseRelay(
	ctx context.Context,
	downstreamConn net.PacketConn,
	upstreamConn net.Conn,
	clientAddr net.Addr,
	sessions *sync.Map,
	clientKey string,
	rt *routeRuntime,
) {
	defer func() {
		upstreamConn.Close()
		sessions.Delete(clientKey)
	}()

	buf := make([]byte, 65535)
	idleTimeout := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		upstreamConn.SetReadDeadline(time.Now().Add(idleTimeout))
		n, err := upstreamConn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Check idle timeout.
				sessIface, ok := sessions.Load(clientKey)
				if !ok {
					return
				}
				sess := sessIface.(*udpSession)
				if time.Since(time.Unix(sess.lastActivity.Load(), 0)) > idleTimeout {
					return // Session expired.
				}
				continue
			}
			return // Connection closed or error.
		}

		s.bytesForwarded.Add(uint64(n))

		if _, err := downstreamConn.WriteTo(buf[:n], clientAddr); err != nil {
			s.logger.Debug("udp reverse relay write error",
				slog.String("route_id", rt.route.ID.String()),
				slog.String("error", err.Error()),
			)
			return
		}
	}
}

// stopRouteLocked stops all listeners for a route. Must be called with s.mu held.
func (s *ForwarderService) stopRouteLocked(rt *routeRuntime) {
	if rt.cancel != nil {
		rt.cancel()
	}
	if rt.tcpListener != nil {
		rt.tcpListener.Close()
	}
	if rt.udpConn != nil {
		rt.udpConn.Close()
	}
}

// safePort returns the port value or 0 if nil.
func safePort(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
