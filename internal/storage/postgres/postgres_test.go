package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// getTestStore returns a PostgresStore connected to the test database.
// Tests using this helper are skipped unless POSTGRES_DSN is set.
func getTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set; skipping PostgreSQL integration test")
	}

	store, err := New(dsn)
	if err != nil {
		t.Fatalf("failed to create postgres store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return store
}

func TestMigrations_Valid(t *testing.T) {
	// Verify that all migrations have non-empty SQL and descriptions.
	for i, m := range migrations {
		if m.SQL == "" {
			t.Errorf("migration %d has empty SQL", i+1)
		}
		if m.Description == "" {
			t.Errorf("migration %d has empty description", i+1)
		}
	}
}

func TestMigrations_Count(t *testing.T) {
	// Ensure the PostgreSQL migration count matches SQLite (minus the 2 ALTER TABLE
	// migrations that are folded into the initial CREATE TABLE in PostgreSQL).
	// SQLite has 32 migrations; PostgreSQL has 30 (no separate ALTER TABLE for
	// last_config_sync on nodes and gateways since the column is in the CREATE TABLE).
	if len(migrations) != 32 {
		t.Errorf("expected 32 PostgreSQL migrations, got %d", len(migrations))
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"ERROR: duplicate key value violates unique constraint \"tenants_pkey\" (SQLSTATE 23505)", true},
		{"some other error", false},
		{"", false},
	}

	for _, tt := range tests {
		err := error(nil)
		if tt.msg != "" {
			err = &testError{msg: tt.msg}
		}
		if got := isUniqueViolation(err); got != tt.want {
			t.Errorf("isUniqueViolation(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// --- Integration tests (require POSTGRES_DSN) ---

func TestTenantCRUD(t *testing.T) {
	store := getTestStore(t)

	ctx := context.Background()
	id := domain.ID(uuid.New())

	tenant := &domain.Tenant{
		ID:     id,
		Name:   "Test Tenant " + id.String()[:8],
		Slug:   "test-" + id.String()[:8],
		Status: domain.TenantStatusActive,
	}

	// Create.
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	// Get.
	got, err := store.GetTenant(ctx, id)
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.Name != tenant.Name {
		t.Errorf("Name = %q, want %q", got.Name, tenant.Name)
	}

	// GetBySlug.
	got, err = store.GetTenantBySlug(ctx, tenant.Slug)
	if err != nil {
		t.Fatalf("GetTenantBySlug: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID = %v, want %v", got.ID, id)
	}

	// Update.
	got.Name = "Updated " + id.String()[:8]
	if err := store.UpdateTenant(ctx, got); err != nil {
		t.Fatalf("UpdateTenant: %v", err)
	}

	// List.
	tenants, total, err := store.ListTenants(ctx, storage.ListParams{Limit: 100})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 tenant, got %d", total)
	}
	found := false
	for _, tt := range tenants {
		if tt.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("created tenant not found in list")
	}

	// Delete.
	if err := store.DeleteTenant(ctx, id); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}

	// Verify gone.
	_, err = store.GetTenant(ctx, id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestNodeCRUD(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	// Create prerequisite tenant.
	tenantID := domain.ID(uuid.New())
	tenant := &domain.Tenant{
		ID:   tenantID,
		Name: "NodeTest " + tenantID.String()[:8],
		Slug: "nodetest-" + tenantID.String()[:8],
	}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	t.Cleanup(func() { store.DeleteTenant(ctx, tenantID) })

	nodeID := domain.ID(uuid.New())
	node := &domain.Node{
		ID:           nodeID,
		TenantID:     &tenantID,
		Name:         "test-node",
		Hostname:     "test.example.com",
		PublicIP:     "1.2.3.4",
		Status:       domain.NodeStatusPending,
		SSHPort:      22,
		SSHUser:      "root",
		Capabilities: []domain.NodeCapability{"dns", "cdn"},
	}

	// Create.
	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Get.
	got, err := store.GetNode(ctx, nodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != "test-node" {
		t.Errorf("Name = %q, want %q", got.Name, "test-node")
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(got.Capabilities))
	}

	// List.
	nodes, total, err := store.ListNodes(ctx, &tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(nodes) != 1 {
		t.Errorf("len(nodes) = %d, want 1", len(nodes))
	}

	// Heartbeat.
	if err := store.UpdateNodeHeartbeat(ctx, nodeID); err != nil {
		t.Fatalf("UpdateNodeHeartbeat: %v", err)
	}
	got, _ = store.GetNode(ctx, nodeID)
	if got.Status != domain.NodeStatusOnline {
		t.Errorf("Status = %q, want %q", got.Status, domain.NodeStatusOnline)
	}
	if got.LastHeartbeat == nil {
		t.Error("LastHeartbeat should be set after heartbeat")
	}

	// Delete.
	if err := store.DeleteNode(ctx, nodeID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	_, err = store.GetNode(ctx, nodeID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSchemaVersion(t *testing.T) {
	store := getTestStore(t)

	version, err := store.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != len(migrations) {
		t.Errorf("SchemaVersion = %d, want %d", version, len(migrations))
	}
}

func TestCursorPagination(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	tenantID := domain.ID(uuid.New())
	tenant := &domain.Tenant{
		ID:   tenantID,
		Name: "CursorTest " + tenantID.String()[:8],
		Slug: "cursortest-" + tenantID.String()[:8],
	}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	t.Cleanup(func() {
		// Clean up nodes first, then tenant.
		nodes, _, _ := store.ListNodes(ctx, &tenantID, storage.ListParams{Limit: 100})
		for _, n := range nodes {
			store.DeleteNode(ctx, n.ID)
		}
		store.DeleteTenant(ctx, tenantID)
	})

	// Create 3 nodes with small delays to get different created_at values.
	for i := 0; i < 3; i++ {
		nid := domain.ID(uuid.New())
		node := &domain.Node{
			ID:       nid,
			TenantID: &tenantID,
			Name:     "cursor-node-" + nid.String()[:8],
			Hostname: "cursor-" + nid.String()[:8] + ".example.com",
			PublicIP: "10.0.0." + string(rune('1'+i)),
			SSHPort:  22,
			SSHUser:  "root",
		}
		if err := store.CreateNode(ctx, node); err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps.
	}

	// First page.
	nodes, total, err := store.ListNodes(ctx, &tenantID, storage.ListParams{Limit: 2})
	if err != nil {
		t.Fatalf("ListNodes page 1: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(nodes) != 2 {
		t.Fatalf("page 1 len = %d, want 2", len(nodes))
	}

	// Second page using cursor.
	last := nodes[len(nodes)-1]
	cursor := storage.EncodeCursor(last.CreatedAt, last.ID.String())
	nodes2, _, err := store.ListNodes(ctx, &tenantID, storage.ListParams{Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("ListNodes page 2: %v", err)
	}
	if len(nodes2) != 1 {
		t.Errorf("page 2 len = %d, want 1", len(nodes2))
	}
}
