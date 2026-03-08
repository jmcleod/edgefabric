// Package tenant manages tenant lifecycle and isolation.
package tenant

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the tenant management interface.
type Service interface {
	Create(ctx context.Context, req CreateRequest) (*domain.Tenant, error)
	Get(ctx context.Context, id domain.ID) (*domain.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	List(ctx context.Context, params storage.ListParams) ([]*domain.Tenant, int, error)
	Update(ctx context.Context, id domain.ID, req UpdateRequest) (*domain.Tenant, error)
	Delete(ctx context.Context, id domain.ID) error
}

// CreateRequest holds the input for creating a tenant.
type CreateRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// UpdateRequest holds the input for updating a tenant.
type UpdateRequest struct {
	Name   *string              `json:"name,omitempty"`
	Status *domain.TenantStatus `json:"status,omitempty"`
}
