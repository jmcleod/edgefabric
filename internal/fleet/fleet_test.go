package fleet_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func newTestService(t *testing.T) (fleet.Service, *sqlite.SQLiteStore) {
	store := newTestStore(t)
	svc := fleet.NewService(store, store, store, store)
	return svc, store
}

func TestCreateNode(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	n, err := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name:     "edge-01",
		Hostname: "edge01.example.com",
		PublicIP:  "203.0.113.1",
		SSHUser:  "root",
		SSHPort:  22,
		Region:   "us-east",
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	if n.Status != domain.NodeStatusPending {
		t.Errorf("expected pending status, got %s", n.Status)
	}
	if n.Name != "edge-01" {
		t.Errorf("expected name edge-01, got %s", n.Name)
	}
}

func TestCreateNodeValidation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name string
		req  fleet.CreateNodeRequest
	}{
		{"missing name", fleet.CreateNodeRequest{Hostname: "h", PublicIP: "1.2.3.4"}},
		{"missing hostname", fleet.CreateNodeRequest{Name: "n", PublicIP: "1.2.3.4"}},
		{"missing public_ip", fleet.CreateNodeRequest{Name: "n", Hostname: "h"}},
		{"invalid public_ip", fleet.CreateNodeRequest{Name: "n", Hostname: "h", PublicIP: "not-an-ip"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateNode(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCreateNodeDefaults(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	n, err := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name:     "edge-02",
		Hostname: "edge02.example.com",
		PublicIP:  "203.0.113.2",
		// SSHUser and SSHPort should default.
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	if n.SSHUser != "root" {
		t.Errorf("expected default SSHUser=root, got %s", n.SSHUser)
	}
	if n.SSHPort != 22 {
		t.Errorf("expected default SSHPort=22, got %d", n.SSHPort)
	}
}

func TestUpdateNode(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	n, _ := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name: "n1", Hostname: "h1.example.com", PublicIP: "1.2.3.4",
	})

	newName := "updated-name"
	newRegion := "eu-west"
	updated, err := svc.UpdateNode(ctx, n.ID, fleet.UpdateNodeRequest{
		Name:   &newName,
		Region: &newRegion,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "updated-name" {
		t.Errorf("expected updated-name, got %s", updated.Name)
	}
	if updated.Region != "eu-west" {
		t.Errorf("expected eu-west, got %s", updated.Region)
	}
}

func TestDeleteNode(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	n, _ := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name: "n1", Hostname: "h1.example.com", PublicIP: "1.2.3.4",
	})

	if err := svc.DeleteNode(ctx, n.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.GetNode(ctx, n.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestNodeHeartbeat(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	n, _ := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name: "n1", Hostname: "h1.example.com", PublicIP: "1.2.3.4",
	})

	if err := svc.RecordNodeHeartbeat(ctx, n.ID); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	got, _ := svc.GetNode(ctx, n.ID)
	if got.LastHeartbeat == nil {
		t.Error("expected last_heartbeat to be set")
	}
}

func TestCreateNodeGroup(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: domain.NewID(), Name: "T1", Slug: "t1"}
	if err := store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	g, err := svc.CreateNodeGroup(ctx, fleet.CreateNodeGroupRequest{
		TenantID:    tenant.ID,
		Name:        "production",
		Description: "Production nodes",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if g.Name != "production" {
		t.Errorf("expected name production, got %s", g.Name)
	}
}

func TestAddRemoveNodeFromGroup(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: domain.NewID(), Name: "T2", Slug: "t2"}
	store.CreateTenant(ctx, tenant)

	n, _ := svc.CreateNode(ctx, fleet.CreateNodeRequest{
		Name: "n1", Hostname: "h1.example.com", PublicIP: "1.2.3.4",
	})

	g, _ := svc.CreateNodeGroup(ctx, fleet.CreateNodeGroupRequest{
		TenantID: tenant.ID, Name: "grp",
	})

	// Add.
	if err := svc.AddNodeToGroup(ctx, g.ID, n.ID); err != nil {
		t.Fatalf("add to group: %v", err)
	}

	// Remove.
	if err := svc.RemoveNodeFromGroup(ctx, g.ID, n.ID); err != nil {
		t.Fatalf("remove from group: %v", err)
	}
}
