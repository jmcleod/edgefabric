package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestGatewayCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	gw := &domain.Gateway{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "gateway-east-1",
		PublicIP: "198.51.100.1",
	}

	// Create.
	if err := store.CreateGateway(ctx, gw); err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	if gw.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if gw.Status != domain.GatewayStatusPending {
		t.Errorf("expected default status pending, got %s", gw.Status)
	}

	// Get.
	got, err := store.GetGateway(ctx, gw.ID)
	if err != nil {
		t.Fatalf("get gateway: %v", err)
	}
	if got.Name != "gateway-east-1" {
		t.Errorf("expected name gateway-east-1, got %s", got.Name)
	}
	if got.PublicIP != "198.51.100.1" {
		t.Errorf("expected public IP 198.51.100.1, got %s", got.PublicIP)
	}

	// List by tenant.
	gateways, total, err := store.ListGateways(ctx, &tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list gateways: %v", err)
	}
	if total != 1 || len(gateways) != 1 {
		t.Errorf("expected 1 gateway, got total=%d len=%d", total, len(gateways))
	}

	// Update.
	got.Name = "gateway-east-1-updated"
	got.Status = domain.GatewayStatusOnline
	got.WireGuardIP = "10.100.0.100"
	if err := store.UpdateGateway(ctx, got); err != nil {
		t.Fatalf("update gateway: %v", err)
	}

	updated, err := store.GetGateway(ctx, gw.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Name != "gateway-east-1-updated" {
		t.Errorf("expected gateway-east-1-updated, got %s", updated.Name)
	}
	if updated.Status != domain.GatewayStatusOnline {
		t.Errorf("expected online status, got %s", updated.Status)
	}
	if updated.WireGuardIP != "10.100.0.100" {
		t.Errorf("expected wireguard IP 10.100.0.100, got %s", updated.WireGuardIP)
	}

	// Heartbeat.
	if err := store.UpdateGatewayHeartbeat(ctx, gw.ID); err != nil {
		t.Fatalf("update gateway heartbeat: %v", err)
	}
	afterHB, err := store.GetGateway(ctx, gw.ID)
	if err != nil {
		t.Fatalf("get after heartbeat: %v", err)
	}
	if afterHB.LastHeartbeat == nil {
		t.Error("expected last_heartbeat to be set after heartbeat")
	}
	if afterHB.Status != domain.GatewayStatusOnline {
		t.Errorf("expected online status after heartbeat, got %s", afterHB.Status)
	}

	// Delete.
	if err := store.DeleteGateway(ctx, gw.ID); err != nil {
		t.Fatalf("delete gateway: %v", err)
	}
	_, err = store.GetGateway(ctx, gw.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGatewayNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetGateway(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGatewayListEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	gateways, total, err := store.ListGateways(ctx, &tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list gateways: %v", err)
	}
	if total != 0 || len(gateways) != 0 {
		t.Errorf("expected 0 gateways, got total=%d len=%d", total, len(gateways))
	}
}
