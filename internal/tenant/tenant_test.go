package tenant_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
	"github.com/jmcleod/edgefabric/internal/tenant"
)

func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestTenantServiceCreate(t *testing.T) {
	store := newTestStore(t)
	svc := tenant.NewService(store)

	got, err := svc.Create(context.Background(), tenant.CreateRequest{
		Name: "Acme Corp",
		Slug: "acme-corp",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("name = %q, want %q", got.Name, "Acme Corp")
	}
	if got.Slug != "acme-corp" {
		t.Errorf("slug = %q, want %q", got.Slug, "acme-corp")
	}
	if got.Status != domain.TenantStatusActive {
		t.Errorf("status = %q, want %q", got.Status, domain.TenantStatusActive)
	}
}

func TestTenantServiceCreateValidation(t *testing.T) {
	store := newTestStore(t)
	svc := tenant.NewService(store)
	ctx := context.Background()

	tests := []struct {
		name string
		req  tenant.CreateRequest
	}{
		{"empty name", tenant.CreateRequest{Name: "", Slug: "valid"}},
		{"empty slug", tenant.CreateRequest{Name: "Valid", Slug: ""}},
		{"invalid slug (special chars)", tenant.CreateRequest{Name: "Valid", Slug: "has_underscore"}},
		{"invalid slug (single char)", tenant.CreateRequest{Name: "Valid", Slug: "a"}},
		{"invalid slug (spaces)", tenant.CreateRequest{Name: "Valid", Slug: "has spaces"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tt.req)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestTenantServiceSoftDelete(t *testing.T) {
	store := newTestStore(t)
	svc := tenant.NewService(store)
	ctx := context.Background()

	created, err := svc.Create(ctx, tenant.CreateRequest{Name: "DeleteMe", Slug: "delete-me"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Delete (soft delete).
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// The tenant should still exist but with status=deleted.
	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got.Status != domain.TenantStatusDeleted {
		t.Errorf("status = %q, want %q", got.Status, domain.TenantStatusDeleted)
	}
}

func TestTenantServiceSlugTrimming(t *testing.T) {
	store := newTestStore(t)
	svc := tenant.NewService(store)

	got, err := svc.Create(context.Background(), tenant.CreateRequest{
		Name: "  Trimmed  ",
		Slug: "  TRIMMED-SLUG  ",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Name != "Trimmed" {
		t.Errorf("name = %q, want %q", got.Name, "Trimmed")
	}
	if got.Slug != "trimmed-slug" {
		t.Errorf("slug = %q, want %q", got.Slug, "trimmed-slug")
	}
}
