package cdn_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

func newTestEnv(t *testing.T) (cdn.Service, *sqlite.SQLiteStore) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := cdn.NewService(store, store, store, store)
	return svc, store
}

func createTestTenant(t *testing.T, store *sqlite.SQLiteStore) domain.ID {
	t.Helper()
	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "test-tenant-" + domain.NewID().String()[:8],
		Slug: "test-" + domain.NewID().String()[:8],
	}
	if err := store.CreateTenant(context.Background(), tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant.ID
}

func createTestNodeGroup(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-group-" + domain.NewID().String()[:8],
	}
	if err := store.CreateNodeGroup(context.Background(), group); err != nil {
		t.Fatalf("create node group: %v", err)
	}
	return group.ID
}

func createTestNode(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: &tenantID,
		Name:     "test-node-" + domain.NewID().String()[:8],
		Hostname: "node.example.com",
		PublicIP: "203.0.113.1",
		SSHPort:  22,
		SSHUser:  "root",
		Status:   domain.NodeStatusOnline,
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.ID
}

func TestCreateSite_HappyPath(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:           tenantID,
		Name:               "my-site",
		Domains:            []string{"cdn.example.com", "www.example.com"},
		TLSMode:            domain.TLSModeAuto,
		CacheEnabled:       true,
		CacheTTL:           3600,
		CompressionEnabled: true,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if site.Name != "my-site" {
		t.Errorf("expected name my-site, got %s", site.Name)
	}
	if site.Status != domain.CDNSiteActive {
		t.Errorf("expected active status, got %s", site.Status)
	}
	if len(site.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(site.Domains))
	}

	// Verify via Get.
	got, err := svc.GetSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if got.Name != "my-site" {
		t.Errorf("expected name my-site, got %s", got.Name)
	}
	if len(got.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(got.Domains))
	}
}

func TestCreateSite_Validation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	tests := []struct {
		name string
		req  cdn.CreateSiteRequest
	}{
		{"empty name", cdn.CreateSiteRequest{TenantID: tenantID, Name: "", Domains: []string{"a.com"}, TLSMode: domain.TLSModeAuto}},
		{"no domains", cdn.CreateSiteRequest{TenantID: tenantID, Name: "site", Domains: []string{}, TLSMode: domain.TLSModeAuto}},
		{"invalid domain", cdn.CreateSiteRequest{TenantID: tenantID, Name: "site", Domains: []string{"not valid!"}, TLSMode: domain.TLSModeAuto}},
		{"invalid TLS mode", cdn.CreateSiteRequest{TenantID: tenantID, Name: "site", Domains: []string{"a.com"}, TLSMode: "invalid"}},
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

func TestCreateOrigin_HappyPath(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "origin-test-site",
		Domains:  []string{"origin.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	origin, err := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:          site.ID,
		Address:         "backend.example.com:443",
		Scheme:          domain.CDNOriginHTTPS,
		Weight:          10,
		HealthCheckPath: "/healthz",
	})
	if err != nil {
		t.Fatalf("create origin: %v", err)
	}
	if origin.Address != "backend.example.com:443" {
		t.Errorf("expected address backend.example.com:443, got %s", origin.Address)
	}
	if origin.Weight != 10 {
		t.Errorf("expected weight 10, got %d", origin.Weight)
	}

	// Verify via Get.
	got, err := svc.GetOrigin(ctx, origin.ID)
	if err != nil {
		t.Fatalf("get origin: %v", err)
	}
	if got.Address != "backend.example.com:443" {
		t.Errorf("expected address, got %s", got.Address)
	}
}

func TestCreateOrigin_Validation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	site, _ := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "validation-site",
		Domains:  []string{"val.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})

	tests := []struct {
		name string
		req  cdn.CreateOriginRequest
	}{
		{"empty address", cdn.CreateOriginRequest{SiteID: site.ID, Address: "", Scheme: domain.CDNOriginHTTPS}},
		{"invalid scheme", cdn.CreateOriginRequest{SiteID: site.ID, Address: "origin.com", Scheme: "ftp"}},
		{"nonexistent site", cdn.CreateOriginRequest{SiteID: domain.NewID(), Address: "origin.com", Scheme: domain.CDNOriginHTTPS}},
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

func TestGetNodeCDNConfig(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	// Add node to group.
	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create a site assigned to the group.
	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:     tenantID,
		Name:         "config-test-site",
		Domains:      []string{"cdn.test.com"},
		TLSMode:      domain.TLSModeAuto,
		CacheEnabled: true,
		CacheTTL:     1800,
		NodeGroupID:  &groupID,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Create origins.
	_, err = svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:  site.ID,
		Address: "backend1.test.com:443",
		Scheme:  domain.CDNOriginHTTPS,
		Weight:  10,
	})
	if err != nil {
		t.Fatalf("create origin 1: %v", err)
	}
	_, err = svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
		SiteID:  site.ID,
		Address: "backend2.test.com:443",
		Scheme:  domain.CDNOriginHTTPS,
		Weight:  5,
	})
	if err != nil {
		t.Fatalf("create origin 2: %v", err)
	}

	// Get node CDN config.
	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(config.Sites))
	}
	if config.Sites[0].Site.Name != "config-test-site" {
		t.Errorf("expected site config-test-site, got %s", config.Sites[0].Site.Name)
	}
	if len(config.Sites[0].Origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(config.Sites[0].Origins))
	}
}

func TestGetNodeCDNConfig_NoSitesForUnassignedNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	// Create a site assigned to the group, but don't add the node.
	_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantID,
		Name:        "unassigned-site",
		Domains:     []string{"unassigned.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 0 {
		t.Errorf("expected 0 sites for unassigned node, got %d", len(config.Sites))
	}
}

func TestGetNodeCDNConfig_DisabledSiteExcluded(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create site and disable it.
	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantID,
		Name:        "disabled-site",
		Domains:     []string{"disabled.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	disabled := domain.CDNSiteDisabled
	_, err = svc.UpdateSite(ctx, site.ID, cdn.UpdateSiteRequest{Status: &disabled})
	if err != nil {
		t.Fatalf("disable site: %v", err)
	}

	config, err := svc.GetNodeCDNConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node cdn config: %v", err)
	}
	if len(config.Sites) != 0 {
		t.Errorf("expected 0 sites (disabled excluded), got %d", len(config.Sites))
	}
}

func TestUpdateSite(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:     tenantID,
		Name:         "update-test",
		Domains:      []string{"old.example.com"},
		TLSMode:      domain.TLSModeAuto,
		CacheEnabled: true,
		CacheTTL:     3600,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Update name and domains.
	newName := "updated-name"
	updated, err := svc.UpdateSite(ctx, site.ID, cdn.UpdateSiteRequest{
		Name:    &newName,
		Domains: []string{"new1.example.com", "new2.example.com"},
	})
	if err != nil {
		t.Fatalf("update site: %v", err)
	}
	if updated.Name != "updated-name" {
		t.Errorf("expected name updated-name, got %s", updated.Name)
	}
	if len(updated.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(updated.Domains))
	}
}

func TestPurgeSiteCache(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "purge-test",
		Domains:  []string{"purge.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	// Purge should succeed.
	if err := svc.PurgeSiteCache(ctx, site.ID); err != nil {
		t.Errorf("expected purge to succeed, got: %v", err)
	}

	// Purge on nonexistent site should fail.
	if err := svc.PurgeSiteCache(ctx, domain.NewID()); err == nil {
		t.Error("expected error for nonexistent site")
	}
}

func TestListSites(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	for i := 0; i < 3; i++ {
		_, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
			TenantID: tenantID,
			Name:     "site-" + domain.NewID().String()[:8],
			Domains:  []string{domain.NewID().String()[:8] + ".example.com"},
			TLSMode:  domain.TLSModeDisabled,
		})
		if err != nil {
			t.Fatalf("create site %d: %v", i, err)
		}
	}

	sites, total, err := svc.ListSites(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if total != 3 || len(sites) != 3 {
		t.Errorf("expected 3 sites, got total=%d len=%d", total, len(sites))
	}
}

func TestListOrigins(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	site, _ := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID: tenantID,
		Name:     "origins-test",
		Domains:  []string{"origins.example.com"},
		TLSMode:  domain.TLSModeDisabled,
	})

	for _, addr := range []string{"a.com", "b.com", "c.com"} {
		_, err := svc.CreateOrigin(ctx, cdn.CreateOriginRequest{
			SiteID:  site.ID,
			Address: addr,
			Scheme:  domain.CDNOriginHTTPS,
		})
		if err != nil {
			t.Fatalf("create origin %s: %v", addr, err)
		}
	}

	origins, total, err := svc.ListOrigins(ctx, site.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list origins: %v", err)
	}
	if total != 3 || len(origins) != 3 {
		t.Errorf("expected 3 origins, got total=%d len=%d", total, len(origins))
	}
}

func TestCreateSite_WithHeaderRules(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	site, err := svc.CreateSite(ctx, cdn.CreateSiteRequest{
		TenantID:    tenantID,
		Name:        "headers-test",
		Domains:     []string{"headers.example.com"},
		TLSMode:     domain.TLSModeDisabled,
		HeaderRules: json.RawMessage(`[{"action":"set","header":"X-CDN","value":"edgefabric"},{"action":"remove","header":"Server"}]`),
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if site.HeaderRules == nil {
		t.Error("expected header_rules to be set")
	}

	// Get and verify.
	got, err := svc.GetSite(ctx, site.ID)
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if got.HeaderRules == nil {
		t.Error("expected header_rules to persist")
	}
}
