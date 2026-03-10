package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// mockProvisioningSvc implements provisioning.Service for enrollment tests.
type mockProvisioningSvc struct {
	completeResult *provisioning.EnrollmentResult
	completeErr    error
	validateResult *domain.EnrollmentToken
	validateErr    error
}

func (m *mockProvisioningSvc) GenerateEnrollmentToken(_ context.Context, _ domain.ID, _ domain.ID) (*domain.EnrollmentToken, error) {
	return &domain.EnrollmentToken{ID: domain.NewID(), Token: "test-token"}, nil
}

func (m *mockProvisioningSvc) ValidateEnrollmentToken(_ context.Context, _ string) (*domain.EnrollmentToken, error) {
	if m.validateErr != nil {
		return nil, m.validateErr
	}
	return m.validateResult, nil
}

func (m *mockProvisioningSvc) CompleteEnrollment(_ context.Context, _ string) (*provisioning.EnrollmentResult, error) {
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	return m.completeResult, nil
}

func (m *mockProvisioningSvc) EnrollNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) StartNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) StopNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) RestartNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) UpgradeNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) ReprovisionNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) DecommissionNode(_ context.Context, _, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) GetJob(_ context.Context, _ domain.ID) (*domain.ProvisioningJob, error) {
	return nil, storage.ErrNotFound
}
func (m *mockProvisioningSvc) ListJobs(_ context.Context, _ *domain.ID, _ storage.ListParams) ([]*domain.ProvisioningJob, int, error) {
	return nil, 0, nil
}
func (m *mockProvisioningSvc) RotateSSHKey(_ context.Context, _ domain.ID) (*domain.SSHKey, error) {
	return nil, nil
}
func (m *mockProvisioningSvc) DeploySSHKey(_ context.Context, _ domain.ID) error { return nil }

func TestEnroll_Success(t *testing.T) {
	tenantID := domain.NewID()
	nodeID := domain.NewID()
	svc := &mockProvisioningSvc{
		completeResult: &provisioning.EnrollmentResult{
			NodeID:      nodeID,
			TenantID:    &tenantID,
			WireGuardIP: "10.100.0.2",
		},
	}
	tokenSvc := newTestTokenService()
	auditLog := newMockAuditLogger()
	handler := NewEnrollmentHandler(svc, tokenSvc, auditLog)

	body := `{"token":"valid-enrollment-token"}`
	req := httptest.NewRequest("POST", "/api/v1/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Enroll(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Status      string `json:"status"`
			NodeID      string `json:"node_id"`
			APIToken    string `json:"api_token"`
			WireGuardIP string `json:"wireguard_ip"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Data.Status != "enrolled" {
		t.Errorf("expected status enrolled, got %s", resp.Data.Status)
	}
	if resp.Data.NodeID != nodeID.String() {
		t.Errorf("expected node_id %s, got %s", nodeID, resp.Data.NodeID)
	}
	if resp.Data.APIToken == "" {
		t.Error("expected non-empty api_token")
	}
	if resp.Data.WireGuardIP != "10.100.0.2" {
		t.Errorf("expected wireguard_ip 10.100.0.2, got %s", resp.Data.WireGuardIP)
	}

	// Verify audit event was logged.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "enrollment_completed" {
		t.Errorf("expected enrollment_completed, got %s", events[0].Action)
	}
}

func TestEnroll_InvalidToken(t *testing.T) {
	svc := &mockProvisioningSvc{
		completeErr: storage.ErrNotFound,
	}
	tokenSvc := newTestTokenService()
	auditLog := newMockAuditLogger()
	handler := NewEnrollmentHandler(svc, tokenSvc, auditLog)

	body := `{"token":"bad-token"}`
	req := httptest.NewRequest("POST", "/api/v1/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Enroll(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Verify failed enrollment is audited.
	events := auditLog.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "enrollment_failed" {
		t.Errorf("expected enrollment_failed, got %s", events[0].Action)
	}
}

func TestEnroll_EmptyToken(t *testing.T) {
	svc := &mockProvisioningSvc{}
	tokenSvc := newTestTokenService()
	auditLog := newMockAuditLogger()
	handler := NewEnrollmentHandler(svc, tokenSvc, auditLog)

	body := `{"token":""}`
	req := httptest.NewRequest("POST", "/api/v1/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Enroll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestEnroll_ReusedToken(t *testing.T) {
	svc := &mockProvisioningSvc{
		completeErr: storage.ErrConflict,
	}
	tokenSvc := newTestTokenService()
	auditLog := newMockAuditLogger()
	handler := NewEnrollmentHandler(svc, tokenSvc, auditLog)

	body := `{"token":"already-used-token"}`
	req := httptest.NewRequest("POST", "/api/v1/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Enroll(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnroll_APITokenIsValid(t *testing.T) {
	// Verify the API token returned in enrollment response is a valid auth token.
	tenantID := domain.NewID()
	nodeID := domain.NewID()
	svc := &mockProvisioningSvc{
		completeResult: &provisioning.EnrollmentResult{
			NodeID:      nodeID,
			TenantID:    &tenantID,
			WireGuardIP: "10.100.0.5",
		},
	}
	tokenSvc := newTestTokenService()
	auditLog := newMockAuditLogger()
	handler := NewEnrollmentHandler(svc, tokenSvc, auditLog)

	body := `{"token":"valid-token"}`
	req := httptest.NewRequest("POST", "/api/v1/enroll", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Enroll(w, req)

	var resp struct {
		Data struct {
			APIToken string `json:"api_token"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify token can be decoded.
	claims, err := tokenSvc.Verify(resp.Data.APIToken)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.UserID != nodeID {
		t.Errorf("expected claims UserID %s, got %s", nodeID, claims.UserID)
	}
	if claims.Role != domain.RoleReadOnly {
		t.Errorf("expected claims role readonly, got %s", claims.Role)
	}
}
