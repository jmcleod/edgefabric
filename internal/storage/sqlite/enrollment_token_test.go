package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func createTestTenant(t *testing.T, store interface {
	CreateTenant(ctx context.Context, t *domain.Tenant) error
}) domain.ID {
	t.Helper()
	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "test-tenant-" + domain.NewID().String()[:8],
		Slug: "test-" + domain.NewID().String()[:8],
	}
	if err := store.CreateTenant(context.Background(), tenant); err != nil {
		t.Fatalf("create test tenant: %v", err)
	}
	return tenant.ID
}

func createTestNode(t *testing.T, store interface {
	CreateNode(ctx context.Context, n *domain.Node) error
}, tenantID *domain.ID) domain.ID {
	t.Helper()
	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-node-" + domain.NewID().String()[:8],
		Hostname: "node.example.com",
		PublicIP: "203.0.113.1",
		SSHPort:  22,
		SSHUser:  "root",
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create test node: %v", err)
	}
	return node.ID
}

func TestEnrollmentTokenCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)

	tok := &domain.EnrollmentToken{
		ID:         domain.NewID(),
		TenantID:   tenantID,
		TargetType: domain.EnrollmentTargetNode,
		TargetID:   nodeID,
		Token:      "test-token-abc123",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	if err := store.CreateEnrollmentToken(ctx, tok); err != nil {
		t.Fatalf("create enrollment token: %v", err)
	}
	if tok.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Get by token string.
	got, err := store.GetEnrollmentToken(ctx, "test-token-abc123")
	if err != nil {
		t.Fatalf("get enrollment token: %v", err)
	}
	if got.ID != tok.ID {
		t.Errorf("expected ID %s, got %s", tok.ID, got.ID)
	}
	if got.TargetType != domain.EnrollmentTargetNode {
		t.Errorf("expected target_type node, got %s", got.TargetType)
	}
	if got.UsedAt != nil {
		t.Error("expected used_at to be nil before marking used")
	}

	// Mark used.
	if err := store.MarkEnrollmentTokenUsed(ctx, tok.ID); err != nil {
		t.Fatalf("mark enrollment token used: %v", err)
	}

	got, err = store.GetEnrollmentToken(ctx, "test-token-abc123")
	if err != nil {
		t.Fatalf("get after mark used: %v", err)
	}
	if got.UsedAt == nil {
		t.Error("expected used_at to be set after marking used")
	}

	// Mark used again should fail (already used).
	err = store.MarkEnrollmentTokenUsed(ctx, tok.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound on double-use, got %v", err)
	}
}

func TestEnrollmentTokenNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetEnrollmentToken(ctx, "nonexistent-token")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
