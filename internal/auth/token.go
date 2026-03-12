package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// TokenService issues and verifies HMAC-SHA256 signed session tokens.
//
// Security assumptions:
// - HMAC-SHA256 is sufficient because the same process issues and verifies tokens.
// - No external parties need to verify tokens, so asymmetric signing is unnecessary.
// - The signing key is derived from the config's secrets.encryption_key.
// - Tokens are short-lived (configurable TTL, default 24h).
type TokenService struct {
	signingKey []byte
	ttl        time.Duration
}

// NewTokenService creates a token service with the given signing key and TTL.
func NewTokenService(signingKey []byte, ttl time.Duration) *TokenService {
	return &TokenService{
		signingKey: signingKey,
		ttl:        ttl,
	}
}

// Issue creates a signed token from the given claims.
func (ts *TokenService) Issue(claims Claims) (string, error) {
	now := time.Now().UTC()
	expires := now.Add(ts.ttl)

	// Encode payload: userID|tenantID|role|issuedAt|expiresAt|mfaPending
	tenantStr := ""
	if claims.TenantID != nil {
		tenantStr = claims.TenantID.String()
	}
	mfaStr := "0"
	if claims.MFAPending {
		mfaStr = "1"
	}
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%s",
		claims.UserID.String(),
		tenantStr,
		string(claims.Role),
		now.Unix(),
		expires.Unix(),
		mfaStr,
	)

	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := ts.sign([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return payloadB64 + "." + sigB64, nil
}

// Verify checks a token's signature and expiry, returning the claims.
func (ts *TokenService) Verify(token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	payloadB64, sigB64 := parts[0], parts[1]

	// Verify signature.
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("invalid token signature encoding")
	}

	expected := ts.sign([]byte(payloadB64))
	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Decode payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("invalid token payload encoding")
	}
	payload := string(payloadBytes)

	// Accept 5-field (legacy) or 6-field tokens.
	fields := strings.Split(payload, "|")
	if len(fields) != 5 && len(fields) != 6 {
		return nil, fmt.Errorf("invalid token payload")
	}

	userID, err := uuid.Parse(fields[0])
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token")
	}

	var tenantID *domain.ID
	if fields[1] != "" {
		tid, err := uuid.Parse(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid tenant ID in token")
		}
		tenantID = &tid
	}

	role := domain.Role(fields[2])

	var issuedAt, expiresAt int64
	if _, err := fmt.Sscanf(fields[3], "%d", &issuedAt); err != nil {
		return nil, fmt.Errorf("invalid issued time in token")
	}
	if _, err := fmt.Sscanf(fields[4], "%d", &expiresAt); err != nil {
		return nil, fmt.Errorf("invalid expiry time in token")
	}

	// Check expiry.
	if time.Now().Unix() > expiresAt {
		return nil, fmt.Errorf("token has expired")
	}

	// Parse MFA pending flag (6th field); legacy 5-field tokens default to false.
	mfaPending := false
	if len(fields) == 6 && fields[5] == "1" {
		mfaPending = true
	}

	return &Claims{
		UserID:     userID,
		TenantID:   tenantID,
		Role:       role,
		MFAPending: mfaPending,
	}, nil
}

// sign computes HMAC-SHA256 of the data.
func (ts *TokenService) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, ts.signingKey)
	mac.Write(data)
	return mac.Sum(nil)
}
