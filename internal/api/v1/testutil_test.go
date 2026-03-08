package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/user"
)

// errInvalidCredentials matches the error returned by the real auth service.
var errInvalidCredentials = fmt.Errorf("invalid credentials")

// --- Mock Audit Logger ---

// mockAuditLogger captures audit events for test assertions.
type mockAuditLogger struct {
	mu     sync.Mutex
	events []audit.Event
}

func newMockAuditLogger() *mockAuditLogger {
	return &mockAuditLogger{}
}

func (m *mockAuditLogger) Log(_ context.Context, event audit.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditLogger) List(_ context.Context, _ *domain.ID, _ storage.ListParams) ([]*domain.AuditEvent, int, error) {
	return nil, 0, nil
}

func (m *mockAuditLogger) Events() []audit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]audit.Event, len(m.events))
	copy(copied, m.events)
	return copied
}

func (m *mockAuditLogger) LastEvent() *audit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) == 0 {
		return nil
	}
	e := m.events[len(m.events)-1]
	return &e
}

// --- Mock Auth Service ---

// mockAuthService implements auth.Service with configurable behavior.
type mockAuthService struct {
	authenticatePasswordFn func(ctx context.Context, email, password string) (*domain.User, error)
	authenticateTOTPFn     func(ctx context.Context, userID domain.ID, code string) error
	authenticateAPIKeyFn   func(ctx context.Context, key string) (*domain.APIKey, error)
	enrollTOTPFn           func(ctx context.Context, userID domain.ID) (string, string, error)
	confirmTOTPFn          func(ctx context.Context, userID domain.ID, code string) error
	generateAPIKeyFn       func(ctx context.Context, tenantID, userID domain.ID, name string, role domain.Role) (string, *domain.APIKey, error)
}

func (m *mockAuthService) AuthenticatePassword(ctx context.Context, email, password string) (*domain.User, error) {
	if m.authenticatePasswordFn != nil {
		return m.authenticatePasswordFn(ctx, email, password)
	}
	return nil, errInvalidCredentials
}

func (m *mockAuthService) AuthenticateTOTP(ctx context.Context, userID domain.ID, code string) error {
	if m.authenticateTOTPFn != nil {
		return m.authenticateTOTPFn(ctx, userID, code)
	}
	return errInvalidCredentials
}

func (m *mockAuthService) AuthenticateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	if m.authenticateAPIKeyFn != nil {
		return m.authenticateAPIKeyFn(ctx, key)
	}
	return nil, errInvalidCredentials
}

func (m *mockAuthService) HashPassword(password string) (string, error) {
	return "$2a$12$test", nil
}

func (m *mockAuthService) EnrollTOTP(ctx context.Context, userID domain.ID) (string, string, error) {
	if m.enrollTOTPFn != nil {
		return m.enrollTOTPFn(ctx, userID)
	}
	return "TESTSECRET", "otpauth://totp/test", nil
}

func (m *mockAuthService) ConfirmTOTP(ctx context.Context, userID domain.ID, code string) error {
	if m.confirmTOTPFn != nil {
		return m.confirmTOTPFn(ctx, userID, code)
	}
	return nil
}

func (m *mockAuthService) GenerateAPIKey(ctx context.Context, tenantID, userID domain.ID, name string, role domain.Role) (string, *domain.APIKey, error) {
	if m.generateAPIKeyFn != nil {
		return m.generateAPIKeyFn(ctx, tenantID, userID, name, role)
	}
	key := &domain.APIKey{
		ID:        domain.NewID(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		Role:      role,
		KeyPrefix: "ef_test",
		CreatedAt: time.Now(),
	}
	return "ef_test_rawkey123", key, nil
}

// --- Mock API Key Store ---

// mockAPIKeyStore is a minimal mock for storage.APIKeyStore.
type mockAPIKeyStore struct {
	keys []*domain.APIKey
}

func (m *mockAPIKeyStore) CreateAPIKey(_ context.Context, k *domain.APIKey) error {
	m.keys = append(m.keys, k)
	return nil
}

func (m *mockAPIKeyStore) GetAPIKey(_ context.Context, id domain.ID) (*domain.APIKey, error) {
	for _, k := range m.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockAPIKeyStore) GetAPIKeyByPrefix(_ context.Context, _ string) (*domain.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockAPIKeyStore) ListAPIKeys(_ context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.APIKey, int, error) {
	var result []*domain.APIKey
	for _, k := range m.keys {
		if k.TenantID == tenantID {
			result = append(result, k)
		}
	}
	return result, len(result), nil
}

func (m *mockAPIKeyStore) DeleteAPIKey(_ context.Context, id domain.ID) error {
	for i, k := range m.keys {
		if k.ID == id {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			return nil
		}
	}
	return storage.ErrNotFound
}

func (m *mockAPIKeyStore) UpdateAPIKeyLastUsed(_ context.Context, _ domain.ID) error {
	return nil
}

// --- Mock User Service ---

// mockUserService implements user.Service with configurable behavior.
type mockUserService struct {
	getFn func(ctx context.Context, id domain.ID) (*domain.User, error)
}

func (m *mockUserService) Create(_ context.Context, _ user.CreateRequest) (*domain.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockUserService) Get(ctx context.Context, id domain.ID) (*domain.User, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *mockUserService) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, storage.ErrNotFound
}

func (m *mockUserService) List(_ context.Context, _ *domain.ID, _ storage.ListParams) ([]*domain.User, int, error) {
	return nil, 0, nil
}

func (m *mockUserService) Update(_ context.Context, _ domain.ID, _ user.UpdateRequest) (*domain.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockUserService) Delete(_ context.Context, _ domain.ID) error {
	return fmt.Errorf("not implemented")
}

// --- Test Token Service ---

var testSigningKey = []byte("test-signing-key-for-unit-tests!")

func newTestTokenService() *auth.TokenService {
	return auth.NewTokenService(testSigningKey, 1*time.Hour)
}
