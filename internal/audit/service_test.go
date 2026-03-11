package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// mockAuditEventStore implements storage.AuditEventStore for testing.
type mockAuditEventStore struct {
	createdEvent *domain.AuditEvent
	createErr    error

	listEvents   []*domain.AuditEvent
	listTotal    int
	listErr      error
	listTenantID *domain.ID
	listParams   storage.ListParams
}

func (m *mockAuditEventStore) CreateAuditEvent(_ context.Context, e *domain.AuditEvent) error {
	m.createdEvent = e
	return m.createErr
}

func (m *mockAuditEventStore) ListAuditEvents(_ context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.AuditEvent, int, error) {
	m.listTenantID = tenantID
	m.listParams = params
	return m.listEvents, m.listTotal, m.listErr
}

func TestNewLogger(t *testing.T) {
	store := &mockAuditEventStore{}
	logger := slog.Default()

	l := NewLogger(store, logger)
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestLog(t *testing.T) {
	store := &mockAuditEventStore{}
	logger := slog.Default()
	l := NewLogger(store, logger)

	tenantID := domain.NewID()
	userID := domain.NewID()
	apiKeyID := domain.NewID()

	details := map[string]string{"key": "value"}

	event := Event{
		TenantID: &tenantID,
		UserID:   &userID,
		APIKeyID: &apiKeyID,
		Action:   "create",
		Resource: "node",
		Details:  details,
		SourceIP: "192.168.1.1",
	}

	err := l.Log(context.Background(), event)
	if err != nil {
		t.Fatalf("Log returned unexpected error: %v", err)
	}

	if store.createdEvent == nil {
		t.Fatal("expected store.CreateAuditEvent to be called")
	}

	ae := store.createdEvent

	// Verify ID was generated.
	if ae.ID == (domain.ID{}) {
		t.Error("expected non-zero audit event ID")
	}

	// Verify fields are passed through.
	if ae.TenantID == nil || *ae.TenantID != tenantID {
		t.Errorf("TenantID = %v, want %v", ae.TenantID, tenantID)
	}
	if ae.UserID == nil || *ae.UserID != userID {
		t.Errorf("UserID = %v, want %v", ae.UserID, userID)
	}
	if ae.APIKeyID == nil || *ae.APIKeyID != apiKeyID {
		t.Errorf("APIKeyID = %v, want %v", ae.APIKeyID, apiKeyID)
	}
	if ae.Action != "create" {
		t.Errorf("Action = %q, want %q", ae.Action, "create")
	}
	if ae.Resource != "node" {
		t.Errorf("Resource = %q, want %q", ae.Resource, "node")
	}
	if ae.SourceIP != "192.168.1.1" {
		t.Errorf("SourceIP = %q, want %q", ae.SourceIP, "192.168.1.1")
	}
	if ae.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}

	// Verify Details are JSON-marshaled.
	var got map[string]string
	if err := json.Unmarshal(ae.Details, &got); err != nil {
		t.Fatalf("failed to unmarshal Details: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("Details[key] = %q, want %q", got["key"], "value")
	}
}

func TestLogWithNilTenantID(t *testing.T) {
	store := &mockAuditEventStore{}
	logger := slog.Default()
	l := NewLogger(store, logger)

	event := Event{
		TenantID: nil,
		UserID:   nil,
		APIKeyID: nil,
		Action:   "system.startup",
		Resource: "system",
		Details:  nil,
		SourceIP: "127.0.0.1",
	}

	err := l.Log(context.Background(), event)
	if err != nil {
		t.Fatalf("Log returned unexpected error: %v", err)
	}

	ae := store.createdEvent
	if ae == nil {
		t.Fatal("expected store.CreateAuditEvent to be called")
	}

	if ae.TenantID != nil {
		t.Errorf("TenantID = %v, want nil", ae.TenantID)
	}
	if ae.UserID != nil {
		t.Errorf("UserID = %v, want nil", ae.UserID)
	}
	if ae.APIKeyID != nil {
		t.Errorf("APIKeyID = %v, want nil", ae.APIKeyID)
	}

	// nil Details should marshal to JSON "null".
	if string(ae.Details) != "null" {
		t.Errorf("Details = %s, want %q", ae.Details, "null")
	}
}

func TestList(t *testing.T) {
	expected := []*domain.AuditEvent{
		{ID: domain.NewID(), Action: "create"},
		{ID: domain.NewID(), Action: "delete"},
	}

	store := &mockAuditEventStore{
		listEvents: expected,
		listTotal:  2,
	}
	logger := slog.Default()
	l := NewLogger(store, logger)

	params := storage.ListParams{Offset: 0, Limit: 10}
	events, total, err := l.List(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("List returned unexpected error: %v", err)
	}

	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len(events) = %d, want 2", len(events))
	}
	if events[0].Action != "create" {
		t.Errorf("events[0].Action = %q, want %q", events[0].Action, "create")
	}

	// Verify params were passed through.
	if store.listParams.Limit != 10 {
		t.Errorf("listParams.Limit = %d, want 10", store.listParams.Limit)
	}
}

func TestListWithTenantFilter(t *testing.T) {
	tenantID := domain.NewID()

	store := &mockAuditEventStore{
		listEvents: []*domain.AuditEvent{},
		listTotal:  0,
	}
	logger := slog.Default()
	l := NewLogger(store, logger)

	params := storage.ListParams{Offset: 0, Limit: 50}
	_, _, err := l.List(context.Background(), &tenantID, params)
	if err != nil {
		t.Fatalf("List returned unexpected error: %v", err)
	}

	// Verify tenant ID filter was passed to the store.
	if store.listTenantID == nil {
		t.Fatal("expected listTenantID to be non-nil")
	}
	if *store.listTenantID != tenantID {
		t.Errorf("listTenantID = %v, want %v", *store.listTenantID, tenantID)
	}
}
