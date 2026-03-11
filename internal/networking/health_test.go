package networking

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

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

func TestOverlayHealthChecker_HealthyPeer(t *testing.T) {
	l, port := startTestListener(t)
	defer l.Close()

	bus := events.NewBus(slog.Default())
	metrics := observability.NewMetrics()
	config := OverlayHealthConfig{
		Interval:           50 * time.Millisecond,
		Timeout:            1 * time.Second,
		UnhealthyThreshold: 3,
		HealthyThreshold:   1,
		ProbePort:          port,
	}

	hc := NewOverlayHealthChecker(
		[]OverlayTarget{{Name: "test-peer", IP: "127.0.0.1"}},
		config, bus, metrics, nil,
	)

	hc.Start()
	time.Sleep(80 * time.Millisecond)
	hc.Stop()

	if !hc.IsHealthy("127.0.0.1") {
		t.Error("expected peer to be healthy")
	}
}

func TestOverlayHealthChecker_UnhealthyAfterThreshold(t *testing.T) {
	// Start then close — peer is unreachable.
	l, port := startTestListener(t)
	l.Close()

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.OverlayPeerUnreachable, collector.handler())
	bus.Subscribe(events.OverlayPeerRecovered, collector.handler())

	metrics := observability.NewMetrics()
	config := OverlayHealthConfig{
		Interval:           30 * time.Millisecond,
		Timeout:            50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
		ProbePort:          port,
	}

	hc := NewOverlayHealthChecker(
		[]OverlayTarget{{Name: "dead-peer", IP: "127.0.0.1"}},
		config, bus, metrics, nil,
	)

	hc.Start()
	// Wait for enough checks to pass the threshold (2).
	time.Sleep(150 * time.Millisecond)
	hc.Stop()

	// Allow async event handlers.
	time.Sleep(20 * time.Millisecond)

	if hc.IsHealthy("127.0.0.1") {
		t.Error("expected peer to be unhealthy after threshold")
	}

	evts := collector.getEvents()
	foundUnreachable := false
	for _, e := range evts {
		if e.Type == events.OverlayPeerUnreachable {
			foundUnreachable = true
			if e.Severity != events.SeverityCritical {
				t.Errorf("expected critical severity, got %s", e.Severity)
			}
		}
	}
	if !foundUnreachable {
		t.Error("expected OverlayPeerUnreachable event")
	}
}

func TestOverlayHealthChecker_RecoveryEvent(t *testing.T) {
	// Start with closed listener.
	l, port := startTestListener(t)
	l.Close()

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.OverlayPeerUnreachable, collector.handler())
	bus.Subscribe(events.OverlayPeerRecovered, collector.handler())

	metrics := observability.NewMetrics()
	config := OverlayHealthConfig{
		Interval:           30 * time.Millisecond,
		Timeout:            50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
		ProbePort:          port,
	}

	hc := NewOverlayHealthChecker(
		[]OverlayTarget{{Name: "flappy-peer", IP: "127.0.0.1"}},
		config, bus, metrics, nil,
	)

	hc.Start()
	// Wait for unhealthy transition.
	time.Sleep(150 * time.Millisecond)

	// Restart listener on the same port — recovery.
	newL, err := net.Listen("tcp", l.Addr().String())
	if err != nil {
		// Port may have been reclaimed; skip recovery test.
		hc.Stop()
		t.Skip("could not reopen listener on same port")
	}
	go func() {
		for {
			conn, err := newL.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	defer newL.Close()

	// Wait for recovery.
	time.Sleep(100 * time.Millisecond)
	hc.Stop()
	time.Sleep(20 * time.Millisecond)

	if !hc.IsHealthy("127.0.0.1") {
		t.Error("expected peer to recover to healthy")
	}

	evts := collector.getEvents()
	foundRecovered := false
	for _, e := range evts {
		if e.Type == events.OverlayPeerRecovered {
			foundRecovered = true
		}
	}
	if !foundRecovered {
		t.Error("expected OverlayPeerRecovered event")
	}
}
