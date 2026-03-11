package bgp

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// mockBGPService implements Service with controllable status responses.
type mockBGPService struct {
	mu       sync.Mutex
	sessions []SessionState
}

func (m *mockBGPService) Start(_ context.Context, _ string, _ uint32) error { return nil }
func (m *mockBGPService) Stop(_ context.Context) error                      { return nil }
func (m *mockBGPService) Reconcile(_ context.Context, _ []*domain.BGPSession) error {
	return nil
}
func (m *mockBGPService) AnnouncePrefix(_ context.Context, _, _ string) error { return nil }
func (m *mockBGPService) WithdrawPrefix(_ context.Context, _ string) error    { return nil }

func (m *mockBGPService) GetStatus(_ context.Context) ([]SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := make([]SessionState, len(m.sessions))
	copy(cpy, m.sessions)
	return cpy, nil
}

func (m *mockBGPService) setSessions(sessions []SessionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = sessions
}

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

func TestMonitor_NoEventOnFirstPoll(t *testing.T) {
	svc := &mockBGPService{
		sessions: []SessionState{
			{PeerAddress: "10.0.0.1", PeerASN: 65001, Status: "established"},
		},
	}

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.BGPSessionEstablished, collector.handler())
	bus.Subscribe(events.BGPSessionDown, collector.handler())

	metrics := observability.NewMetrics()
	mon := NewMonitor(svc, MonitorConfig{PollInterval: 50 * time.Millisecond}, bus, metrics, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon.Start(ctx)
	// Wait long enough for the initial poll but not a second tick.
	time.Sleep(30 * time.Millisecond)
	mon.Stop()

	evts := collector.getEvents()
	if len(evts) != 0 {
		t.Errorf("expected no events on initial poll, got %d: %+v", len(evts), evts)
	}
}

func TestMonitor_SessionEstablishedEvent(t *testing.T) {
	svc := &mockBGPService{
		sessions: []SessionState{
			{PeerAddress: "10.0.0.1", PeerASN: 65001, Status: "active"},
		},
	}

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.BGPSessionEstablished, collector.handler())
	bus.Subscribe(events.BGPSessionDown, collector.handler())

	metrics := observability.NewMetrics()
	mon := NewMonitor(svc, MonitorConfig{PollInterval: 50 * time.Millisecond}, bus, metrics, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon.Start(ctx)
	// Wait for initial poll to capture baseline.
	time.Sleep(30 * time.Millisecond)

	// Transition to established.
	svc.setSessions([]SessionState{
		{PeerAddress: "10.0.0.1", PeerASN: 65001, Status: "established"},
	})

	// Wait for at least one poll.
	time.Sleep(80 * time.Millisecond)
	mon.Stop()

	// Allow async handler to complete.
	time.Sleep(20 * time.Millisecond)

	evts := collector.getEvents()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(evts), evts)
	}
	if evts[0].Type != events.BGPSessionEstablished {
		t.Errorf("expected BGPSessionEstablished, got %s", evts[0].Type)
	}
	if evts[0].Severity != events.SeverityInfo {
		t.Errorf("expected severity info, got %s", evts[0].Severity)
	}
}

func TestMonitor_SessionDownEvent(t *testing.T) {
	svc := &mockBGPService{
		sessions: []SessionState{
			{PeerAddress: "10.0.0.1", PeerASN: 65001, Status: "established"},
		},
	}

	bus := events.NewBus(slog.Default())
	collector := &eventCollector{}
	bus.Subscribe(events.BGPSessionEstablished, collector.handler())
	bus.Subscribe(events.BGPSessionDown, collector.handler())

	metrics := observability.NewMetrics()
	mon := NewMonitor(svc, MonitorConfig{PollInterval: 50 * time.Millisecond}, bus, metrics, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	// Transition away from established.
	svc.setSessions([]SessionState{
		{PeerAddress: "10.0.0.1", PeerASN: 65001, Status: "idle"},
	})

	time.Sleep(80 * time.Millisecond)
	mon.Stop()
	time.Sleep(20 * time.Millisecond)

	evts := collector.getEvents()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(evts), evts)
	}
	if evts[0].Type != events.BGPSessionDown {
		t.Errorf("expected BGPSessionDown, got %s", evts[0].Type)
	}
	if evts[0].Severity != events.SeverityCritical {
		t.Errorf("expected severity critical, got %s", evts[0].Severity)
	}
}
