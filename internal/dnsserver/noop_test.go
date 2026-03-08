package dnsserver

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestNoopStartStop(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Start.
	if err := svc.Start(ctx, ":5353"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx, ":5353"); err == nil {
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

	if err := svc.Start(ctx, ":5353"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Reconcile with two zones.
	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{
						ID:    domain.NewID(),
						Name:  "www",
						Type:  domain.DNSRecordTypeA,
						Value: "192.0.2.1",
					},
				},
			},
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.org.",
					Serial: 5,
					TTL:    600,
					Status: domain.DNSZoneActive,
				},
				Records: nil,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if svc.ZoneCount() != 2 {
		t.Errorf("expected 2 zones, got %d", svc.ZoneCount())
	}

	serials := svc.ZoneSerials()
	if serials["example.com."] != 1 {
		t.Errorf("expected serial 1 for example.com., got %d", serials["example.com."])
	}
	if serials["example.org."] != 5 {
		t.Errorf("expected serial 5 for example.org., got %d", serials["example.org."])
	}

	// Reconcile with one zone removed.
	config.Zones = config.Zones[:1]
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile reduced: %v", err)
	}

	if svc.ZoneCount() != 1 {
		t.Errorf("expected 1 zone after reduction, got %d", svc.ZoneCount())
	}

	serials = svc.ZoneSerials()
	if _, ok := serials["example.org."]; ok {
		t.Error("example.org. should have been removed")
	}

	// Reconcile with nil config should clear all zones.
	if err := svc.Reconcile(ctx, nil); err != nil {
		t.Fatalf("reconcile nil: %v", err)
	}
	if svc.ZoneCount() != 0 {
		t.Errorf("expected 0 zones, got %d", svc.ZoneCount())
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
	if status.Listening {
		t.Error("expected not listening before start")
	}
	if status.ListenAddr != "" {
		t.Errorf("expected empty listen addr, got %q", status.ListenAddr)
	}

	// Start and check status.
	if err := svc.Start(ctx, ":5353"); err != nil {
		t.Fatalf("start: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after start: %v", err)
	}
	if !status.Listening {
		t.Error("expected listening after start")
	}
	if status.ListenAddr != ":5353" {
		t.Errorf("expected listen addr :5353, got %q", status.ListenAddr)
	}
	if status.ZoneCount != 0 {
		t.Errorf("expected 0 zones, got %d", status.ZoneCount)
	}
	if status.QueriesTotal != 0 {
		t.Errorf("expected 0 queries, got %d", status.QueriesTotal)
	}

	// Reconcile and check status.
	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 3,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
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
	if status.ZoneCount != 1 {
		t.Errorf("expected 1 zone, got %d", status.ZoneCount)
	}
	if status.ZoneSerials["example.com."] != 3 {
		t.Errorf("expected serial 3, got %d", status.ZoneSerials["example.com."])
	}

	// Stop resets zones.
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after stop: %v", err)
	}
	if status.Listening {
		t.Error("expected not listening after stop")
	}
	if status.ZoneCount != 0 {
		t.Errorf("expected 0 zones after stop, got %d", status.ZoneCount)
	}
}
