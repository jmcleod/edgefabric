package cdn_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// TestCDNIntegration_FullLifecycle exercises the full CDN site and origin
// lifecycle through the service layer, using a real SQLite store.
func TestCDNIntegration_FullLifecycle(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// Step 1: Create tenant, node group, and node.
	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Step 2: Create CDN site with domains, assigned to group.
	rps := 100
	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:           tenantID,
		Name:               "full-lifecycle-site",
		Domains:            []string{"cdn.example.com", "www.example.com"},
		TLSMode:            domain.TLSModeAuto,
		CacheEnabled:       true,
		CacheTTL:           1800,
		CompressionEnabled: true,
		RateLimitRPS:       &rps,
		NodeGroupID:        &groupID,
		HeaderRules:        json.RawMessage(`[{"action":"set","header":"X-CDN","value":"edgefabric"}]`),
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	if site.Status != domain.CDNSiteActive {
		t.Errorf("expected active status, got %s", site.Status)
	}
	if len(site.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(site.Domains))
	}

	// Step 3: Create origins for the site.
	origin1, err := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:          site.ID,
		Address:         "backend1.example.com:443",
		Scheme:          domain.CDNOriginHTTPS,
		Weight:          10,
		HealthCheckPath: "/healthz",
	})
	if err != nil {
		t.Fatalf("create origin 1: %v", err)
	}

	origin2, err := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:  site.ID,
		Address: "backend2.example.com:443",
		Scheme:  domain.CDNOriginHTTPS,
		Weight:  5,
	})
	if err != nil {
		t.Fatalf("create origin 2: %v", err)
	}

	// Step 4: Get node CDN config — verify sites + origins returned.
	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 1 {
		t.Fatalf("expected 1 site in config, got %d", len(config.Sites))
	}
	swo := config.Sites[0]
	if swo.Site.Name != "full-lifecycle-site" {
		t.Errorf("expected site name full-lifecycle-site, got %s", swo.Site.Name)
	}
	if len(swo.Origins) != 2 {
		t.Errorf("expected 2 origins in config, got %d", len(swo.Origins))
	}
	if !swo.Site.CacheEnabled {
		t.Error("expected CacheEnabled=true")
	}
	if swo.Site.CacheTTL != 1800 {
		t.Errorf("expected CacheTTL=1800, got %d", swo.Site.CacheTTL)
	}
	if !swo.Site.CompressionEnabled {
		t.Error("expected CompressionEnabled=true")
	}
	if swo.Site.RateLimitRPS == nil || *swo.Site.RateLimitRPS != 100 {
		t.Errorf("expected RateLimitRPS=100, got %v", swo.Site.RateLimitRPS)
	}
	if swo.Site.HeaderRules == nil {
		t.Error("expected HeaderRules to be set")
	}

	// Step 5: Update site — change domains, toggle cache, update rate limit.
	newName := "updated-lifecycle-site"
	newTTL := 3600
	newRPS := 200
	cacheDisabled := false
	updated, err := svc.UpdateSite(ctx, site.ID, cdn.UpdateSiteRequest{
		Name:         &newName,
		Domains:      []string{"cdn.example.com", "new.example.com", "api.example.com"},
		CacheTTL:     &newTTL,
		CacheEnabled: &cacheDisabled,
		RateLimitRPS: &newRPS,
	})
	if err != nil {
		t.Fatalf("update site: %v", err)
	}
	if updated.Name != "updated-lifecycle-site" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	if len(updated.Domains) != 3 {
		t.Errorf("expected 3 domains after update, got %d", len(updated.Domains))
	}
	if updated.CacheEnabled {
		t.Error("expected CacheEnabled=false after update")
	}

	// Step 6: Delete origin → verify removed from config.
	if err := svc.DeleteOrigin(ctx, origin2.ID); err != nil {
		t.Fatalf("delete origin 2: %v", err)
	}

	config2, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config after origin delete: %v", err)
	}
	if len(config2.Sites[0].Origins) != 1 {
		t.Errorf("expected 1 origin after delete, got %d", len(config2.Sites[0].Origins))
	}
	if config2.Sites[0].Origins[0].ID != origin1.ID {
		t.Errorf("expected remaining origin to be origin1")
	}

	// Step 7: Purge cache — verify 204 (no error).
	if err := svc.PurgeSiteCache(ctx, site.ID); err != nil {
		t.Fatalf("purge cache: %v", err)
	}

	// Step 8: Delete site → verify cascade deletes origins.
	if err := svc.DeleteSite(ctx, site.ID); err != nil {
		t.Fatalf("delete site: %v", err)
	}

	// Verify site is gone.
	_, err = svc.GetSite(ctx, site.ID)
	if err == nil {
		t.Error("expected error getting deleted site")
	}

	// Verify origins were cascade-deleted.
	_, err = svc.GetOrigin(ctx, origin1.ID)
	if err == nil {
		t.Error("expected error getting origin after site cascade delete")
	}

	// Verify node config is empty.
	config3, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config after site delete: %v", err)
	}
	if len(config3.Sites) != 0 {
		t.Errorf("expected 0 sites after delete, got %d", len(config3.Sites))
	}
}

// TestCDNIntegration_ValidationEdgeCases tests various validation failures.
func TestCDNIntegration_ValidationEdgeCases(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	tests := []struct {
		name string
		req  cdn.CreateSiteRequest
	}{
		{
			"bad domain - has spaces",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-domain", Domains: []string{"has spaces.com"}, TLSMode: domain.TLSModeAuto},
		},
		{
			"invalid TLS mode",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-tls", Domains: []string{"a.com"}, TLSMode: "bogus"},
		},
		{
			"negative cache TTL",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-ttl", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, CacheTTL: -1},
		},
		{
			"zero rate limit",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-rps", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, RateLimitRPS: intPtr(0)},
		},
		{
			"negative rate limit",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-rps2", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, RateLimitRPS: intPtr(-10)},
		},
		{
			"bad header rules JSON",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-rules", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, HeaderRules: json.RawMessage(`not json`)},
		},
		{
			"header rule missing action",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-action", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, HeaderRules: json.RawMessage(`[{"action":"invalid","header":"X-Foo","value":"bar"}]`)},
		},
		{
			"header rule missing header name",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "bad-header", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto, HeaderRules: json.RawMessage(`[{"action":"set","header":"","value":"bar"}]`)},
		},
		{
			"empty name",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto},
		},
		{
			"no domains",
			cdn.CreateSiteRequest{TenantID: tenantID, Name: "no-domains", Domains: []string{}, TLSMode: domain.TLSModeAuto},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateSite(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestCDNIntegration_OriginValidation tests origin creation validation.
func TestCDNIntegration_OriginValidation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	site, _ := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "origin-val-test",
		Domains:  []string{"origin-val.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})

	tests := []struct {
		name string
		req  cdn.CreateOriginRequest
	}{
		{
			"empty address",
			cdn.CreateOriginRequest{SiteID: site.ID, Address: "", Scheme: domain.CDNOriginHTTPS},
		},
		{
			"invalid scheme",
			cdn.CreateOriginRequest{SiteID: site.ID, Address: "origin.com", Scheme: "ftp"},
		},
		{
			"bad health check path",
			cdn.CreateOriginRequest{SiteID: site.ID, Address: "origin.com", Scheme: domain.CDNOriginHTTPS, HealthCheckPath: "no-slash"},
		},
		{
			"health check interval too low",
			cdn.CreateOriginRequest{SiteID: site.ID, Address: "origin.com", Scheme: domain.CDNOriginHTTPS, HealthCheckInterval: intPtr(2)},
		},
		{
			"nonexistent site",
			cdn.CreateOriginRequest{SiteID: domain.NewID(), Address: "origin.com", Scheme: domain.CDNOriginHTTPS},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateOrigin(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestCDNIntegration_TenantIsolation verifies that sites from one tenant
// are not visible in the CDN config for another tenant's nodes.
func TestCDNIntegration_TenantIsolation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// Tenant A: create node, group, site.
	tenantA := createTestTenant(t, store)
	nodeA := createTestNode(t, store, tenantA)
	groupA := createTestNodeGroup(t, store, tenantA)
	if err := store.AddNodeToGroup(ctx, groupA, nodeA); err != nil {
		t.Fatalf("add node A to group: %v", err)
	}
	_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantA,
		Name:        "tenant-a-site",
		Domains:     []string{"a.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		NodeGroupID: &groupA,
	})
	if err != nil {
		t.Fatalf("create site A: %v", err)
	}

	// Tenant B: create node, group, site.
	tenantB := createTestTenant(t, store)
	nodeB := createTestNode(t, store, tenantB)
	groupB := createTestNodeGroup(t, store, tenantB)
	if err := store.AddNodeToGroup(ctx, groupB, nodeB); err != nil {
		t.Fatalf("add node B to group: %v", err)
	}
	_, err = svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantB,
		Name:        "tenant-b-site",
		Domains:     []string{"b.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		NodeGroupID: &groupB,
	})
	if err != nil {
		t.Fatalf("create site B: %v", err)
	}

	// Node A should only see tenant A's site.
	configA, err := svc.GetNodeCDNConfig(ctx, nodeA)
	if err != nil {
		t.Fatalf("get node A config: %v", err)
	}
	if len(configA.Sites) != 1 {
		t.Fatalf("expected 1 site for node A, got %d", len(configA.Sites))
	}
	if configA.Sites[0].Site.Name != "tenant-a-site" {
		t.Errorf("expected tenant-a-site, got %s", configA.Sites[0].Site.Name)
	}

	// Node B should only see tenant B's site.
	configB, err := svc.GetNodeCDNConfig(ctx, nodeB)
	if err != nil {
		t.Fatalf("get node B config: %v", err)
	}
	if len(configB.Sites) != 1 {
		t.Fatalf("expected 1 site for node B, got %d", len(configB.Sites))
	}
	if configB.Sites[0].Site.Name != "tenant-b-site" {
		t.Errorf("expected tenant-b-site, got %s", configB.Sites[0].Site.Name)
	}
}

// TestCDNIntegration_MultipleSitesPerNode verifies that a node in a group
// with multiple sites gets all of them in its config.
func TestCDNIntegration_MultipleSitesPerNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create three sites in the same group.
	for i := 0; i < 3; i++ {
		_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
			TenantID:    tenantID,
			Name:        "multi-site-" + domain.NewID().String()[:8],
			Domains:     []string{domain.NewID().String()[:8] + ".example.com"},
			TLSMode:     domain.TLSModeDisabled,
			NodeGroupID: &groupID,
		})
		if err != nil {
			t.Fatalf("create site %d: %v", i, err)
		}
	}

	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 3 {
		t.Errorf("expected 3 sites, got %d", len(config.Sites))
	}
}

// TestCDNIntegration_OriginDefaultWeight verifies that origins default weight to 1.
func TestCDNIntegration_OriginDefaultWeight(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	site, _ := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "weight-test",
		Domains:  []string{"weight.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})

	// Create origin without specifying weight.
	origin, err := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:  site.ID,
		Address: "origin.example.com",
		Scheme:  domain.CDNOriginHTTPS,
	})
	if err != nil {
		t.Fatalf("create origin: %v", err)
	}
	if origin.Weight != 1 {
		t.Errorf("expected default weight 1, got %d", origin.Weight)
	}
}

// TestCDNIntegration_UpdateOrigin exercises origin update operations.
func TestCDNIntegration_UpdateOrigin(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	site, _ := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "origin-update-test",
		Domains:  []string{"origin-update.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})

	origin, _ := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:  site.ID,
		Address: "old.example.com:443",
		Scheme:  domain.CDNOriginHTTPS,
		Weight:  5,
	})

	// Update address and weight.
	newAddr := "new.example.com:8443"
	newWeight := 20
	newScheme := domain.CDNOriginHTTP
	newStatus := domain.CDNOriginHealthy
	updated, err := svc.UpdateOrigin(ctx, origin.ID, cdn.UpdateOriginRequest{
		Address: &newAddr,
		Weight:  &newWeight,
		Scheme:  &newScheme,
		Status:  &newStatus,
	})
	if err != nil {
		t.Fatalf("update origin: %v", err)
	}
	if updated.Address != "new.example.com:8443" {
		t.Errorf("expected new address, got %s", updated.Address)
	}
	if updated.Weight != 20 {
		t.Errorf("expected weight 20, got %d", updated.Weight)
	}
	if updated.Scheme != domain.CDNOriginHTTP {
		t.Errorf("expected http scheme, got %s", updated.Scheme)
	}
	if updated.Status != domain.CDNOriginHealthy {
		t.Errorf("expected healthy status, got %s", updated.Status)
	}
}

// TestCDNIntegration_DeleteOriginNotFound verifies error on deleting nonexistent origin.
func TestCDNIntegration_DeleteOriginNotFound(t *testing.T) {
	svc, _ := newTestEnv(t)
	ctx := context.Background()

	if err := svc.DeleteOrigin(ctx, domain.NewID()); err == nil {
		t.Error("expected error deleting nonexistent origin")
	}
}

// TestCDNIntegration_DeleteSiteNotFound verifies error on deleting nonexistent site.
func TestCDNIntegration_DeleteSiteNotFound(t *testing.T) {
	svc, _ := newTestEnv(t)
	ctx := context.Background()

	if err := svc.DeleteSite(ctx, domain.NewID()); err == nil {
		t.Error("expected error deleting nonexistent site")
	}
}

// TestCDNIntegration_SiteWithoutNodeGroup verifies that sites without a
// node group assignment don't appear in any node's config.
func TestCDNIntegration_SiteWithoutNodeGroup(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create a site with no node group.
	_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "unassigned-site",
		Domains:  []string{"unassigned.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 0 {
		t.Errorf("expected 0 sites for unassigned site, got %d", len(config.Sites))
	}
}

// TestCDNIntegration_ClearNodeGroup verifies clearing a site's node group.
func TestCDNIntegration_ClearNodeGroup(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantID,
		Name:        "clear-group-test",
		Domains:     []string{"clear-group.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Verify site is in config.
	config, _ := svc.GetNodeCDNConfig(ctx, nodeID)
	if len(config.Sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(config.Sites))
	}

	// Clear node group.
	_, err = svc.UpdateSite(ctx, site.ID, cdn.UpdateSiteRequest{
		ClearNodeGroup: true,
	})
	if err != nil {
		t.Fatalf("clear node group: %v", err)
	}

	// Verify site is no longer in config.
	config2, _ := svc.GetNodeCDNConfig(ctx, nodeID)
	if len(config2.Sites) != 0 {
		t.Errorf("expected 0 sites after clearing group, got %d", len(config2.Sites))
	}
}

// TestCDNIntegration_ListPagination tests the list pagination.
func TestCDNIntegration_ListPagination(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	for i := 0; i < 5; i++ {
		_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
			TenantID: tenantID,
			Name:     "page-site-" + domain.NewID().String()[:8],
			Domains:  []string{domain.NewID().String()[:8] + ".example.com"},
			TLSMode:  domain.TLSModeDisabled,
		})
		if err != nil {
			t.Fatalf("create site %d: %v", i, err)
		}
	}

	// Page 1: limit 2.
	sites, total, err := svc.ListSites(ctx, tenantID, storage.ListParams{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list sites page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(sites) != 2 {
		t.Errorf("expected 2 sites on page 1, got %d", len(sites))
	}

	// Page 2: offset 2, limit 2.
	sites2, total2, err := svc.ListSites(ctx, tenantID, storage.ListParams{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list sites page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("expected total 5, got %d", total2)
	}
	if len(sites2) != 2 {
		t.Errorf("expected 2 sites on page 2, got %d", len(sites2))
	}

	// No overlap between pages.
	if sites[0].ID == sites2[0].ID {
		t.Error("pages should not overlap")
	}
}

func intPtr(v int) *int {
	return &v
}
