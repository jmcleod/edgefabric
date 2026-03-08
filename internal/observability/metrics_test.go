package observability

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.Registry == nil {
		t.Fatal("Registry is nil")
	}
	if m.HTTPRequestsTotal == nil {
		t.Fatal("HTTPRequestsTotal is nil")
	}
	if m.ActiveNodes == nil {
		t.Fatal("ActiveNodes is nil")
	}
	if m.RouteConnectionsActive == nil {
		t.Fatal("RouteConnectionsActive is nil")
	}
	if m.CDNCacheHits == nil {
		t.Fatal("CDNCacheHits is nil")
	}
	if m.DNSQueriesTotal == nil {
		t.Fatal("DNSQueriesTotal is nil")
	}

	// Verify Handler() works.
	handler := m.Handler()
	if handler == nil {
		t.Fatal("Handler returned nil")
	}
}

func TestStartGaugeUpdater(t *testing.T) {
	m := NewMetrics()

	var callCount atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	StartGaugeUpdater(ctx, m, 50*time.Millisecond, func(metrics *Metrics) {
		callCount.Add(1)
		metrics.ActiveNodes.Set(42)
		metrics.ActiveGateways.Set(3)
		metrics.ActiveTenants.Set(7)
	})

	// Wait for at least one tick beyond the immediate call.
	time.Sleep(200 * time.Millisecond)

	count := callCount.Load()
	if count < 2 {
		t.Errorf("expected updater called at least 2 times (immediate + ticks), got %d", count)
	}

	// Cancel and verify it stops.
	cancel()
	time.Sleep(100 * time.Millisecond)
	countAfterCancel := callCount.Load()
	time.Sleep(100 * time.Millisecond)
	countLater := callCount.Load()
	if countLater > countAfterCancel+1 {
		t.Error("gauge updater did not stop after context cancellation")
	}
}
