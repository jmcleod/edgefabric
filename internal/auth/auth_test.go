package auth_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

// testKey is a valid base64-encoded 32-byte key for testing.
// echo -n '0123456789abcdef0123456789abcdef' | base64
const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func newTestAuthService(t *testing.T) (auth.Service, *sqlite.SQLiteStore) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	secretStore, err := secrets.NewStore(testKey)
	if err != nil {
		t.Fatalf("create secret store: %v", err)
	}

	svc := auth.NewService(store, store, secretStore, "test-issuer")
	return svc, store
}

func TestHashPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)

	hash, err := svc.HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" {
		t.Fatal("hash is empty")
	}
	if hash == "testpassword123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestAuthenticatePassword(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	hash, _ := svc.HashPassword("correctpassword")

	user := &domain.User{
		ID:           domain.NewID(),
		Email:        "auth@example.com",
		Name:         "Auth User",
		PasswordHash: hash,
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusActive,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Correct password.
	got, err := svc.AuthenticatePassword(ctx, "auth@example.com", "correctpassword")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("user id mismatch")
	}

	// Wrong password.
	_, err = svc.AuthenticatePassword(ctx, "auth@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	// Non-existent user.
	_, err = svc.AuthenticatePassword(ctx, "nobody@example.com", "anything")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestAuthenticatePasswordDisabledAccount(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	hash, _ := svc.HashPassword("password123")

	user := &domain.User{
		ID:           domain.NewID(),
		Email:        "disabled@example.com",
		Name:         "Disabled",
		PasswordHash: hash,
		Role:         domain.RoleSuperUser,
		Status:       domain.UserStatusDisabled,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err := svc.AuthenticatePassword(ctx, "disabled@example.com", "password123")
	if err == nil {
		t.Fatal("expected error for disabled account")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	svc, store := newTestAuthService(t)
	ctx := context.Background()

	// Create prerequisite tenant and user.
	tenant := &domain.Tenant{ID: domain.NewID(), Name: "Test", Slug: "test-apikey", Status: domain.TenantStatusActive}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: domain.NewID(), TenantID: &tenant.ID, Email: "apikey@example.com", Name: "API", PasswordHash: "hash", Role: domain.RoleAdmin, Status: domain.UserStatusActive}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	rawKey, apiKey, err := svc.GenerateAPIKey(ctx, tenant.ID, user.ID, "my-key", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	if rawKey == "" {
		t.Fatal("raw key is empty")
	}
	if apiKey.Name != "my-key" {
		t.Errorf("name = %q, want %q", apiKey.Name, "my-key")
	}
	if apiKey.KeyPrefix == "" {
		t.Fatal("key prefix is empty")
	}

	// Authenticate with the raw key.
	got, err := svc.AuthenticateAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("authenticate api key: %v", err)
	}
	if got.ID != apiKey.ID {
		t.Errorf("api key id mismatch")
	}

	// Wrong key.
	_, err = svc.AuthenticateAPIKey(ctx, "totally-wrong-key-here")
	if err == nil {
		t.Fatal("expected error for wrong api key")
	}
}
