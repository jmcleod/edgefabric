package domain

import "time"

// Role defines the access level of a user.
type Role string

const (
	RoleSuperUser Role = "superuser"
	RoleAdmin     Role = "admin"
	RoleReadOnly  Role = "readonly"
)

// UserStatus represents whether a user account is active.
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User is a human operator with authentication credentials.
type User struct {
	ID           ID         `json:"id" db:"id"`
	TenantID     *ID        `json:"tenant_id,omitempty" db:"tenant_id"`
	Email        string     `json:"email" db:"email"`
	Name         string     `json:"name" db:"name"`
	PasswordHash string     `json:"-" db:"password_hash"`
	TOTPSecret   string     `json:"-" db:"totp_secret"`
	TOTPEnabled  bool       `json:"totp_enabled" db:"totp_enabled"`
	Role         Role       `json:"role" db:"role"`
	Status       UserStatus `json:"status" db:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}
