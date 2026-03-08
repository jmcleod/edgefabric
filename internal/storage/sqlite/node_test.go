package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestNodeCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a tenant first (nodes reference tenants).
	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "Test Tenant",
		Slug: "test-tenant",
	}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// Create a node.
	node := &domain.Node{
		ID:           domain.NewID(),
		TenantID:     &tenant.ID,
		Name:         "edge-us-east-1",
		Hostname:     "edge01.example.com",
		PublicIP:     "203.0.113.1",
		SSHPort:      22,
		SSHUser:      "root",
		Region:       "us-east-1",
		Provider:     "aws",
		Capabilities: []domain.NodeCapability{domain.CapabilityDNS, domain.CapabilityCDN},
	}
	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	if node.Status != domain.NodeStatusPending {
		t.Errorf("expected status pending, got %s", node.Status)
	}

	// Get.
	got, err := store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if got.Name != "edge-us-east-1" {
		t.Errorf("expected name edge-us-east-1, got %s", got.Name)
	}
	if got.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", got.Region)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(got.Capabilities))
	}

	// List all nodes.
	nodes, total, err := store.ListNodes(ctx, nil, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 total, got %d", total)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	// List by tenant.
	nodes, total, err = store.ListNodes(ctx, &tenant.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list nodes by tenant: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 node for tenant, got %d", total)
	}

	// List with wrong tenant.
	wrongTenantID := domain.NewID()
	nodes, total, err = store.ListNodes(ctx, &wrongTenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list nodes by wrong tenant: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 nodes for wrong tenant, got %d", total)
	}

	// Update.
	got.Name = "edge-us-east-1-updated"
	got.Status = domain.NodeStatusOnline
	got.Capabilities = []domain.NodeCapability{domain.CapabilityBGP}
	if err := store.UpdateNode(ctx, got); err != nil {
		t.Fatalf("update node: %v", err)
	}

	got2, _ := store.GetNode(ctx, node.ID)
	if got2.Name != "edge-us-east-1-updated" {
		t.Errorf("expected updated name, got %s", got2.Name)
	}
	if got2.Status != domain.NodeStatusOnline {
		t.Errorf("expected status online, got %s", got2.Status)
	}
	if len(got2.Capabilities) != 1 {
		t.Errorf("expected 1 capability after update, got %d", len(got2.Capabilities))
	}

	// Heartbeat.
	if err := store.UpdateNodeHeartbeat(ctx, node.ID); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	got3, _ := store.GetNode(ctx, node.ID)
	if got3.LastHeartbeat == nil {
		t.Error("expected last_heartbeat to be set after heartbeat")
	}
	if got3.Status != domain.NodeStatusOnline {
		t.Errorf("expected status online after heartbeat, got %s", got3.Status)
	}

	// Delete.
	if err := store.DeleteNode(ctx, node.ID); err != nil {
		t.Fatalf("delete node: %v", err)
	}

	_, err = store.GetNode(ctx, node.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestNodeNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetNode(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
