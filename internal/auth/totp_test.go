package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestTOTPEnrollConfirmVerify(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	// Create user.
	hash, _ := svc.HashPassword("password123")
	user := &domain.User{
		ID:           domain.NewID(),
		Email:        "totp@example.com",
		Name:         "TOTP User",
		PasswordHash: hash,
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusActive,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Step 1: Enroll TOTP — generates secret and provisioning URI.
	secret, uri, err := svc.EnrollTOTP(ctx, user.ID)
	if err != nil {
		t.Fatalf("enroll TOTP: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty TOTP secret")
	}
	if uri == "" {
		t.Fatal("expected non-empty provisioning URI")
	}

	// Verify user has encrypted secret stored but TOTP not yet enabled.
	u, _ := store.GetUser(ctx, user.ID)
	if u.TOTPEnabled {
		t.Error("TOTP should NOT be enabled before confirmation")
	}
	if u.TOTPSecret == "" {
		t.Error("TOTP secret should be stored (encrypted)")
	}

	// Step 2: Confirm TOTP with a valid code.
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}

	if err := svc.ConfirmTOTP(ctx, user.ID, code); err != nil {
		t.Fatalf("confirm TOTP: %v", err)
	}

	// Verify TOTP is now enabled.
	u2, _ := store.GetUser(ctx, user.ID)
	if !u2.TOTPEnabled {
		t.Error("TOTP should be enabled after confirmation")
	}

	// Step 3: Authenticate with TOTP.
	validCode, _ := totp.GenerateCode(secret, time.Now())
	if err := svc.AuthenticateTOTP(ctx, user.ID, validCode); err != nil {
		t.Fatalf("authenticate TOTP: %v", err)
	}

	// Invalid code should fail.
	if err := svc.AuthenticateTOTP(ctx, user.ID, "000000"); err == nil {
		t.Error("expected error for invalid TOTP code")
	}
}

func TestTOTPConfirmWithBadCode(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	hash, _ := svc.HashPassword("password123")
	user := &domain.User{
		ID:           domain.NewID(),
		Email:        "badtotp@example.com",
		Name:         "Bad TOTP",
		PasswordHash: hash,
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusActive,
	}
	store.CreateUser(ctx, user)
	svc.EnrollTOTP(ctx, user.ID)

	// Confirm with wrong code should fail.
	err := svc.ConfirmTOTP(ctx, user.ID, "000000")
	if err == nil {
		t.Error("expected error for bad TOTP code during confirmation")
	}

	// Verify TOTP is still NOT enabled.
	u, _ := store.GetUser(ctx, user.ID)
	if u.TOTPEnabled {
		t.Error("TOTP should NOT be enabled after failed confirmation")
	}
}

func TestTOTPConfirmWithoutEnrollment(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	hash, _ := svc.HashPassword("password123")
	user := &domain.User{
		ID:           domain.NewID(),
		Email:        "nototp@example.com",
		Name:         "No TOTP",
		PasswordHash: hash,
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusActive,
	}
	store.CreateUser(ctx, user)

	// Confirm without enrollment should fail.
	err := svc.ConfirmTOTP(ctx, user.ID, "123456")
	if err == nil {
		t.Error("expected error when confirming TOTP without enrollment")
	}
}
