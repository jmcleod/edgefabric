package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestAuditEventCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := domain.NewID()
	userID := domain.NewID()

	event := &domain.AuditEvent{
		ID:        domain.NewID(),
		TenantID:  &tenantID,
		UserID:    &userID,
		Action:    "create",
		Resource:  "tenant",
		Details:   []byte(`{"name":"test"}`),
		SourceIP:  "127.0.0.1",
		Timestamp: time.Now().UTC(),
	}

	// Create.
	if err := store.CreateAuditEvent(ctx, event); err != nil {
		t.Fatalf("create audit event: %v", err)
	}

	// List all.
	events, total, err := store.ListAuditEvents(ctx, nil, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(events) != 1 {
		t.Errorf("len = %d, want 1", len(events))
	}
	if events[0].Action != "create" {
		t.Errorf("action = %q, want %q", events[0].Action, "create")
	}

	// List by tenant.
	events2, total2, err := store.ListAuditEvents(ctx, &tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list audit events by tenant: %v", err)
	}
	if total2 != 1 || len(events2) != 1 {
		t.Errorf("tenant-scoped list: total=%d, len=%d", total2, len(events2))
	}

	// List with non-existent tenant.
	otherTenant := domain.NewID()
	_, total3, err := store.ListAuditEvents(ctx, &otherTenant, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list audit events other tenant: %v", err)
	}
	if total3 != 0 {
		t.Errorf("expected 0 events for other tenant, got %d", total3)
	}
}
