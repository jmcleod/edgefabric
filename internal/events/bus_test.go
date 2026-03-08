package events

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBus_PublishDelivers(t *testing.T) {
	bus := NewBus(testLogger())

	var received atomic.Int32
	bus.Subscribe(NodeStatusChanged, func(ctx context.Context, event Event) error {
		received.Add(1)
		return nil
	})

	bus.Publish(context.Background(), Event{
		Type:     NodeStatusChanged,
		Severity: SeverityInfo,
		Resource: "node/abc",
	})

	// Wait briefly for async handler.
	time.Sleep(50 * time.Millisecond)
	if received.Load() != 1 {
		t.Errorf("expected 1 delivery, got %d", received.Load())
	}
}

func TestBus_PublishNoSubscribers(t *testing.T) {
	bus := NewBus(testLogger())

	// Should not panic when there are no subscribers.
	bus.Publish(context.Background(), Event{
		Type:     GatewayStatusChanged,
		Severity: SeverityWarning,
		Resource: "gateway/xyz",
	})
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus(testLogger())

	var count atomic.Int32
	for i := 0; i < 3; i++ {
		bus.Subscribe(ProvisioningFailed, func(ctx context.Context, event Event) error {
			count.Add(1)
			return nil
		})
	}

	bus.Publish(context.Background(), Event{
		Type:     ProvisioningFailed,
		Severity: SeverityCritical,
		Resource: "provisioning/job-1",
		Details:  map[string]string{"reason": "timeout"},
	})

	time.Sleep(50 * time.Millisecond)
	if count.Load() != 3 {
		t.Errorf("expected 3 deliveries, got %d", count.Load())
	}
}

func TestBus_HandlerError(t *testing.T) {
	bus := NewBus(testLogger())

	var successCalled atomic.Int32
	// Register a failing handler and a succeeding handler.
	bus.Subscribe(HealthCheckFailed, func(ctx context.Context, event Event) error {
		return fmt.Errorf("handler broke")
	})
	bus.Subscribe(HealthCheckFailed, func(ctx context.Context, event Event) error {
		successCalled.Add(1)
		return nil
	})

	bus.Publish(context.Background(), Event{
		Type:     HealthCheckFailed,
		Severity: SeverityWarning,
		Resource: "node/health",
	})

	time.Sleep(50 * time.Millisecond)
	// The succeeding handler should still have been called despite the other failing.
	if successCalled.Load() != 1 {
		t.Errorf("expected success handler to be called, got %d", successCalled.Load())
	}
}

func TestBus_DifferentEventTypes(t *testing.T) {
	bus := NewBus(testLogger())

	var nodeCount, gwCount atomic.Int32
	bus.Subscribe(NodeStatusChanged, func(ctx context.Context, event Event) error {
		nodeCount.Add(1)
		return nil
	})
	bus.Subscribe(GatewayStatusChanged, func(ctx context.Context, event Event) error {
		gwCount.Add(1)
		return nil
	})

	bus.Publish(context.Background(), Event{Type: NodeStatusChanged, Severity: SeverityInfo, Resource: "node/1"})
	bus.Publish(context.Background(), Event{Type: NodeStatusChanged, Severity: SeverityInfo, Resource: "node/2"})
	bus.Publish(context.Background(), Event{Type: GatewayStatusChanged, Severity: SeverityInfo, Resource: "gw/1"})

	time.Sleep(50 * time.Millisecond)
	if nodeCount.Load() != 2 {
		t.Errorf("expected 2 node events, got %d", nodeCount.Load())
	}
	if gwCount.Load() != 1 {
		t.Errorf("expected 1 gateway event, got %d", gwCount.Load())
	}
}

func TestLogHandler(t *testing.T) {
	logger := testLogger()
	handler := NewLogHandler(logger)

	// Should not panic or error for any severity.
	for _, sev := range []Severity{SeverityInfo, SeverityWarning, SeverityCritical} {
		err := handler(context.Background(), Event{
			Type:     NodeStatusChanged,
			Severity: sev,
			Resource: "node/test",
			Details:  map[string]string{"old_status": "online", "new_status": "offline"},
		})
		if err != nil {
			t.Errorf("log handler returned error for severity %s: %v", sev, err)
		}
	}
}
