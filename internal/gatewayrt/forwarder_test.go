package gatewayrt

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

func intPtr(v int) *int { return &v }

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

func TestGatewayForwarderStartStop(t *testing.T) {
	svc := NewForwarderService("127.0.0.1", slog.Default(), nil)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := svc.Stop(ctx); err == nil {
		t.Error("expected error on double stop")
	}
}

func TestGatewayForwarderTCPForward(t *testing.T) {
	// Start a TCP echo server to act as the private destination.
	destPort := freePort(t)
	echoLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", destPort))
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
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Use a different port for the gateway listener (WireGuardIP:EntryPort).
	listenPort := freePort(t)

	svc := NewForwarderService("127.0.0.1", slog.Default(), nil)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	config := &route.GatewayRouteConfig{
		Routes: []*domain.Route{
			{
				ID:              domain.NewID(),
				Name:            "tcp-gw-test",
				Protocol:        domain.RouteProtocolTCP,
				EntryIP:         "198.51.100.1",
				EntryPort:       intPtr(listenPort),
				GatewayID:       domain.NewID(),
				DestinationIP:   "127.0.0.1",
				DestinationPort: intPtr(destPort),
				Status:          domain.RouteStatusActive,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Connect to the gateway listener on WireGuardIP:EntryPort.
	clientConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listenPort), 5*time.Second)
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer clientConn.Close()

	msg := "hello from node via overlay"
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

func TestGatewayForwarderUDPForward(t *testing.T) {
	// Start a UDP echo server as private destination.
	destPort := freePort(t)
	echoConn, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", destPort))
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

	listenPort := freePort(t)

	svc := NewForwarderService("127.0.0.1", slog.Default(), nil)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	config := &route.GatewayRouteConfig{
		Routes: []*domain.Route{
			{
				ID:              domain.NewID(),
				Name:            "udp-gw-test",
				Protocol:        domain.RouteProtocolUDP,
				EntryIP:         "198.51.100.1",
				EntryPort:       intPtr(listenPort),
				GatewayID:       domain.NewID(),
				DestinationIP:   "127.0.0.1",
				DestinationPort: intPtr(destPort),
				Status:          domain.RouteStatusActive,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	clientConn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", listenPort), 5*time.Second)
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer clientConn.Close()

	msg := "hello udp gateway"
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
	if status.UDPListeners != 1 {
		t.Errorf("expected 1 udp listener, got %d", status.UDPListeners)
	}
}

func TestGatewayForwarderReconcile(t *testing.T) {
	svc := NewForwarderService("127.0.0.1", slog.Default(), nil)
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	port := freePort(t)
	routeID := domain.NewID()

	config := &route.GatewayRouteConfig{
		Routes: []*domain.Route{
			{
				ID:              routeID,
				Name:            "reconcile-test",
				Protocol:        domain.RouteProtocolTCP,
				EntryIP:         "198.51.100.1",
				EntryPort:       intPtr(port),
				GatewayID:       domain.NewID(),
				DestinationIP:   "10.0.1.1",
				DestinationPort: intPtr(8080),
				Status:          domain.RouteStatusActive,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile add: %v", err)
	}

	status, _ := svc.GetStatus(ctx)
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1, got %d", status.ActiveRoutes)
	}

	// Remove all routes.
	if err := svc.Reconcile(ctx, &route.GatewayRouteConfig{}); err != nil {
		t.Fatalf("reconcile remove: %v", err)
	}

	status, _ = svc.GetStatus(ctx)
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0, got %d", status.ActiveRoutes)
	}
}
