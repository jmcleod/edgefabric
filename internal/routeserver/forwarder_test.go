package routeserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

func newTestForwarder() *ForwarderService {
	return NewForwarderService(slog.Default(), nil)
}

func intPtr(v int) *int { return &v }

func TestForwarderStartStop(t *testing.T) {
	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Double stop should fail.
	if err := svc.Stop(ctx); err == nil {
		t.Error("expected error on double stop")
	}
}

// freePort returns a free TCP port on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestForwarderTCPForward(t *testing.T) {
	// The gateway port is where the echo server listens.
	gatewayPort := freePort(t)

	// Start a TCP echo server on the gateway port (simulates gateway listener).
	echoLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", gatewayPort))
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}
	defer echoLn.Close()

	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // Echo back.
			}(conn)
		}
	}()

	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Configure route: the forwarder listens on entryPort and dials GatewayWGIP:EntryPort.
	// Since both use the same EntryPort, we need them on different ports.
	// We'll use EntryPort = gatewayPort (so the dial hits the echo server),
	// BUT the forwarder also binds to EntryIP:EntryPort = 127.0.0.1:gatewayPort
	// which conflicts with the echo server.
	//
	// Solution: We use EntryPort = entryPort for binding, and the dial goes to
	// GatewayWGIP:EntryPort = 127.0.0.1:entryPort which would be the forwarder
	// itself — that's a loop!
	//
	// The real solution for tests: we make EntryPort = gatewayPort and bind to
	// entryPort by temporarily patching. But we don't want test hooks in prod code.
	//
	// Instead: We accept that the design uses the SAME port, so for testing we
	// set EntryPort = gatewayPort. The echo server is on 127.0.0.1:gatewayPort.
	// The forwarder also tries to bind 127.0.0.1:gatewayPort — conflict!
	//
	// Final approach: Use entryPort for EntryPort. Start the echo server on
	// entryPort too but on a different address. Since macOS doesn't support
	// 127.0.0.2, we'll use a Unix domain-like test by skipping real forwarding
	// and just testing that the listener was created. For end-to-end, see
	// integration tests in Linux CI.
	//
	// HOWEVER: we can test by using loopback aliases if available, or just test
	// the reconcile/status behavior and skip the data path test on platforms
	// where 127.0.0.2 isn't available.

	// Try to use 127.0.0.2 — skip data path test if not available.
	testConn, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skip("skipping TCP data path test: 127.0.0.2 not available (add loopback alias for this test)")
	}
	testConn.Close()

	// 127.0.0.2 is available. Use it for the entry side.
	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              domain.NewID(),
					Name:            "tcp-test",
					Protocol:        domain.RouteProtocolTCP,
					EntryIP:         "127.0.0.2",
					EntryPort:       intPtr(gatewayPort),
					GatewayID:       domain.NewID(),
					DestinationIP:   "10.0.1.1",
					DestinationPort: intPtr(8080),
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "127.0.0.1",
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Connect to the forwarder's entry point.
	clientConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.2:%d", gatewayPort), 5*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer clientConn.Close()

	msg := "hello from client"
	if _, err := clientConn.Write([]byte(msg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(msg))
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(clientConn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != msg {
		t.Errorf("expected echo %q, got %q", msg, string(buf))
	}

	status, _ := svc.GetStatus(ctx)
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1 active route, got %d", status.ActiveRoutes)
	}
	if status.TCPListeners != 1 {
		t.Errorf("expected 1 tcp listener, got %d", status.TCPListeners)
	}
}

func TestForwarderUDPForward(t *testing.T) {
	gatewayPort := freePort(t)

	echoConn, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", gatewayPort))
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}
	defer echoConn.Close()

	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := echoConn.ReadFrom(buf)
			if err != nil {
				return
			}
			echoConn.WriteTo(buf[:n], addr)
		}
	}()

	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Check if 127.0.0.2 is available for the entry side.
	testConn, err := net.ListenPacket("udp", "127.0.0.2:0")
	if err != nil {
		t.Skip("skipping UDP data path test: 127.0.0.2 not available")
	}
	testConn.Close()

	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              domain.NewID(),
					Name:            "udp-test",
					Protocol:        domain.RouteProtocolUDP,
					EntryIP:         "127.0.0.2",
					EntryPort:       intPtr(gatewayPort),
					GatewayID:       domain.NewID(),
					DestinationIP:   "10.0.1.1",
					DestinationPort: intPtr(53),
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "127.0.0.1",
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	clientConn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.2:%d", gatewayPort), 5*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer clientConn.Close()

	msg := "hello udp"
	if _, err := clientConn.Write([]byte(msg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(msg))
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(clientConn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != msg {
		t.Errorf("expected echo %q, got %q", msg, string(buf))
	}

	status, _ := svc.GetStatus(ctx)
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1 active route, got %d", status.ActiveRoutes)
	}
	if status.UDPListeners != 1 {
		t.Errorf("expected 1 udp listener, got %d", status.UDPListeners)
	}
}

func TestForwarderReconcileAddRemove(t *testing.T) {
	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Use a free port on 127.0.0.1 for a simple TCP listener test.
	port := freePort(t)

	routeID := domain.NewID()
	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              routeID,
					Name:            "add-remove-test",
					Protocol:        domain.RouteProtocolTCP,
					EntryIP:         "127.0.0.1",
					EntryPort:       intPtr(port),
					GatewayID:       domain.NewID(),
					DestinationIP:   "10.0.1.1",
					DestinationPort: intPtr(8080),
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "10.100.0.5",
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile add: %v", err)
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1 active route after add, got %d", status.ActiveRoutes)
	}
	if status.TCPListeners != 1 {
		t.Errorf("expected 1 tcp listener, got %d", status.TCPListeners)
	}

	// Verify the listener is active by trying to connect.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("expected listener to be active: %v", err)
	}
	conn.Close()

	// Reconcile with empty config — route should be removed.
	if err := svc.Reconcile(ctx, &route.NodeRouteConfig{}); err != nil {
		t.Fatalf("reconcile remove: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after remove: %v", err)
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 active routes after remove, got %d", status.ActiveRoutes)
	}
	if status.TCPListeners != 0 {
		t.Errorf("expected 0 tcp listeners, got %d", status.TCPListeners)
	}

	// Listener should be gone — connection should be refused.
	time.Sleep(50 * time.Millisecond)
	conn, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err == nil {
		conn.Close()
		t.Error("expected connection to be refused after route removal")
	}
}

func TestForwarderGetStatus(t *testing.T) {
	svc := newTestForwarder()
	ctx := context.Background()

	status, _ := svc.GetStatus(ctx)
	if status.Running {
		t.Error("expected not running before start")
	}

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	status, _ = svc.GetStatus(ctx)
	if !status.Running {
		t.Error("expected running after start")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 active routes, got %d", status.ActiveRoutes)
	}
	if status.BytesForwarded != 0 {
		t.Errorf("expected 0 bytes forwarded, got %d", status.BytesForwarded)
	}
}

func TestForwarderICMPSkipped(t *testing.T) {
	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// ICMP routes require CAP_NET_RAW for the raw socket.
	// Without CAP_NET_RAW the ICMP proxy fails to init, and the route
	// is silently skipped (logged as warning, reconcile continues).
	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:            domain.NewID(),
					Name:          "icmp-test",
					Protocol:      domain.RouteProtocolICMP,
					EntryIP:       "198.51.100.1",
					GatewayID:     domain.NewID(),
					DestinationIP: "10.0.1.1",
					Status:        domain.RouteStatusActive,
				},
				GatewayWGIP: "10.100.0.5",
			},
		},
	}

	// Reconcile should succeed — ICMP route failure doesn't break reconcile.
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Without CAP_NET_RAW, 0 active routes (ICMP skipped gracefully).
	// With CAP_NET_RAW, the route would be active — either way, no error.
	status, _ := svc.GetStatus(ctx)
	t.Logf("active routes: %d, icmp routes: %d", status.ActiveRoutes, status.ICMPRoutes)
}
