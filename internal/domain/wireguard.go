package domain

import "time"

// PeerOwnerType identifies what type of entity owns a WireGuard peer.
type PeerOwnerType string

const (
	PeerOwnerNode       PeerOwnerType = "node"
	PeerOwnerGateway    PeerOwnerType = "gateway"
	PeerOwnerController PeerOwnerType = "controller"
)

// WireGuardPeer holds the WireGuard configuration for a peer.
type WireGuardPeer struct {
	ID            ID            `json:"id" db:"id"`
	OwnerType     PeerOwnerType `json:"owner_type" db:"owner_type"`
	OwnerID       ID            `json:"owner_id" db:"owner_id"`
	PublicKey     string        `json:"public_key" db:"public_key"`
	PrivateKey    string        `json:"-" db:"private_key"` // encrypted at rest
	PresharedKey  string        `json:"-" db:"preshared_key"`
	AllowedIPs    []string      `json:"allowed_ips" db:"-"`
	Endpoint      string        `json:"endpoint,omitempty" db:"endpoint"`
	LastRotatedAt time.Time     `json:"last_rotated_at" db:"last_rotated_at"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at" db:"updated_at"`
}
