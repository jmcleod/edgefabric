package gatewayrt

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/route"
)

// Ensure ForwarderService implements Service at compile time.
var _ Service = (*ForwarderService)(nil)

// routeRuntime tracks the state of a single active route on the gateway.
type routeRuntime struct {
	route       *domain.Route
	tcpListener net.Listener
	udpConn     net.PacketConn
	cancel      context.CancelFunc
}

// ForwarderService is the gateway-side route forwarding implementation.
// It binds to the gateway's WireGuard overlay IP for incoming traffic
// from nodes and forwards it to private network destinations.
type ForwarderService struct {
	mu          sync.Mutex
	logger      *slog.Logger
	metrics     *observability.Metrics
	wireGuardIP string // Gateway's overlay IP to bind listeners to.

	running bool
	routes  map[domain.ID]*routeRuntime

	// ICMP proxy — lazy-initialized on first ICMP route.
	icmpProxy *icmpProxy
	icmpOnce  sync.Once

	connectionsOpen atomic.Uint64
	bytesForwarded  atomic.Uint64
}

// NewForwarderService creates a new gateway route forwarder.
// wireGuardIP is the gateway's WireGuard overlay IP address.
func NewForwarderService(wireGuardIP string, logger *slog.Logger, metrics *observability.Metrics) *ForwarderService {
	return &ForwarderService{
		logger:      logger,
		metrics:     metrics,
		wireGuardIP: wireGuardIP,
		routes:      make(map[domain.ID]*routeRuntime),
	}
}

func (s *ForwarderService) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("gateway route forwarder already running")
	}

	s.running = true
	s.logger.Info("gateway route forwarder started",
		slog.String("wireguard_ip", s.wireGuardIP),
	)
	return nil
}

func (s *ForwarderService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("gateway route forwarder not running")
	}

	for id, rt := range s.routes {
		s.stopRouteLocked(rt)
		delete(s.routes, id)
	}

	// Close ICMP proxy if initialized.
	if s.icmpProxy != nil {
		s.icmpProxy.close()
		s.icmpProxy = nil
	}

	s.running = false
	s.logger.Info("gateway route forwarder stopped")
	return nil
}

func (s *ForwarderService) Reconcile(_ context.Context, config *route.GatewayRouteConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("gateway route forwarder not running")
	}

	desired := make(map[domain.ID]*domain.Route)
	if config != nil {
		for _, r := range config.Routes {
			desired[r.ID] = r
		}
	}

	// Stop removed routes.
	for id, rt := range s.routes {
		if _, ok := desired[id]; !ok {
			s.logger.Info("stopping removed gateway route",
				slog.String("route_id", id.String()),
				slog.String("route_name", rt.route.Name),
			)
			s.stopRouteLocked(rt)
			delete(s.routes, id)
		}
	}

	// Start new or changed routes.
	for id, r := range desired {
		existing, ok := s.routes[id]
		if ok && !s.routeChanged(existing.route, r) {
			continue
		}

		if ok {
			s.logger.Info("restarting changed gateway route",
				slog.String("route_id", id.String()),
				slog.String("route_name", r.Name),
			)
			s.stopRouteLocked(existing)
			delete(s.routes, id)
		}

		rt, err := s.startRoute(r)
		if err != nil {
			s.logger.Error("failed to start gateway route",
				slog.String("route_id", id.String()),
				slog.String("route_name", r.Name),
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

	var icmpRoutes int
	if s.icmpProxy != nil {
		icmpRoutes = s.icmpProxy.routeCount()
	}

	return &ServerStatus{
		Running:         s.running,
		ActiveRoutes:    len(s.routes),
		TCPListeners:    tcpListeners,
		UDPListeners:    udpListeners,
		ICMPRoutes:      icmpRoutes,
		ConnectionsOpen: s.connectionsOpen.Load(),
		BytesForwarded:  s.bytesForwarded.Load(),
	}, nil
}

func (s *ForwarderService) routeChanged(existing, desired *domain.Route) bool {
	if !intPtrEqual(existing.EntryPort, desired.EntryPort) {
		return true
	}
	if existing.Protocol != desired.Protocol {
		return true
	}
	if existing.DestinationIP != desired.DestinationIP {
		return true
	}
	if !intPtrEqual(existing.DestinationPort, desired.DestinationPort) {
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

// startRoute creates listeners for a single route on the gateway.
// Gateway listens on WireGuardIP:EntryPort and forwards to DestinationIP:DestinationPort.
func (s *ForwarderService) startRoute(r *domain.Route) (*routeRuntime, error) {
	ctx, cancel := context.WithCancel(context.Background())

	rt := &routeRuntime{
		route:  r,
		cancel: cancel,
	}

	switch r.Protocol {
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
		if err := s.startTCPListener(ctx, rt); err != nil {
			cancel()
			return nil, err
		}
		if err := s.startUDPListener(ctx, rt); err != nil {
			if rt.tcpListener != nil {
				rt.tcpListener.Close()
			}
			cancel()
			return nil, err
		}
	case domain.RouteProtocolICMP:
		if err := s.startICMPRoute(r); err != nil {
			s.logger.Warn("ICMP gateway route failed to start, skipping",
				slog.String("route_id", r.ID.String()),
				slog.String("route_name", r.Name),
				slog.String("error", err.Error()),
			)
			cancel()
			return nil, err
		}
	default:
		cancel()
		return nil, fmt.Errorf("unsupported protocol: %s", r.Protocol)
	}

	s.logger.Info("gateway route started",
		slog.String("route_id", r.ID.String()),
		slog.String("route_name", r.Name),
		slog.String("protocol", string(r.Protocol)),
		slog.String("listen", net.JoinHostPort(s.wireGuardIP, strconv.Itoa(safePort(r.EntryPort)))),
		slog.String("destination", net.JoinHostPort(r.DestinationIP, strconv.Itoa(safePort(r.DestinationPort)))),
	)

	return rt, nil
}

// startICMPRoute lazy-initializes the shared ICMP proxy and registers
// the route. Returns an error if the raw socket cannot be opened.
func (s *ForwarderService) startICMPRoute(r *domain.Route) error {
	var initErr error
	s.icmpOnce.Do(func() {
		s.icmpProxy, initErr = newICMPProxy(s.logger, nil)
	})
	if initErr != nil {
		return fmt.Errorf("init ICMP proxy: %w", initErr)
	}
	if s.icmpProxy == nil {
		return fmt.Errorf("ICMP proxy unavailable")
	}

	s.icmpProxy.addRoute(r.ID.String(), r.DestinationIP)
	return nil
}

// startTCPListener binds on WireGuardIP:EntryPort and forwards to DestinationIP:DestinationPort.
func (s *ForwarderService) startTCPListener(ctx context.Context, rt *routeRuntime) error {
	addr := net.JoinHostPort(s.wireGuardIP, strconv.Itoa(safePort(rt.route.EntryPort)))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen tcp %s: %w", addr, err)
	}
	rt.tcpListener = ln

	go s.tcpAcceptLoop(ctx, ln, rt)
	return nil
}

func (s *ForwarderService) tcpAcceptLoop(ctx context.Context, ln net.Listener, rt *routeRuntime) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				s.logger.Error("gateway tcp accept error",
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

// tcpRelay connects to the private destination and relays traffic bidirectionally.
func (s *ForwarderService) tcpRelay(ctx context.Context, clientConn net.Conn, rt *routeRuntime) {
	defer func() {
		clientConn.Close()
		s.connectionsOpen.Add(^uint64(0)) // Decrement.
	}()

	// Connect to private destination.
	destAddr := net.JoinHostPort(rt.route.DestinationIP, strconv.Itoa(safePort(rt.route.DestinationPort)))
	destConn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		s.logger.Error("gateway tcp dial destination failed",
			slog.String("route_id", rt.route.ID.String()),
			slog.String("destination", destAddr),
			slog.String("error", err.Error()),
		)
		return
	}
	defer destConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(destConn, clientConn)
		s.bytesForwarded.Add(uint64(n))
		s.recordTenantBytes(rt, n)
		done <- struct{}{}
	}()
	go func() {
		n, _ := io.Copy(clientConn, destConn)
		s.bytesForwarded.Add(uint64(n))
		s.recordTenantBytes(rt, n)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

// recordTenantBytes increments the per-tenant route bytes forwarded counter.
func (s *ForwarderService) recordTenantBytes(rt *routeRuntime, n int64) {
	if s.metrics != nil && rt.route.TenantID.String() != "" {
		s.metrics.TenantRouteBytesForwarded.WithLabelValues(rt.route.TenantID.String()).Add(float64(n))
	}
}

// startUDPListener binds on WireGuardIP:EntryPort and forwards to DestinationIP:DestinationPort.
func (s *ForwarderService) startUDPListener(ctx context.Context, rt *routeRuntime) error {
	addr := net.JoinHostPort(s.wireGuardIP, strconv.Itoa(safePort(rt.route.EntryPort)))
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", addr, err)
	}
	rt.udpConn = conn

	go s.udpReadLoop(ctx, conn, rt)
	return nil
}

// udpSession tracks a single upstream UDP session.
type udpSession struct {
	upstreamConn net.Conn
	lastActivity atomic.Int64
}

func (s *ForwarderService) udpReadLoop(ctx context.Context, conn net.PacketConn, rt *routeRuntime) {
	var sessions sync.Map
	buf := make([]byte, 65535)
	destAddr := net.JoinHostPort(rt.route.DestinationIP, strconv.Itoa(safePort(rt.route.DestinationPort)))

	for {
		select {
		case <-ctx.Done():
			sessions.Range(func(key, value any) bool {
				if sess, ok := value.(*udpSession); ok {
					sess.upstreamConn.Close()
				}
				return true
			})
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, clientAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				s.logger.Error("gateway udp read error",
					slog.String("route_id", rt.route.ID.String()),
					slog.String("error", err.Error()),
				)
				return
			}
		}

		clientKey := clientAddr.String()
		s.bytesForwarded.Add(uint64(n))
		s.recordTenantBytes(rt, int64(n))

		sessIface, loaded := sessions.Load(clientKey)
		if !loaded {
			destConn, err := net.DialTimeout("udp", destAddr, 10*time.Second)
			if err != nil {
				s.logger.Error("gateway udp dial destination failed",
					slog.String("route_id", rt.route.ID.String()),
					slog.String("destination", destAddr),
					slog.String("error", err.Error()),
				)
				continue
			}

			sess := &udpSession{upstreamConn: destConn}
			sess.lastActivity.Store(time.Now().Unix())
			sessions.Store(clientKey, sess)
			sessIface = sess

			go s.udpReverseRelay(ctx, conn, destConn, clientAddr, &sessions, clientKey, rt)
		}

		sess := sessIface.(*udpSession)
		sess.lastActivity.Store(time.Now().Unix())

		if _, err := sess.upstreamConn.Write(buf[:n]); err != nil {
			s.logger.Debug("gateway udp forward error",
				slog.String("route_id", rt.route.ID.String()),
				slog.String("error", err.Error()),
			)
			sess.upstreamConn.Close()
			sessions.Delete(clientKey)
		}
	}
}

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
				sessIface, ok := sessions.Load(clientKey)
				if !ok {
					return
				}
				sess := sessIface.(*udpSession)
				if time.Since(time.Unix(sess.lastActivity.Load(), 0)) > idleTimeout {
					return
				}
				continue
			}
			return
		}

		s.bytesForwarded.Add(uint64(n))
		s.recordTenantBytes(rt, int64(n))

		if _, err := downstreamConn.WriteTo(buf[:n], clientAddr); err != nil {
			s.logger.Debug("gateway udp reverse relay write error",
				slog.String("route_id", rt.route.ID.String()),
				slog.String("error", err.Error()),
			)
			return
		}
	}
}

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

func safePort(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
