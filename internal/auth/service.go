// Package auth handles authentication: password, TOTP 2FA, and API keys.
package auth

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// Service defines the authentication interface.
type Service interface {
	// AuthenticatePassword validates email/password and returns the user.
	AuthenticatePassword(ctx context.Context, email, password string) (*domain.User, error)

	// AuthenticateTOTP validates a TOTP code for a user.
	AuthenticateTOTP(ctx context.Context, userID domain.ID, code string) error

	// AuthenticateAPIKey validates an API key string and returns the associated API key record.
	AuthenticateAPIKey(ctx context.Context, key string) (*domain.APIKey, error)

	// HashPassword returns a bcrypt hash of the given password.
	HashPassword(password string) (string, error)

	// EnrollTOTP generates a TOTP secret for a user and returns the provisioning URI.
	EnrollTOTP(ctx context.Context, userID domain.ID) (secret string, provisioningURI string, err error)

	// ConfirmTOTP activates TOTP for a user after verifying a code.
	ConfirmTOTP(ctx context.Context, userID domain.ID, code string) error

	// GenerateAPIKey creates a new API key and returns the raw key (shown once).
	GenerateAPIKey(ctx context.Context, tenantID, userID domain.ID, name string, role domain.Role) (rawKey string, apiKey *domain.APIKey, err error)
}

// Claims represents the authenticated identity extracted from a request.
type Claims struct {
	UserID   domain.ID
	TenantID *domain.ID
	Role     domain.Role
	// APIKeyID is set when authentication was via API key.
	APIKeyID *domain.ID
	// MFAPending is true when the user has passed password auth but not yet
	// completed TOTP verification. Tokens with MFAPending=true must be
	// restricted to MFA-related endpoints only.
	MFAPending bool
}
