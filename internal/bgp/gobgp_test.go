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

func TestGoBGPAnnounceIPv6(t *testing.T) {
	svc := NewGoBGPService()
	ctx := context.Background()

	if err := svc.Start(ctx, "127.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Announce an IPv6 prefix.
	if err := svc.AnnouncePrefix(ctx, "fd00:ef::/48", "fd00:ef::1"); err != nil {
		t.Fatalf("announce IPv6: %v", err)
	}

	// Verify prefix is tracked.
	if len(svc.prefixes) != 1 {
		t.Errorf("expected 1 prefix tracked, got %d", len(svc.prefixes))
	}
	if _, ok := svc.prefixes["fd00:ef::/48"]; !ok {
		t.Error("expected fd00:ef::/48 to be tracked")
	}

	// Announce a second IPv6 prefix.
	if err := svc.AnnouncePrefix(ctx, "2001:db8::/32", "fd00:ef::1"); err != nil {
		t.Fatalf("announce IPv6 second: %v", err)
	}

	if len(svc.prefixes) != 2 {
		t.Errorf("expected 2 prefixes tracked, got %d", len(svc.prefixes))
	}
}

func TestGoBGPAnnounceWithdrawIPv6(t *testing.T) {
	svc := NewGoBGPService()
	ctx := context.Background()

	if err := svc.Start(ctx, "127.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Announce IPv6 prefix.
	if err := svc.AnnouncePrefix(ctx, "fd00:ef::/48", "fd00:ef::1"); err != nil {
		t.Fatalf("announce: %v", err)
	}

	if len(svc.prefixes) != 1 {
		t.Errorf("expected 1 prefix, got %d", len(svc.prefixes))
	}

	// Withdraw it.
	if err := svc.WithdrawPrefix(ctx, "fd00:ef::/48"); err != nil {
		t.Fatalf("withdraw IPv6: %v", err)
	}

	if len(svc.prefixes) != 0 {
		t.Errorf("expected 0 prefixes after withdraw, got %d", len(svc.prefixes))
	}
}

func TestGoBGPAnnounceMixedIPv4IPv6(t *testing.T) {
	svc := NewGoBGPService()
	ctx := context.Background()

	if err := svc.Start(ctx, "127.0.0.1", 65000); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	// Announce both IPv4 and IPv6 prefixes.
	if err := svc.AnnouncePrefix(ctx, "10.0.0.0/24", "127.0.0.1"); err != nil {
		t.Fatalf("announce IPv4: %v", err)
	}
	if err := svc.AnnouncePrefix(ctx, "fd00:ef::/48", "fd00:ef::1"); err != nil {
		t.Fatalf("announce IPv6: %v", err)
	}

	if len(svc.prefixes) != 2 {
		t.Errorf("expected 2 prefixes (mixed), got %d", len(svc.prefixes))
	}

	// Withdraw IPv4 only.
	if err := svc.WithdrawPrefix(ctx, "10.0.0.0/24"); err != nil {
		t.Fatalf("withdraw IPv4: %v", err)
	}

	if len(svc.prefixes) != 1 {
		t.Errorf("expected 1 prefix after IPv4 withdraw, got %d", len(svc.prefixes))
	}
	if _, ok := svc.prefixes["fd00:ef::/48"]; !ok {
		t.Error("IPv6 prefix should still be tracked")
	}
}
