package domain

import (
	"encoding/json"
	"time"
)

// GatewayStatus represents the lifecycle state of a gateway.
type GatewayStatus string

const (
	GatewayStatusPending   GatewayStatus = "pending"
	GatewayStatusEnrolling GatewayStatus = "enrolling"
	GatewayStatusOnline    GatewayStatus = "online"
	GatewayStatusOffline   GatewayStatus = "offline"
	GatewayStatusError     GatewayStatus = "error"
)

// Gateway connects EdgeFabric to a private network.
type Gateway struct {
	ID              ID              `json:"id" db:"id"`
	TenantID        ID              `json:"tenant_id" db:"tenant_id"`
	Name            string          `json:"name" db:"name"`
	PublicIP        string          `json:"public_ip,omitempty" db:"public_ip"`
	WireGuardIP     string          `json:"wireguard_ip" db:"wireguard_ip"`
	Status          GatewayStatus   `json:"status" db:"status"`
	EnrollmentToken string          `json:"-" db:"enrollment_token"`
	LastHeartbeat   *time.Time      `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	Metadata        json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}
