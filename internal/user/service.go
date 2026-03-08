// Package user manages user lifecycle and CRUD operations.
package user

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the user management interface.
type Service interface {
	Create(ctx context.Context, req CreateRequest) (*domain.User, error)
	Get(ctx context.Context, id domain.ID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.User, int, error)
	Update(ctx context.Context, id domain.ID, req UpdateRequest) (*domain.User, error)
	Delete(ctx context.Context, id domain.ID) error
}

// CreateRequest holds the input for creating a user.
type CreateRequest struct {
	TenantID *domain.ID  `json:"tenant_id,omitempty"`
	Email    string      `json:"email"`
	Name     string      `json:"name"`
	Password string      `json:"password"`
	Role     domain.Role `json:"role"`
}

// UpdateRequest holds the input for updating a user.
type UpdateRequest struct {
	Name     *string            `json:"name,omitempty"`
	Password *string            `json:"password,omitempty"`
	Role     *domain.Role       `json:"role,omitempty"`
	Status   *domain.UserStatus `json:"status,omitempty"`
}
