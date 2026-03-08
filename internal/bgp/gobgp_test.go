package bgp

import (
	"context"
	"testing"
)

func TestGoBGPStartStop(t *testing.T) {
	svc := NewGoBGPService()
	ctx := context.Background()

	// Start with a private ASN and localhost router ID.
	if err := svc.Start(ctx, "127.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx, "127.0.0.1", 65000); err == nil {
		t.Error("expected error on double start")
	}

	// GetStatus should return empty (no peers configured).
	states, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
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

func TestGoBGPAnnounceWithdraw(t *testing.T) {
	svc := NewGoBGPService()
	ctx := context.Background()

	if err := svc.Start(ctx, "127.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Announce a prefix.
	if err := svc.AnnouncePrefix(ctx, "10.0.0.0/24", "127.0.0.1"); err != nil {
		t.Fatalf("announce: %v", err)
	}

	// Verify prefix is tracked.
	if len(svc.prefixes) != 1 {
		t.Errorf("expected 1 prefix tracked, got %d", len(svc.prefixes))
	}

	// Announce another prefix.
	if err := svc.AnnouncePrefix(ctx, "10.1.0.0/24", "127.0.0.1"); err != nil {
		t.Fatalf("announce 2: %v", err)
	}

	if len(svc.prefixes) != 2 {
		t.Errorf("expected 2 prefixes tracked, got %d", len(svc.prefixes))
	}

	// Withdraw first prefix.
	if err := svc.WithdrawPrefix(ctx, "10.0.0.0/24"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	if len(svc.prefixes) != 1 {
		t.Errorf("expected 1 prefix after withdraw, got %d", len(svc.prefixes))
	}

	// Withdraw non-existent prefix should be a no-op.
	if err := svc.WithdrawPrefix(ctx, "192.168.0.0/16"); err != nil {
		t.Fatalf("withdraw non-existent: %v", err)
	}
}
