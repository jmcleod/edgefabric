package domain

import "time"

// APIKey is a programmatic access credential scoped to a tenant.
type APIKey struct {
	ID         ID         `json:"id" db:"id"`
	TenantID   ID         `json:"tenant_id" db:"tenant_id"`
	UserID     ID         `json:"user_id" db:"user_id"`
	Name       string     `json:"name" db:"name"`
	KeyHash    string     `json:"-" db:"key_hash"`
	KeyPrefix  string     `json:"key_prefix" db:"key_prefix"`
	Role       Role       `json:"role" db:"role"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}
