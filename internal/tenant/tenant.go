package tenant

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// slugPattern enforces slug format: lowercase alphanumeric and hyphens only.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// DefaultService implements the tenant Service interface.
type DefaultService struct {
	store storage.TenantStore
}

// NewService creates a new tenant service.
func NewService(store storage.TenantStore) Service {
	return &DefaultService{store: store}
}

// Create validates inputs and creates a new tenant.
func (s *DefaultService) Create(ctx context.Context, req CreateRequest) (*domain.Tenant, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	if len(req.Slug) < 2 || !slugPattern.MatchString(req.Slug) {
		return nil, fmt.Errorf("slug must be lowercase alphanumeric with hyphens, at least 2 characters")
	}

	t := &domain.Tenant{
		ID:     domain.NewID(),
		Name:   req.Name,
		Slug:   req.Slug,
		Status: domain.TenantStatusActive,
	}

	if err := s.store.CreateTenant(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Get returns a tenant by ID.
func (s *DefaultService) Get(ctx context.Context, id domain.ID) (*domain.Tenant, error) {
	return s.store.GetTenant(ctx, id)
}

// GetBySlug returns a tenant by slug.
func (s *DefaultService) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	return s.store.GetTenantBySlug(ctx, slug)
}

// List returns a paginated list of tenants.
func (s *DefaultService) List(ctx context.Context, params storage.ListParams) ([]*domain.Tenant, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	return s.store.ListTenants(ctx, params)
}

// Update applies a partial update to a tenant.
func (s *DefaultService) Update(ctx context.Context, id domain.ID, req UpdateRequest) (*domain.Tenant, error) {
	t, err := s.store.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		t.Name = name
	}
	if req.Status != nil {
		t.Status = *req.Status
	}

	t.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTenant(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete performs a soft delete by setting status to deleted.
func (s *DefaultService) Delete(ctx context.Context, id domain.ID) error {
	t, err := s.store.GetTenant(ctx, id)
	if err != nil {
		return err
	}

	t.Status = domain.TenantStatusDeleted
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTenant(ctx, t)
}
