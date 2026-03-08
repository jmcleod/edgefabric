package domain

import "time"

// SSHKey stores SSH credentials used by the controller for provisioning.
type SSHKey struct {
	ID            ID        `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	PublicKey     string    `json:"public_key" db:"public_key"`
	PrivateKey    string    `json:"-" db:"private_key"` // encrypted at rest
	Fingerprint   string    `json:"fingerprint" db:"fingerprint"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	LastRotatedAt time.Time `json:"last_rotated_at" db:"last_rotated_at"`
}
