package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService implements Service using bcrypt, TOTP, and encrypted secrets.
type DefaultService struct {
	users      storage.UserStore
	apiKeys    storage.APIKeyStore
	secretStore *secrets.Store
	issuer     string // TOTP issuer name
}

// NewService creates a new authentication service.
func NewService(users storage.UserStore, apiKeys storage.APIKeyStore, secretStore *secrets.Store, issuer string) Service {
	return &DefaultService{
		users:      users,
		apiKeys:    apiKeys,
		secretStore: secretStore,
		issuer:     issuer,
	}
}

// AuthenticatePassword validates email/password and returns the user.
// Security: uses bcrypt.CompareHashAndPassword for constant-time comparison.
// FUTURE: Add rate limiting for AuthenticatePassword to mitigate brute-force attacks.
func (s *DefaultService) AuthenticatePassword(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		return nil, fmt.Errorf("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login time.
	now := time.Now().UTC()
	user.LastLoginAt = &now
	_ = s.users.UpdateUser(ctx, user) // best-effort

	return user, nil
}

// AuthenticateTOTP validates a TOTP code for a user.
func (s *DefaultService) AuthenticateTOTP(ctx context.Context, userID domain.ID, code string) error {
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !user.TOTPEnabled || user.TOTPSecret == "" {
		return fmt.Errorf("TOTP not enabled for this user")
	}

	// Decrypt the stored TOTP secret.
	secret, err := s.secretStore.Decrypt(user.TOTPSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}

	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}

	return nil
}

// AuthenticateAPIKey validates an API key and returns the associated record.
// Security: API keys use a prefix for lookup + bcrypt hash for verification.
// The prefix (first 8 chars) is stored in plaintext for efficient lookup.
// The full key is compared via bcrypt to prevent timing attacks.
func (s *DefaultService) AuthenticateAPIKey(ctx context.Context, rawKey string) (*domain.APIKey, error) {
	// Strip the "ef_" prefix added during generation.
	rawKey = strings.TrimPrefix(rawKey, "ef_")

	if len(rawKey) < 8 {
		return nil, fmt.Errorf("invalid API key format")
	}

	prefix := rawKey[:8]
	apiKey, err := s.apiKeys.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check expiry.
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Compare full key hash.
	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(rawKey)); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last used timestamp (best-effort).
	_ = s.apiKeys.UpdateAPIKeyLastUsed(ctx, apiKey.ID)

	return apiKey, nil
}

// HashPassword returns a bcrypt hash of the given password.
// Uses bcrypt cost 12 for a good balance of security and performance.
func (s *DefaultService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// EnrollTOTP generates a TOTP secret and returns the provisioning URI.
// The secret is encrypted and stored on the user record but TOTP is NOT
// enabled yet — call ConfirmTOTP after the user verifies with a code.
func (s *DefaultService) EnrollTOTP(ctx context.Context, userID domain.ID) (string, string, error) {
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("user not found")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: user.Email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate TOTP key: %w", err)
	}

	// Encrypt and store the secret (not yet enabled).
	encrypted, err := s.secretStore.Encrypt(key.Secret())
	if err != nil {
		return "", "", fmt.Errorf("encrypt TOTP secret: %w", err)
	}

	user.TOTPSecret = encrypted
	if err := s.users.UpdateUser(ctx, user); err != nil {
		return "", "", fmt.Errorf("save TOTP secret: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ConfirmTOTP activates TOTP after verifying a code against the enrolled secret.
func (s *DefaultService) ConfirmTOTP(ctx context.Context, userID domain.ID, code string) error {
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if user.TOTPSecret == "" {
		return fmt.Errorf("TOTP not enrolled — call EnrollTOTP first")
	}

	secret, err := s.secretStore.Decrypt(user.TOTPSecret)
	if err != nil {
		return fmt.Errorf("decrypt TOTP secret: %w", err)
	}

	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}

	user.TOTPEnabled = true
	if err := s.users.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("enable TOTP: %w", err)
	}

	return nil
}

// GenerateAPIKey creates a new API key. Returns the raw key (shown once to the user)
// and the persisted APIKey record.
// Security: raw key is 32 random bytes base64url-encoded (~43 chars).
// First 8 chars stored as prefix for lookup; full key stored as bcrypt hash.
func (s *DefaultService) GenerateAPIKey(ctx context.Context, tenantID, userID domain.ID, name string, role domain.Role) (string, *domain.APIKey, error) {
	// Generate 32 random bytes.
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", nil, fmt.Errorf("generate random key: %w", err)
	}
	rawKey := base64.RawURLEncoding.EncodeToString(keyBytes)
	prefix := rawKey[:8]

	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), 12)
	if err != nil {
		return "", nil, fmt.Errorf("hash API key: %w", err)
	}

	apiKey := &domain.APIKey{
		ID:        domain.NewID(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		KeyHash:   string(hash),
		KeyPrefix: prefix,
		Role:      role,
	}

	if err := s.apiKeys.CreateAPIKey(ctx, apiKey); err != nil {
		return "", nil, fmt.Errorf("create API key: %w", err)
	}

	return "ef_" + rawKey, apiKey, nil
}
