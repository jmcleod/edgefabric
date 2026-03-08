package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/rbac"
)

func newTestAuthHandler(authSvc auth.Service, auditLog *mockAuditLogger) *AuthHandler {
	tokenSvc := newTestTokenService()
	apiKeys := &mockAPIKeyStore{}
	authorizer := rbac.NewAuthorizer()
	return NewAuthHandler(authSvc, tokenSvc, apiKeys, authorizer, auditLog)
}

func TestLogin_Success(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()

	authSvc := &mockAuthService{
		authenticatePasswordFn: func(_ context.Context, email, password string) (*domain.User, error) {
			if email == "user@example.com" && password == "correct-password" {
				return &domain.User{
					ID:       userID,
					TenantID: &tenantID,
					Role:     domain.RoleAdmin,
				}, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	body := `{"email":"user@example.com","password":"correct-password"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiutil.Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	// Verify a token was returned in the data.
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Error("expected non-empty token in response")
	}
	if totpRequired, ok := data["totp_required"].(bool); !ok || totpRequired {
		t.Error("expected totp_required to be false")
	}

	// Verify audit event was logged.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "login" {
		t.Errorf("expected audit action 'login', got %q", events[0].Action)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	authSvc := &mockAuthService{
		authenticatePasswordFn: func(_ context.Context, _, _ string) (*domain.User, error) {
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	body := `{"email":"user@example.com","password":"wrong-password"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	// Verify failed login audit event.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event for failed login, got %d", len(events))
	}
	if events[0].Action != "login_failed" {
		t.Errorf("expected audit action 'login_failed', got %q", events[0].Action)
	}
	// Should NOT include UserID/TenantID (to avoid leaking account existence).
	if events[0].UserID != nil {
		t.Error("expected no UserID in failed login audit event")
	}
	if events[0].TenantID != nil {
		t.Error("expected no TenantID in failed login audit event")
	}
}

func TestLogin_TOTPRequired(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()

	authSvc := &mockAuthService{
		authenticatePasswordFn: func(_ context.Context, _, _ string) (*domain.User, error) {
			return &domain.User{
				ID:          userID,
				TenantID:    &tenantID,
				Role:        domain.RoleAdmin,
				TOTPEnabled: true,
			}, nil
		},
	}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	body := `{"email":"user@example.com","password":"correct-password"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiutil.Response
	json.NewDecoder(w.Body).Decode(&resp)
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}
	if totpRequired, ok := data["totp_required"].(bool); !ok || !totpRequired {
		t.Error("expected totp_required to be true")
	}

	// No login audit event yet (login isn't complete until TOTP verification).
	events := auditLog.Events()
	if len(events) != 0 {
		t.Errorf("expected 0 audit events (login incomplete), got %d", len(events))
	}
}

func TestLogin_BadRequestBody(t *testing.T) {
	authSvc := &mockAuthService{}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestEnrollTOTP_AuditLogged(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()

	authSvc := &mockAuthService{
		enrollTOTPFn: func(_ context.Context, _ domain.ID) (string, string, error) {
			return "TESTSECRET", "otpauth://totp/test", nil
		},
	}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	req := httptest.NewRequest("POST", "/api/v1/auth/totp/enroll", nil)
	// Inject claims into context.
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   userID,
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.EnrollTOTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify audit event for TOTP enrollment.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "totp_enroll" {
		t.Errorf("expected audit action 'totp_enroll', got %q", events[0].Action)
	}
	if events[0].Resource != "user" {
		t.Errorf("expected audit resource 'user', got %q", events[0].Resource)
	}
}

func TestCreateAPIKey_Success(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()

	authSvc := &mockAuthService{}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	body := `{"name":"test-key","role":"operator"}`
	req := httptest.NewRequest("POST", "/api/v1/api-keys", strings.NewReader(body))
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   userID,
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.CreateAPIKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiutil.Response
	json.NewDecoder(w.Body).Decode(&resp)
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}
	rawKey, ok := data["raw_key"].(string)
	if !ok || rawKey == "" {
		t.Error("expected non-empty raw_key in response")
	}

	// Verify audit event.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "create" {
		t.Errorf("expected audit action 'create', got %q", events[0].Action)
	}
	if events[0].Resource != "api_key" {
		t.Errorf("expected audit resource 'api_key', got %q", events[0].Resource)
	}
}

func TestDeleteAPIKey_Success(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()
	apiKeyID := domain.NewID()

	authSvc := &mockAuthService{}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	// Pre-populate the mock store with a key.
	handler.apiKeys.(*mockAPIKeyStore).keys = append(handler.apiKeys.(*mockAPIKeyStore).keys, &domain.APIKey{
		ID:        apiKeyID,
		TenantID:  tenantID,
		UserID:    userID,
		Name:      "test-key",
		Role:      domain.RoleReadOnly,
		KeyPrefix: "ef_test",
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("DELETE", "/api/v1/api-keys/"+apiKeyID.String(), nil)
	req.SetPathValue("id", apiKeyID.String())
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   userID,
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.DeleteAPIKey(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify audit event.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "delete" {
		t.Errorf("expected audit action 'delete', got %q", events[0].Action)
	}
	if events[0].Resource != "api_key" {
		t.Errorf("expected audit resource 'api_key', got %q", events[0].Resource)
	}
}

func TestListAPIKeys_Success(t *testing.T) {
	tenantID := domain.NewID()
	userID := domain.NewID()

	authSvc := &mockAuthService{}
	auditLog := newMockAuditLogger()
	handler := newTestAuthHandler(authSvc, auditLog)

	// Add some keys.
	for i := 0; i < 3; i++ {
		handler.apiKeys.(*mockAPIKeyStore).keys = append(handler.apiKeys.(*mockAPIKeyStore).keys, &domain.APIKey{
			ID:        domain.NewID(),
			TenantID:  tenantID,
			UserID:    userID,
			Name:      fmt.Sprintf("key-%d", i),
			Role:      domain.RoleReadOnly,
			KeyPrefix: "ef_test",
			CreatedAt: time.Now(),
		})
	}

	req := httptest.NewRequest("GET", "/api/v1/api-keys", nil)
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   userID,
		TenantID: &tenantID,
		Role:     domain.RoleAdmin,
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ListAPIKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiutil.ListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}
}
