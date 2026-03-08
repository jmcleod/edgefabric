package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestNodeGroupCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create tenant and node.
	tenant := &domain.Tenant{ID: domain.NewID(), Name: "Grp Tenant", Slug: "grp-tenant"}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	node := &domain.Node{
		ID: domain.NewID(), TenantID: &tenant.ID,
		Name: "n1", Hostname: "n1.example.com", PublicIP: "1.2.3.4",
		SSHPort: 22, SSHUser: "root",
	}
	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	// Create group.
	group := &domain.NodeGroup{
		ID:          domain.NewID(),
		TenantID:    tenant.ID,
		Name:        "production",
		Description: "Production nodes",
	}
	if err := store.CreateNodeGroup(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Get.
	got, err := store.GetNodeGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if got.Name != "production" {
		t.Errorf("expected name production, got %s", got.Name)
	}

	// List.
	groups, total, err := store.ListNodeGroups(ctx, tenant.ID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if total != 1 || len(groups) != 1 {
		t.Errorf("expected 1 group, got total=%d len=%d", total, len(groups))
	}

	// Add node to group.
	if err := store.AddNodeToGroup(ctx, group.ID, node.ID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Duplicate add should fail.
	if err := store.AddNodeToGroup(ctx, group.ID, node.ID); err == nil {
		t.Error("expected error on duplicate add")
	}

	// List group nodes.
	nodes, err := store.ListGroupNodes(ctx, group.ID)
	if err != nil {
		t.Fatalf("list group nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node in group, got %d", len(nodes))
	}
	if nodes[0].ID != node.ID {
		t.Error("wrong node in group")
	}

	// List groups by node.
	nodeGroups, err := store.ListNodeGroups_ByNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("list groups by node: %v", err)
	}
	if len(nodeGroups) != 1 {
		t.Errorf("expected 1 group for node, got %d", len(nodeGroups))
	}

	// Remove node from group.
	if err := store.RemoveNodeFromGroup(ctx, group.ID, node.ID); err != nil {
		t.Fatalf("remove node from group: %v", err)
	}
	nodes, _ = store.ListGroupNodes(ctx, group.ID)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after removal, got %d", len(nodes))
	}

	// Update group.
	got.Name = "staging"
	got.Description = "Staging nodes"
	if err := store.UpdateNodeGroup(ctx, got); err != nil {
		t.Fatalf("update group: %v", err)
	}
	got2, _ := store.GetNodeGroup(ctx, group.ID)
	if got2.Name != "staging" {
		t.Errorf("expected name staging, got %s", got2.Name)
	}

	// Delete group.
	if err := store.DeleteNodeGroup(ctx, group.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	_, err = store.GetNodeGroup(ctx, group.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
