package user

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService implements the user Service interface.
type DefaultService struct {
	store   storage.UserStore
	authSvc auth.Service
}

// NewService creates a new user service.
func NewService(store storage.UserStore, authSvc auth.Service) Service {
	return &DefaultService{store: store, authSvc: authSvc}
}

// Create validates inputs, hashes the password, and creates a new user.
func (s *DefaultService) Create(ctx context.Context, req CreateRequest) (*domain.User, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email address")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	if req.Role == "" {
		return nil, fmt.Errorf("role is required")
	}

	// SuperUsers must not have a tenant; others must.
	if req.Role == domain.RoleSuperUser && req.TenantID != nil {
		return nil, fmt.Errorf("superusers cannot be assigned to a tenant")
	}
	if req.Role != domain.RoleSuperUser && req.TenantID == nil {
		return nil, fmt.Errorf("non-superuser users must be assigned to a tenant")
	}

	hash, err := s.authSvc.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &domain.User{
		ID:           domain.NewID(),
		TenantID:     req.TenantID,
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
		Role:         req.Role,
		Status:       domain.UserStatusActive,
	}

	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Get returns a user by ID.
func (s *DefaultService) Get(ctx context.Context, id domain.ID) (*domain.User, error) {
	return s.store.GetUser(ctx, id)
}

// GetByEmail returns a user by email.
func (s *DefaultService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.store.GetUserByEmail(ctx, email)
}

// List returns a paginated list of users, optionally filtered by tenant.
func (s *DefaultService) List(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.User, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	return s.store.ListUsers(ctx, tenantID, params)
}

// Update applies a partial update to a user.
func (s *DefaultService) Update(ctx context.Context, id domain.ID, req UpdateRequest) (*domain.User, error) {
	u, err := s.store.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		u.Name = name
	}
	if req.Password != nil {
		if len(*req.Password) < 8 {
			return nil, fmt.Errorf("password must be at least 8 characters")
		}
		hash, err := s.authSvc.HashPassword(*req.Password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		u.PasswordHash = hash
	}
	if req.Role != nil {
		u.Role = *req.Role
	}
	if req.Status != nil {
		u.Status = *req.Status
	}

	u.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Delete removes a user.
func (s *DefaultService) Delete(ctx context.Context, id domain.ID) error {
	return s.store.DeleteUser(ctx, id)
}
