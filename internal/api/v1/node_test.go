package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// mockFleetService implements fleet.Service for node tests.
type mockFleetService struct {
	nodes      []*domain.Node
	nextErr    error
	nextNode   *domain.Node
	lastCreate fleet.CreateNodeRequest
}

func (m *mockFleetService) CreateNode(_ context.Context, req fleet.CreateNodeRequest) (*domain.Node, error) {
	if m.nextErr != nil {
		return nil, m.nextErr
	}
	m.lastCreate = req
	tenantID := domain.NewID()
	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: &tenantID,
		Name:     req.Name,
		Hostname: req.Hostname,
		PublicIP: req.PublicIP,
		Region:   req.Region,
		Provider: req.Provider,
		Status:   domain.NodeStatusPending,
	}
	m.nodes = append(m.nodes, node)
	if m.nextNode != nil {
		return m.nextNode, nil
	}
	return node, nil
}

func (m *mockFleetService) GetNode(_ context.Context, id domain.ID) (*domain.Node, error) {
	for _, n := range m.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockFleetService) ListNodes(_ context.Context, tenantID *domain.ID, _ storage.ListParams) ([]*domain.Node, int, error) {
	if tenantID == nil {
		return m.nodes, len(m.nodes), nil
	}
	var filtered []*domain.Node
	for _, n := range m.nodes {
		if n.TenantID != nil && *n.TenantID == *tenantID {
			filtered = append(filtered, n)
		}
	}
	return filtered, len(filtered), nil
}

func (m *mockFleetService) UpdateNode(_ context.Context, id domain.ID, _ fleet.UpdateNodeRequest) (*domain.Node, error) {
	for _, n := range m.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockFleetService) DeleteNode(_ context.Context, id domain.ID) error {
	for i, n := range m.nodes {
		if n.ID == id {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			return nil
		}
	}
	return storage.ErrNotFound
}

func (m *mockFleetService) RecordNodeHeartbeat(_ context.Context, _ domain.ID) error { return nil }

// Stubs for non-node fleet methods.
func (m *mockFleetService) CreateGateway(_ context.Context, _ fleet.CreateGatewayRequest) (*domain.Gateway, error) {
	return nil, nil
}
func (m *mockFleetService) GetGateway(_ context.Context, _ domain.ID) (*domain.Gateway, error) {
	return nil, storage.ErrNotFound
}
func (m *mockFleetService) ListGateways(_ context.Context, _ domain.ID, _ storage.ListParams) ([]*domain.Gateway, int, error) {
	return nil, 0, nil
}
func (m *mockFleetService) UpdateGateway(_ context.Context, _ domain.ID, _ fleet.UpdateGatewayRequest) (*domain.Gateway, error) {
	return nil, storage.ErrNotFound
}
func (m *mockFleetService) DeleteGateway(_ context.Context, _ domain.ID) error {
	return storage.ErrNotFound
}
func (m *mockFleetService) RecordGatewayHeartbeat(_ context.Context, _ domain.ID) error { return nil }
func (m *mockFleetService) CreateNodeGroup(_ context.Context, _ fleet.CreateNodeGroupRequest) (*domain.NodeGroup, error) {
	return nil, nil
}
func (m *mockFleetService) GetNodeGroup(_ context.Context, _ domain.ID) (*domain.NodeGroup, error) {
	return nil, storage.ErrNotFound
}
func (m *mockFleetService) ListNodeGroups(_ context.Context, _ domain.ID, _ storage.ListParams) ([]*domain.NodeGroup, int, error) {
	return nil, 0, nil
}
func (m *mockFleetService) DeleteNodeGroup(_ context.Context, _ domain.ID) error {
	return storage.ErrNotFound
}
func (m *mockFleetService) AddNodeToGroup(_ context.Context, _, _ domain.ID) error    { return nil }
func (m *mockFleetService) RemoveNodeFromGroup(_ context.Context, _, _ domain.ID) error { return nil }
func (m *mockFleetService) CreateSSHKey(_ context.Context, _ *domain.SSHKey) error     { return nil }
func (m *mockFleetService) GetSSHKey(_ context.Context, _ domain.ID) (*domain.SSHKey, error) {
	return nil, storage.ErrNotFound
}
func (m *mockFleetService) ListSSHKeys(_ context.Context, _ storage.ListParams) ([]*domain.SSHKey, int, error) {
	return nil, 0, nil
}
func (m *mockFleetService) DeleteSSHKey(_ context.Context, _ domain.ID) error {
	return storage.ErrNotFound
}

func newTestNodeHandler() (*NodeHandler, *mockFleetService) {
	fleetSvc := &mockFleetService{}
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()
	return NewNodeHandler(fleetSvc, authorizer, auditLog), fleetSvc
}

func TestNodeCreate_SuperUser(t *testing.T) {
	handler, _ := newTestNodeHandler()

	body := `{"name":"edge-1","hostname":"edge1.example.com","public_ip":"10.0.0.1","region":"us-east-1","provider":"aws"}`
	req := httptest.NewRequest("POST", "/api/v1/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// SuperUser with a tenant.
	tenantID := domain.NewID()
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleSuperUser,
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNodeCreate_ReadOnlyForbidden(t *testing.T) {
	handler, _ := newTestNodeHandler()
	authorizer := rbac.NewAuthorizer()
	auditLog := newMockAuditLogger()

	// Wire up full route with auth middleware to test RBAC.
	mux := http.NewServeMux()
	tokenSvc := newTestTokenService()
	authMW := middleware.Auth(tokenSvc, nil)
	nh := NewNodeHandler(handler.svc, authorizer, auditLog)
	nh.Register(mux, authMW)

	// Issue a ReadOnly token.
	tenantID := domain.NewID()
	token, err := tokenSvc.Issue(auth.Claims{
		UserID:   domain.NewID(),
		TenantID: &tenantID,
		Role:     domain.RoleReadOnly,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	body := `{"name":"edge-1","hostname":"edge1.example.com","public_ip":"10.0.0.1"}`
	req := httptest.NewRequest("POST", "/api/v1/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for ReadOnly user, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNodeGet_NotFound(t *testing.T) {
	handler, _ := newTestNodeHandler()

	req := httptest.NewRequest("GET", "/api/v1/nodes/"+domain.NewID().String(), nil)
	req.SetPathValue("id", domain.NewID().String())

	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleSuperUser,
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNodeList_Empty(t *testing.T) {
	handler, _ := newTestNodeHandler()

	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	ctx := middleware.ContextWithClaims(req.Context(), &auth.Claims{
		UserID: domain.NewID(),
		Role:   domain.RoleSuperUser,
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data  json.RawMessage `json:"data"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}
