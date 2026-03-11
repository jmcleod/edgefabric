package routeserver

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// eventCollector captures events published to the bus.
type eventCollector struct {
	mu     sync.Mutex
	events []events.Event
}

func (ec *eventCollector) handler() events.Handler {
	return func(_ context.Context, e events.Event) error {
		ec.mu.Lock()
		defer ec.mu.Unlock()
		ec.events = append(ec.events, e)
		return nil
	}
}

func (ec *eventCollector) getEvents() []events.Event {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	cpy := make([]events.Event, len(ec.events))
	copy(cpy, ec.events)
	return cpy
}

func startTestListener(t *testing.T) (net.Listener, int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	// Accept connections in background so DialTimeout succeeds.
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return l, port
}

func TestRouteHealthChecker_HealthyRoute(t *testing.T) {
	l, port := startTestListener(t)
	defer l.Close()

	bus := events.NewBus(slog.Default())
	metrics := observability.NewMetrics()
	config := RouteHealthConfig{
		Interval:           50 * time.Millisecond,
		Timeout:            1 * time.Second,
		UnhealthyThreshold: 3,
		HealthyThreshold:   1,
	}

	hc := NewRouteHealthChecker(config, bus, metrics, nil)

	routeID := domain.NewID()
	hc.UpdateTargets([]RouteTarget{
		{
			RouteID:     routeID,
			RouteName:   "test-route",
			Protocol:    domain.RouteProtocolTCP,
			GatewayWGIP: "127.0.0.1",
			EntryPort:   port,
		},
	})

	hc.Start()
	// Wait for at least one check cycle.
	time.Sleep(80 * time.Millisecond)
	hc.Stop()

	if !hc.IsRouteHealthy(routeID) {
		t.Error("expected route to be healthy")
	}
}

func TestRouteHealthChecker_UnhealthyAfterThreshold(t *testing.T) {
	// Start a listener then close it.
	l, port := startTestListener(t)
	l.Close()

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.RouteHealthCheckFailed, collector.handler())
	bus.Subscribe(events.RouteHealthCheckRecovered, collector.handler())

	metrics := observability.NewMetrics()
	config := RouteHealthConfig{
		Interval:           30 * time.Millisecond,
		Timeout:            50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}

	hc := NewRouteHealthChecker(config, bus, metrics, nil)

	routeID := domain.NewID()
	hc.UpdateTargets([]RouteTarget{
		{
			RouteID:     routeID,
			RouteName:   "fail-route",
			Protocol:    domain.RouteProtocolTCP,
			GatewayWGIP: "127.0.0.1",
			EntryPort:   port,
		},
	})

	hc.Start()
	// Wait for enough check cycles to pass the unhealthy threshold (2).
	// Initial check + 2 ticks = 3 checks, well past threshold of 2.
	time.Sleep(150 * time.Millisecond)
	hc.Stop()

	// Allow async event handlers.
	time.Sleep(20 * time.Millisecond)

	if hc.IsRouteHealthy(routeID) {
		t.Error("expected route to be unhealthy after threshold")
	}

	evts := collector.getEvents()
	foundFailed := false
	for _, e := range evts {
		if e.Type == events.RouteHealthCheckFailed {
			foundFailed = true
			if e.Severity != events.SeverityCritical {
				t.Errorf("expected critical severity, got %s", e.Severity)
			}
		}
	}
	if !foundFailed {
		t.Error("expected RouteHealthCheckFailed event")
	}
}

func TestRouteHealthChecker_UpdateTargets(t *testing.T) {
	bus := events.NewBus(slog.Default())
	metrics := observability.NewMetrics()
	config := DefaultRouteHealthConfig()

	hc := NewRouteHealthChecker(config, bus, metrics, nil)

	id1 := domain.NewID()
	id2 := domain.NewID()

	// Add two routes.
	hc.UpdateTargets([]RouteTarget{
		{RouteID: id1, RouteName: "route-1", Protocol: domain.RouteProtocolTCP, GatewayWGIP: "10.0.0.1", EntryPort: 8080},
		{RouteID: id2, RouteName: "route-2", Protocol: domain.RouteProtocolTCP, GatewayWGIP: "10.0.0.1", EntryPort: 9090},
	})

	hc.mu.RLock()
	if len(hc.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(hc.routes))
	}
	hc.mu.RUnlock()

	// Remove one route.
	hc.UpdateTargets([]RouteTarget{
		{RouteID: id1, RouteName: "route-1", Protocol: domain.RouteProtocolTCP, GatewayWGIP: "10.0.0.1", EntryPort: 8080},
	})

	hc.mu.RLock()
	if len(hc.routes) != 1 {
		t.Errorf("expected 1 route after removal, got %d", len(hc.routes))
	}
	hc.mu.RUnlock()
}
