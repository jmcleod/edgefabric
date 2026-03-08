package gatewayrt

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

func TestNoopStartStop(t *testing.T) {
	svc := NewNoopService()
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

func TestNoopReconcile(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	if err := svc.Reconcile(ctx, nil); err == nil {
		t.Error("expected error reconciling before start")
	}

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	port80 := 80
	port443 := 443
	route1ID := domain.NewID()
	route2ID := domain.NewID()

	config := &route.GatewayRouteConfig{
		Routes: []*domain.Route{
			{
				ID: route1ID, Name: "gw-route-1",
				Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: &port443,
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: &port443,
				Status: domain.RouteStatusActive,
			},
			{
				ID: route2ID, Name: "gw-route-2",
				Protocol: domain.RouteProtocolUDP,
				EntryIP: "198.51.100.1", EntryPort: &port80,
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.2", DestinationPort: &port80,
				Status: domain.RouteStatusActive,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if svc.RouteCount() != 2 {
		t.Errorf("expected 2 routes, got %d", svc.RouteCount())
	}

	names := svc.RouteNames()
	if names[route1ID] != "gw-route-1" {
		t.Errorf("expected gw-route-1, got %q", names[route1ID])
	}

	// Remove one.
	config.Routes = config.Routes[:1]
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile reduced: %v", err)
	}
	if svc.RouteCount() != 1 {
		t.Errorf("expected 1 route, got %d", svc.RouteCount())
	}

	// Nil config clears all.
	if err := svc.Reconcile(ctx, nil); err != nil {
		t.Fatalf("reconcile nil: %v", err)
	}
	if svc.RouteCount() != 0 {
		t.Errorf("expected 0 routes, got %d", svc.RouteCount())
	}
}

func TestNoopGetStatus(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	status, _ := svc.GetStatus(ctx)
	if status.Running {
		t.Error("expected not running")
	}

	svc.Start(ctx)
	status, _ = svc.GetStatus(ctx)
	if !status.Running {
		t.Error("expected running")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0, got %d", status.ActiveRoutes)
	}

	port80 := 80
	svc.Reconcile(ctx, &route.GatewayRouteConfig{
		Routes: []*domain.Route{
			{
				ID: domain.NewID(), Name: "test",
				Protocol: domain.RouteProtocolTCP,
				EntryIP: "1.2.3.4", EntryPort: &port80,
				GatewayID: domain.NewID(), DestinationIP: "10.0.0.1", DestinationPort: &port80,
				Status: domain.RouteStatusActive,
			},
		},
	})

	status, _ = svc.GetStatus(ctx)
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1, got %d", status.ActiveRoutes)
	}

	svc.Stop(ctx)
	status, _ = svc.GetStatus(ctx)
	if status.Running {
		t.Error("expected not running after stop")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 after stop, got %d", status.ActiveRoutes)
	}
}
