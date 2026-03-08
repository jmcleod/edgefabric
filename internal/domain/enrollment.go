package domain

import "time"

// EnrollmentTargetType identifies what is being enrolled.
type EnrollmentTargetType string

const (
	EnrollmentTargetNode    EnrollmentTargetType = "node"
	EnrollmentTargetGateway EnrollmentTargetType = "gateway"
)

// EnrollmentToken is a one-time signed token for node/gateway bootstrap.
type EnrollmentToken struct {
	ID         ID                   `json:"id" db:"id"`
	TenantID   ID                   `json:"tenant_id" db:"tenant_id"`
	TargetType EnrollmentTargetType `json:"target_type" db:"target_type"`
	TargetID   ID                   `json:"target_id" db:"target_id"`
	Token      string               `json:"token" db:"token"`
	ExpiresAt  time.Time            `json:"expires_at" db:"expires_at"`
	UsedAt     *time.Time           `json:"used_at,omitempty" db:"used_at"`
	CreatedAt  time.Time            `json:"created_at" db:"created_at"`
}
