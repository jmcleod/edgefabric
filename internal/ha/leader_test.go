package ha

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNoopLeaderElector_AlwaysLeader(t *testing.T) {
	var elected atomic.Bool
	var demoted atomic.Bool

	e := NewNoopLeaderElector(
		func() { elected.Store(true) },
		func() { demoted.Store(true) },
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- e.Start(ctx)
	}()

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)

	if !e.IsLeader() {
		t.Fatal("expected NoopLeaderElector to be leader")
	}
	if !elected.Load() {
		t.Fatal("expected onElected to be called")
	}

	// Stop it.
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}

	if !demoted.Load() {
		t.Fatal("expected onDemoted to be called on context cancel")
	}
}

func TestNoopLeaderElector_StopIsNoop(t *testing.T) {
	e := NewNoopLeaderElector(nil, nil)
	// Stop should not panic even without Start.
	e.Stop()
}

func TestPostgresLeaderElector_InterfaceCompliance(t *testing.T) {
	// Verify that both elector types implement the LeaderElector interface.
	var _ LeaderElector = (*PostgresLeaderElector)(nil)
	var _ LeaderElector = (*NoopLeaderElector)(nil)
}

func TestWithElectionInterval(t *testing.T) {
	e := &PostgresLeaderElector{}
	WithElectionInterval(10 * time.Second)(e)
	if e.interval != 10*time.Second {
		t.Fatalf("expected interval 10s, got %v", e.interval)
	}
}

func TestWithLockID(t *testing.T) {
	e := &PostgresLeaderElector{}
	WithLockID(42)(e)
	if e.lockID != 42 {
		t.Fatalf("expected lockID 42, got %d", e.lockID)
	}
}
