package bgp

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestNoopStartStop(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Start.
	if err := svc.Start(ctx, "10.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx, "10.0.0.1", 65000); err == nil {
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

	if err := svc.Start(ctx, "10.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Reconcile with two sessions.
	sessions := []*domain.BGPSession{
		{
			ID:                domain.NewID(),
			PeerASN:           65001,
			PeerAddress:       "192.0.2.1",
			LocalASN:          65000,
			AnnouncedPrefixes: []string{"10.0.0.0/24"},
		},
		{
			ID:          domain.NewID(),
			PeerASN:     65002,
			PeerAddress: "192.0.2.2",
			LocalASN:    65000,
		},
	}

	if err := svc.Reconcile(ctx, sessions); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if svc.SessionCount() != 2 {
		t.Errorf("expected 2 sessions, got %d", svc.SessionCount())
	}

	// Check announced prefixes.
	prefixes := svc.AnnouncedPrefixes()
	if _, ok := prefixes["10.0.0.0/24"]; !ok {
		t.Error("expected prefix 10.0.0.0/24 to be announced")
	}

	// GetStatus should return 2 established sessions.
	states, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	for _, state := range states {
		if state.Status != "established" {
			t.Errorf("expected established, got %s", state.Status)
		}
	}

	// Reconcile with one session removed.
	if err := svc.Reconcile(ctx, sessions[:1]); err != nil {
		t.Fatalf("reconcile reduced: %v", err)
	}

	if svc.SessionCount() != 1 {
		t.Errorf("expected 1 session after reduction, got %d", svc.SessionCount())
	}

	// Reconcile with empty desired state.
	if err := svc.Reconcile(ctx, nil); err != nil {
		t.Fatalf("reconcile empty: %v", err)
	}
	if svc.SessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", svc.SessionCount())
	}
}

func TestNoopAnnounceWithdraw(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Announce before start should fail.
	if err := svc.AnnouncePrefix(ctx, "10.0.0.0/24", "10.0.0.1"); err == nil {
		t.Error("expected error announcing before start")
	}

	if err := svc.Start(ctx, "10.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Announce.
	if err := svc.AnnouncePrefix(ctx, "10.0.0.0/24", "10.0.0.1"); err != nil {
		t.Fatalf("announce: %v", err)
	}
	if err := svc.AnnouncePrefix(ctx, "10.1.0.0/24", "10.0.0.1"); err != nil {
		t.Fatalf("announce 2: %v", err)
	}

	prefixes := svc.AnnouncedPrefixes()
	if len(prefixes) != 2 {
		t.Errorf("expected 2 prefixes, got %d", len(prefixes))
	}

	// Withdraw.
	if err := svc.WithdrawPrefix(ctx, "10.0.0.0/24"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	prefixes = svc.AnnouncedPrefixes()
	if len(prefixes) != 1 {
		t.Errorf("expected 1 prefix after withdraw, got %d", len(prefixes))
	}
	if _, ok := prefixes["10.1.0.0/24"]; !ok {
		t.Error("expected 10.1.0.0/24 to still be announced")
	}

	// Withdraw non-existent prefix should be a no-op (no error).
	if err := svc.WithdrawPrefix(ctx, "192.168.0.0/16"); err != nil {
		t.Fatalf("withdraw non-existent: %v", err)
	}
}
