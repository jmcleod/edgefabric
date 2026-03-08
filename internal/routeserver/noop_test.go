package routeserver

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

func TestNoopStartStop(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Start.
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	// Stop.
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Double stop should fail.
	if err := svc.Stop(ctx); err == nil {
		t.Error("expected error on double stop")
	}
}

func TestNoopReconcile(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Reconcile without start should fail.
	if err := svc.Reconcile(ctx, nil); err == nil {
		t.Error("expected error reconciling before start")
	}

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	route1ID := domain.NewID()
	route2ID := domain.NewID()
	gwID := domain.NewID()
	port80 := 80
	port443 := 443

	// Reconcile with two routes.
	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              route1ID,
					Name:            "web-traffic",
					Protocol:        domain.RouteProtocolTCP,
					EntryIP:         "198.51.100.1",
					EntryPort:       &port443,
					GatewayID:       gwID,
					DestinationIP:   "10.0.1.1",
					DestinationPort: &port443,
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "10.100.0.5",
			},
			{
				Route: &domain.Route{
					ID:              route2ID,
					Name:            "http-traffic",
					Protocol:        domain.RouteProtocolTCP,
					EntryIP:         "198.51.100.1",
					EntryPort:       &port80,
					GatewayID:       gwID,
					DestinationIP:   "10.0.1.1",
					DestinationPort: &port80,
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "10.100.0.5",
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
	if names[route1ID] != "web-traffic" {
		t.Errorf("expected name web-traffic for route1, got %q", names[route1ID])
	}
	if names[route2ID] != "http-traffic" {
		t.Errorf("expected name http-traffic for route2, got %q", names[route2ID])
	}

	// Reconcile with one route removed.
	config.Routes = config.Routes[:1]
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile reduced: %v", err)
	}

	if svc.RouteCount() != 1 {
		t.Errorf("expected 1 route after reduction, got %d", svc.RouteCount())
	}

	names = svc.RouteNames()
	if _, ok := names[route2ID]; ok {
		t.Error("route2 should have been removed")
	}

	// Reconcile with nil config should clear all routes.
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

	// Status before start.
	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.Running {
		t.Error("expected not running before start")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 active routes, got %d", status.ActiveRoutes)
	}

	// Start and check status.
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after start: %v", err)
	}
	if !status.Running {
		t.Error("expected running after start")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 active routes, got %d", status.ActiveRoutes)
	}
	if status.BytesForwarded != 0 {
		t.Errorf("expected 0 bytes forwarded, got %d", status.BytesForwarded)
	}

	// Reconcile and check status.
	port80 := 80
	config := &route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              domain.NewID(),
					Name:            "test-route",
					Protocol:        domain.RouteProtocolTCP,
					EntryIP:         "198.51.100.1",
					EntryPort:       &port80,
					GatewayID:       domain.NewID(),
					DestinationIP:   "10.0.1.1",
					DestinationPort: &port80,
					Status:          domain.RouteStatusActive,
				},
				GatewayWGIP: "10.100.0.5",
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after reconcile: %v", err)
	}
	if status.ActiveRoutes != 1 {
		t.Errorf("expected 1 active route, got %d", status.ActiveRoutes)
	}

	// Stop resets routes.
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after stop: %v", err)
	}
	if status.Running {
		t.Error("expected not running after stop")
	}
	if status.ActiveRoutes != 0 {
		t.Errorf("expected 0 active routes after stop, got %d", status.ActiveRoutes)
	}
}
