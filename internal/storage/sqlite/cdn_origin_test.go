package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func createTestCDNSite(t *testing.T, store interface {
	CreateCDNSite(ctx context.Context, s *domain.CDNSite) error
}, tenantID domain.ID) domain.ID {
	t.Helper()
	site := &domain.CDNSite{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-site-" + domain.NewID().String()[:8],
		Domains:  []string{"test-" + domain.NewID().String()[:8] + ".example.com"},
		TLSMode:  domain.TLSModeDisabled,
	}
	if err := store.CreateCDNSite(context.Background(), site); err != nil {
		t.Fatalf("create test cdn site: %v", err)
	}
	return site.ID
}

func TestCDNOriginCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	siteID := createTestCDNSite(t, store, tenantID)

	origin := &domain.CDNOrigin{
		ID:                  domain.NewID(),
		SiteID:              siteID,
		Address:             "origin.example.com:443",
		Scheme:              domain.CDNOriginHTTPS,
		Weight:              10,
		HealthCheckPath:     "/healthz",
		HealthCheckInterval: intPtr(30),
	}

	// Create.
	if err := store.CreateCDNOrigin(ctx, origin); err != nil {
		t.Fatalf("create cdn origin: %v", err)
	}
	if origin.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if origin.Status != domain.CDNOriginUnknown {
		t.Errorf("expected default status unknown, got %s", origin.Status)
	}

	// Get by ID.
	got, err := store.GetCDNOrigin(ctx, origin.ID)
	if err != nil {
		t.Fatalf("get cdn origin: %v", err)
	}
	if got.Address != "origin.example.com:443" {
		t.Errorf("expected address origin.example.com:443, got %s", got.Address)
	}
	if got.SiteID != siteID {
		t.Errorf("expected site_id %s, got %s", siteID, got.SiteID)
	}
	if got.Scheme != domain.CDNOriginHTTPS {
		t.Errorf("expected scheme https, got %s", got.Scheme)
	}
	if got.Weight != 10 {
		t.Errorf("expected weight 10, got %d", got.Weight)
	}
	if got.HealthCheckPath != "/healthz" {
		t.Errorf("expected health_check_path /healthz, got %s", got.HealthCheckPath)
	}
	if got.HealthCheckInterval == nil || *got.HealthCheckInterval != 30 {
		t.Errorf("expected health_check_interval 30, got %v", got.HealthCheckInterval)
	}

	// List by site.
	origins, total, err := store.ListCDNOrigins(ctx, siteID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list cdn origins: %v", err)
	}
	if total != 1 || len(origins) != 1 {
		t.Errorf("expected 1 origin, got total=%d len=%d", total, len(origins))
	}

	// Update — change address and weight.
	got.Address = "new-origin.example.com:8443"
	got.Weight = 20
	got.Status = domain.CDNOriginHealthy
	if err := store.UpdateCDNOrigin(ctx, got); err != nil {
		t.Fatalf("update cdn origin: %v", err)
	}

	updated, err := store.GetCDNOrigin(ctx, origin.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Address != "new-origin.example.com:8443" {
		t.Errorf("expected address new-origin.example.com:8443, got %s", updated.Address)
	}
	if updated.Weight != 20 {
		t.Errorf("expected weight 20, got %d", updated.Weight)
	}
	if updated.Status != domain.CDNOriginHealthy {
		t.Errorf("expected status healthy, got %s", updated.Status)
	}

	// Delete.
	if err := store.DeleteCDNOrigin(ctx, origin.ID); err != nil {
		t.Fatalf("delete cdn origin: %v", err)
	}
	_, err = store.GetCDNOrigin(ctx, origin.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCDNOriginNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetCDNOrigin(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCDNOriginDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteCDNOrigin(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCDNOriginDefaults(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	siteID := createTestCDNSite(t, store, tenantID)

	origin := &domain.CDNOrigin{
		ID:      domain.NewID(),
		SiteID:  siteID,
		Address: "minimal-origin.example.com",
	}
	if err := store.CreateCDNOrigin(ctx, origin); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetCDNOrigin(ctx, origin.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Scheme != domain.CDNOriginHTTPS {
		t.Errorf("expected default scheme https, got %s", got.Scheme)
	}
	if got.Weight != 1 {
		t.Errorf("expected default weight 1, got %d", got.Weight)
	}
	if got.Status != domain.CDNOriginUnknown {
		t.Errorf("expected default status unknown, got %s", got.Status)
	}
}

func intPtr(v int) *int {
	return &v
}
