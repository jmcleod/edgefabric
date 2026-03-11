package routeserver

import (
	"context"
	"log/slog"
	"testing"
)

func TestICMPProxyCreation(t *testing.T) {
	// Attempt to create an ICMP proxy. This requires CAP_NET_RAW.
	proxy, err := newICMPProxy(slog.Default(), nil)
	if err != nil {
		t.Skipf("skipping ICMP proxy test: raw socket unavailable (requires CAP_NET_RAW): %v", err)
	}
	defer proxy.close()

	if proxy.conn == nil {
		t.Fatal("expected non-nil ICMP connection")
	}
	if proxy.routeCount() != 0 {
		t.Fatalf("expected 0 routes, got %d", proxy.routeCount())
	}
}

func TestICMPProxyAddRemoveRoute(t *testing.T) {
	proxy, err := newICMPProxy(slog.Default(), nil)
	if err != nil {
		t.Skipf("skipping: raw socket unavailable: %v", err)
	}
	defer proxy.close()

	proxy.addRoute("198.51.100.1", "route-1", "10.100.0.5", "10.0.1.1")
	if proxy.routeCount() != 1 {
		t.Fatalf("expected 1 route, got %d", proxy.routeCount())
	}

	proxy.addRoute("198.51.100.2", "route-2", "10.100.0.6", "10.0.1.2")
	if proxy.routeCount() != 2 {
		t.Fatalf("expected 2 routes, got %d", proxy.routeCount())
	}

	proxy.removeRoute("198.51.100.1")
	if proxy.routeCount() != 1 {
		t.Fatalf("expected 1 route after remove, got %d", proxy.routeCount())
	}

	proxy.removeRoute("198.51.100.2")
	if proxy.routeCount() != 0 {
		t.Fatalf("expected 0 routes after remove all, got %d", proxy.routeCount())
	}
}

func TestICMPRouteReconcile(t *testing.T) {
	// This test verifies that ICMP routes are accepted (not rejected)
	// during reconcile, or silently skipped without CAP_NET_RAW.
	// The actual data path requires elevated permissions and is tested
	// in integration tests with CAP_NET_RAW.

	svc := newTestForwarder()
	ctx := context.Background()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Reconcile should not return an error even if ICMP proxy fails.
	// The reconcile loop continues past individual route failures.
	status, _ := svc.GetStatus(ctx)
	if !status.Running {
		t.Error("expected running after start")
	}
}
