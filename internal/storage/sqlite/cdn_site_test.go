package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestCDNSiteCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site := &domain.CDNSite{
		ID:                 domain.NewID(),
		TenantID:           tenantID,
		Name:               "my-cdn-site",
		Domains:            []string{"cdn.example.com", "www.example.com"},
		TLSMode:            domain.TLSModeAuto,
		CacheEnabled:       true,
		CacheTTL:           3600,
		CompressionEnabled: true,
		HeaderRules:        json.RawMessage(`[{"action":"set","header":"X-CDN","value":"edgefabric"}]`),
	}

	// Create.
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create cdn site: %v", err)
	}
	if site.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if site.Status != domain.CDNSiteActive {
		t.Errorf("expected default status active, got %s", site.Status)
	}

	// Get by ID.
	got, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get cdn site: %v", err)
	}
	if got.Name != "my-cdn-site" {
		t.Errorf("expected name my-cdn-site, got %s", got.Name)
	}
	if got.TenantID != tenantID {
		t.Errorf("expected tenant_id %s, got %s", tenantID, got.TenantID)
	}
	if len(got.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(got.Domains))
	}
	// Domains sorted alphabetically.
	if got.Domains[0] != "cdn.example.com" {
		t.Errorf("expected first domain cdn.example.com, got %s", got.Domains[0])
	}
	if got.Domains[1] != "www.example.com" {
		t.Errorf("expected second domain www.example.com, got %s", got.Domains[1])
	}
	if got.TLSMode != domain.TLSModeAuto {
		t.Errorf("expected TLS mode auto, got %s", got.TLSMode)
	}
	if !got.CacheEnabled {
		t.Error("expected cache_enabled true")
	}
	if got.CacheTTL != 3600 {
		t.Errorf("expected cache_ttl 3600, got %d", got.CacheTTL)
	}
	if !got.CompressionEnabled {
		t.Error("expected compression_enabled true")
	}
	if got.HeaderRules == nil {
		t.Error("expected header_rules to be set")
	}

	// List by tenant.
	sites, total, err := store.ListCDNSites(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list cdn sites: %v", err)
	}
	if total != 1 || len(sites) != 1 {
		t.Errorf("expected 1 site, got total=%d len=%d", total, len(sites))
	}
	if len(sites[0].Domains) != 2 {
		t.Errorf("expected listed site to have 2 domains, got %d", len(sites[0].Domains))
	}

	// Update — change name, domains, cache TTL.
	got.Name = "updated-cdn-site"
	got.Domains = []string{"new.example.com"}
	got.CacheTTL = 7200
	if err := store.UpdateCDNSite(ctx, got); err != nil {
		t.Fatalf("update cdn site: %v", err)
	}

	updated, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Name != "updated-cdn-site" {
		t.Errorf("expected name updated-cdn-site, got %s", updated.Name)
	}
	if updated.CacheTTL != 7200 {
		t.Errorf("expected cache_ttl 7200, got %d", updated.CacheTTL)
	}
	if len(updated.Domains) != 1 {
		t.Fatalf("expected 1 domain after update, got %d", len(updated.Domains))
	}
	if updated.Domains[0] != "new.example.com" {
		t.Errorf("expected domain new.example.com, got %s", updated.Domains[0])
	}

	// Delete.
	if err := store.DeleteCDNSite(ctx, site.ID); err != nil {
		t.Fatalf("delete cdn site: %v", err)
	}
	_, err = store.GetCDNSite(ctx, site.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCDNSiteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetCDNSite(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCDNSiteDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteCDNSite(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCDNSiteDomains(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	// Site with no domains.
	site := &domain.CDNSite{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "no-domains-site",
		TLSMode:  domain.TLSModeDisabled,
	}
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(got.Domains))
	}

	// Update to add domains.
	got.Domains = []string{"a.example.com", "b.example.com", "c.example.com"}
	if err := store.UpdateCDNSite(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if len(updated.Domains) != 3 {
		t.Errorf("expected 3 domains, got %d", len(updated.Domains))
	}
}

func TestCDNSiteWithNodeGroup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	groupID := createTestNodeGroup(t, store, tenantID)

	site := &domain.CDNSite{
		ID:          domain.NewID(),
		TenantID:    tenantID,
		Name:        "grouped-cdn-site",
		Domains:     []string{"cdn.test.com"},
		TLSMode:     domain.TLSModeAuto,
		NodeGroupID: &groupID,
	}
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.NodeGroupID == nil {
		t.Fatal("expected node_group_id to be set")
	}
	if *got.NodeGroupID != groupID {
		t.Errorf("expected node_group_id %s, got %s", groupID, *got.NodeGroupID)
	}
}

func TestCDNSiteRateLimitRPS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	rps := 100
	site := &domain.CDNSite{
		ID:           domain.NewID(),
		TenantID:     tenantID,
		Name:         "ratelimit-site",
		Domains:      []string{"limited.example.com"},
		TLSMode:      domain.TLSModeDisabled,
		RateLimitRPS: &rps,
	}
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetCDNSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RateLimitRPS == nil || *got.RateLimitRPS != 100 {
		t.Errorf("expected rate_limit_rps 100, got %v", got.RateLimitRPS)
	}
}

func TestDeleteCDNSiteCascadesOrigins(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site := &domain.CDNSite{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "cascade-test-site",
		Domains:  []string{"cascade.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	}
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create site: %v", err)
	}

	origin := &domain.CDNOrigin{
		ID:      domain.NewID(),
		SiteID:  site.ID,
		Address: "origin.example.com:443",
		Scheme:  domain.CDNOriginHTTPS,
	}
	if err := store.CreateCDNOrigin(ctx, origin); err != nil {
		t.Fatalf("create origin: %v", err)
	}

	// Verify origin exists.
	_, err := store.GetCDNOrigin(ctx, origin.ID)
	if err != nil {
		t.Fatalf("get origin before delete: %v", err)
	}

	// Delete site — should cascade to origins.
	if err := store.DeleteCDNSite(ctx, site.ID); err != nil {
		t.Fatalf("delete site: %v", err)
	}

	// Origin should be gone.
	_, err = store.GetCDNOrigin(ctx, origin.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected origin ErrNotFound after site delete, got %v", err)
	}
}

func TestCDNSiteTenantIsolation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenant1 := createTestTenant(t, store)
	tenant2 := createTestTenant(t, store)

	site := &domain.CDNSite{
		ID:       domain.NewID(),
		TenantID: tenant1,
		Name:     "isolated-site",
		Domains:  []string{"isolated.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	}
	if err := store.CreateCDNSite(ctx, site); err != nil {
		t.Fatalf("create: %v", err)
	}

	// List for tenant2 should return 0.
	sites, total, err := store.ListCDNSites(ctx, tenant2, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(sites) != 0 {
		t.Errorf("expected 0 sites for tenant2, got total=%d len=%d", total, len(sites))
	}
}
