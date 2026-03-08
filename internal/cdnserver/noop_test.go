package cdnserver

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestNoopStartStop(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Start.
	if err := svc.Start(ctx, ":8080"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Double start should fail.
	if err := svc.Start(ctx, ":8080"); err == nil {
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

	if err := svc.Start(ctx, ":8080"); err != nil {
		t.Fatalf("start: %v", err)
	}

	site1ID := domain.NewID()
	site2ID := domain.NewID()

	// Reconcile with two sites.
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:       site1ID,
					Name:     "example-site",
					Domains:  []string{"example.com", "www.example.com"},
					TLSMode:  domain.TLSModeDisabled,
					Status:   domain.CDNSiteActive,
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:      domain.NewID(),
						SiteID:  site1ID,
						Address: "origin.example.com:443",
						Scheme:  domain.CDNOriginHTTPS,
						Weight:  10,
					},
				},
			},
			{
				Site: &domain.CDNSite{
					ID:      site2ID,
					Name:    "another-site",
					Domains: []string{"another.com"},
					TLSMode: domain.TLSModeDisabled,
					Status:  domain.CDNSiteActive,
				},
				Origins: nil,
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if svc.SiteCount() != 2 {
		t.Errorf("expected 2 sites, got %d", svc.SiteCount())
	}

	names := svc.SiteNames()
	if names[site1ID] != "example-site" {
		t.Errorf("expected name example-site for site1, got %q", names[site1ID])
	}
	if names[site2ID] != "another-site" {
		t.Errorf("expected name another-site for site2, got %q", names[site2ID])
	}

	// Reconcile with one site removed.
	config.Sites = config.Sites[:1]
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile reduced: %v", err)
	}

	if svc.SiteCount() != 1 {
		t.Errorf("expected 1 site after reduction, got %d", svc.SiteCount())
	}

	names = svc.SiteNames()
	if _, ok := names[site2ID]; ok {
		t.Error("site2 should have been removed")
	}

	// Reconcile with nil config should clear all sites.
	if err := svc.Reconcile(ctx, nil); err != nil {
		t.Fatalf("reconcile nil: %v", err)
	}
	if svc.SiteCount() != 0 {
		t.Errorf("expected 0 sites, got %d", svc.SiteCount())
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
	if err := svc.Start(ctx, ":8080"); err != nil {
		t.Fatalf("start: %v", err)
	}

	status, err = svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after start: %v", err)
	}
	if !status.Listening {
		t.Error("expected listening after start")
	}
	if status.ListenAddr != ":8080" {
		t.Errorf("expected listen addr :8080, got %q", status.ListenAddr)
	}
	if status.SiteCount != 0 {
		t.Errorf("expected 0 sites, got %d", status.SiteCount)
	}
	if status.RequestsTotal != 0 {
		t.Errorf("expected 0 requests, got %d", status.RequestsTotal)
	}

	// Reconcile and check status.
	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:      siteID,
					Name:    "test-site",
					Domains: []string{"test.com"},
					TLSMode: domain.TLSModeDisabled,
					Status:  domain.CDNSiteActive,
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
	if status.SiteCount != 1 {
		t.Errorf("expected 1 site, got %d", status.SiteCount)
	}

	// Stop resets sites.
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
	if status.SiteCount != 0 {
		t.Errorf("expected 0 sites after stop, got %d", status.SiteCount)
	}
}

func TestNoopPurgeCache(t *testing.T) {
	svc := NewNoopService()
	ctx := context.Background()

	// Purge without start should fail.
	if err := svc.PurgeCache(ctx, domain.NewID()); err == nil {
		t.Error("expected error purging before start")
	}

	if err := svc.Start(ctx, ":8080"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Purge unknown site should fail.
	if err := svc.PurgeCache(ctx, domain.NewID()); err == nil {
		t.Error("expected error purging unknown site")
	}

	// Reconcile with a site, then purge should succeed.
	siteID := domain.NewID()
	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:      siteID,
					Name:    "purge-test",
					Domains: []string{"purge.example.com"},
					TLSMode: domain.TLSModeDisabled,
					Status:  domain.CDNSiteActive,
				},
			},
		},
	}
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if err := svc.PurgeCache(ctx, siteID); err != nil {
		t.Fatalf("purge cache: %v", err)
	}
}
