package auth_test

import (
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestTokenIssueAndVerify(t *testing.T) {
	svc := auth.NewTokenService([]byte("test-signing-key-32-bytes-long!!"), time.Hour)

	tenantID := domain.NewID()
	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	}

	token, err := svc.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	got, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if got.UserID != claims.UserID {
		t.Errorf("user_id = %v, want %v", got.UserID, claims.UserID)
	}
	if got.TenantID == nil || *got.TenantID != tenantID {
		t.Errorf("tenant_id mismatch")
	}
	if got.Role != domain.RoleAdmin {
		t.Errorf("role = %v, want %v", got.Role, domain.RoleAdmin)
	}
}

func TestTokenSuperUserNoTenant(t *testing.T) {
	svc := auth.NewTokenService([]byte("test-signing-key-32-bytes-long!!"), time.Hour)

	claims := auth.Claims{
		UserID:   domain.NewID(),
		TenantID: nil,
		Role:     domain.RoleSuperUser,
	}

	token, err := svc.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if got.TenantID != nil {
		t.Errorf("superuser token should have nil tenant_id")
	}
}

func TestTokenExpired(t *testing.T) {
	// Use a negative TTL to produce an already-expired token.
	svc := auth.NewTokenService([]byte("test-signing-key-32-bytes-long!!"), -time.Hour)

	claims := auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleAdmin,
	}

	token, err := svc.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	_, err = svc.Verify(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestTokenTampered(t *testing.T) {
	svc := auth.NewTokenService([]byte("test-signing-key-32-bytes-long!!"), time.Hour)

	claims := auth.Claims{UserID: domain.NewID(), Role: domain.RoleAdmin}
	token, _ := svc.Issue(claims)

	// Tamper with the signature.
	tampered := token[:len(token)-4] + "XXXX"
	_, err := svc.Verify(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestTokenInvalidFormat(t *testing.T) {
	svc := auth.NewTokenService([]byte("test-signing-key-32-bytes-long!!"), time.Hour)

	_, err := svc.Verify("not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}

	_, err = svc.Verify("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}
