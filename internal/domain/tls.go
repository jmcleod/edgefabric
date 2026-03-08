package domain

import "time"

// TLSCertificate stores a TLS cert/key pair.
type TLSCertificate struct {
	ID        ID        `json:"id" db:"id"`
	TenantID  ID        `json:"tenant_id" db:"tenant_id"`
	Domains   []string  `json:"domains" db:"-"`
	CertPEM   string    `json:"-" db:"cert_pem"`
	KeyPEM    string    `json:"-" db:"key_pem"`
	Issuer    string    `json:"issuer" db:"issuer"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	AutoRenew bool      `json:"auto_renew" db:"auto_renew"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
