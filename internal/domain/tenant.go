package domain

import (
	"encoding/json"
	"time"
)

// TenantStatus represents the lifecycle state of a tenant.
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// Tenant is the multi-tenant isolation boundary.
type Tenant struct {
	ID        ID              `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	Slug      string          `json:"slug" db:"slug"`
	Status    TenantStatus    `json:"status" db:"status"`
	Settings  json.RawMessage `json:"settings,omitempty" db:"settings"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
