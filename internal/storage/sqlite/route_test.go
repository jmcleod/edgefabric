package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func createTestGatewayForRoute(t *testing.T, store interface {
	CreateGateway(ctx context.Context, g *domain.Gateway) error
}, tenantID domain.ID) domain.ID {
	t.Helper()
	gw := &domain.Gateway{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-gw-" + domain.NewID().String()[:8],
		Status:   domain.GatewayStatusOnline,
	}
	if err := store.CreateGateway(context.Background(), gw); err != nil {
		t.Fatalf("create test gateway: %v", err)
	}
	return gw.ID
}

func TestRouteCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gatewayID := createTestGatewayForRoute(t, store, tenantID)

	route := &domain.Route{
		ID:              domain.NewID(),
		TenantID:        tenantID,
		Name:            "web-traffic",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gatewayID,
		DestinationIP:   "10.0.1.100",
		DestinationPort: intPtr(8443),
	}

	// Create.
	if err := store.CreateRoute(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}
	if route.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if route.Status != domain.RouteStatusActive {
		t.Errorf("expected default status active, got %s", route.Status)
	}

	// Get by ID.
	got, err := store.GetRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if got.Name != "web-traffic" {
		t.Errorf("expected name web-traffic, got %s", got.Name)
	}
	if got.TenantID != tenantID {
		t.Errorf("expected tenant_id %s, got %s", tenantID, got.TenantID)
	}
	if got.Protocol != domain.RouteProtocolTCP {
		t.Errorf("expected protocol tcp, got %s", got.Protocol)
	}
	if got.EntryIP != "198.51.100.1" {
		t.Errorf("expected entry_ip 198.51.100.1, got %s", got.EntryIP)
	}
	if got.EntryPort == nil || *got.EntryPort != 443 {
		t.Errorf("expected entry_port 443, got %v", got.EntryPort)
	}
	if got.GatewayID != gatewayID {
		t.Errorf("expected gateway_id %s, got %s", gatewayID, got.GatewayID)
	}
	if got.DestinationIP != "10.0.1.100" {
		t.Errorf("expected destination_ip 10.0.1.100, got %s", got.DestinationIP)
	}
	if got.DestinationPort == nil || *got.DestinationPort != 8443 {
		t.Errorf("expected destination_port 8443, got %v", got.DestinationPort)
	}
	if got.NodeGroupID != nil {
		t.Errorf("expected nil node_group_id, got %v", got.NodeGroupID)
	}

	// Update.
	got.Name = "web-traffic-updated"
	got.Status = domain.RouteStatusDisabled
	if err := store.UpdateRoute(ctx, got); err != nil {
		t.Fatalf("update route: %v", err)
	}
	updated, err := store.GetRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("get updated route: %v", err)
	}
	if updated.Name != "web-traffic-updated" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	if updated.Status != domain.RouteStatusDisabled {
		t.Errorf("expected disabled status, got %s", updated.Status)
	}

	// List.
	routes, total, err := store.ListRoutes(ctx, tenantID, storage.ListParams{Limit: 50})
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}

	// Delete.
	if err := store.DeleteRoute(ctx, route.ID); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	_, err = store.GetRoute(ctx, route.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestRouteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetRoute(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRouteDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteRoute(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRouteNullablePorts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gatewayID := createTestGatewayForRoute(t, store, tenantID)

	// ICMP route: no entry_port or destination_port.
	route := &domain.Route{
		ID:            domain.NewID(),
		TenantID:      tenantID,
		Name:          "icmp-ping",
		Protocol:      domain.RouteProtocolICMP,
		EntryIP:       "198.51.100.1",
		EntryPort:     nil,
		GatewayID:     gatewayID,
		DestinationIP: "10.0.1.100",
		DestinationPort: nil,
	}

	if err := store.CreateRoute(ctx, route); err != nil {
		t.Fatalf("create icmp route: %v", err)
	}

	got, err := store.GetRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("get icmp route: %v", err)
	}
	if got.EntryPort != nil {
		t.Errorf("expected nil entry_port for ICMP, got %v", got.EntryPort)
	}
	if got.DestinationPort != nil {
		t.Errorf("expected nil destination_port for ICMP, got %v", got.DestinationPort)
	}
	if got.Protocol != domain.RouteProtocolICMP {
		t.Errorf("expected protocol icmp, got %s", got.Protocol)
	}
}

func TestListRoutesByGateway(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gw1 := createTestGatewayForRoute(t, store, tenantID)
	gw2 := createTestGatewayForRoute(t, store, tenantID)

	// Create 2 active routes for gw1, 1 active for gw2, 1 disabled for gw1.
	r1 := &domain.Route{
		ID: domain.NewID(), TenantID: tenantID, Name: "alpha",
		Protocol: domain.RouteProtocolTCP, EntryIP: "198.51.100.1",
		EntryPort: intPtr(80), GatewayID: gw1,
		DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
	}
	r2 := &domain.Route{
		ID: domain.NewID(), TenantID: tenantID, Name: "bravo",
		Protocol: domain.RouteProtocolUDP, EntryIP: "198.51.100.1",
		EntryPort: intPtr(53), GatewayID: gw1,
		DestinationIP: "10.0.1.2", DestinationPort: intPtr(53),
	}
	r3 := &domain.Route{
		ID: domain.NewID(), TenantID: tenantID, Name: "charlie",
		Protocol: domain.RouteProtocolTCP, EntryIP: "198.51.100.2",
		EntryPort: intPtr(443), GatewayID: gw2,
		DestinationIP: "10.0.2.1", DestinationPort: intPtr(443),
	}
	r4 := &domain.Route{
		ID: domain.NewID(), TenantID: tenantID, Name: "delta-disabled",
		Protocol: domain.RouteProtocolTCP, EntryIP: "198.51.100.1",
		EntryPort: intPtr(8080), GatewayID: gw1,
		DestinationIP: "10.0.1.3", DestinationPort: intPtr(8080),
		Status: domain.RouteStatusDisabled,
	}

	for _, r := range []*domain.Route{r1, r2, r3, r4} {
		if err := store.CreateRoute(ctx, r); err != nil {
			t.Fatalf("create route %s: %v", r.Name, err)
		}
	}

	// List routes for gw1: should return 2 active (alpha, bravo), not disabled delta.
	gw1Routes, err := store.ListRoutesByGateway(ctx, gw1)
	if err != nil {
		t.Fatalf("list routes by gateway 1: %v", err)
	}
	if len(gw1Routes) != 2 {
		t.Fatalf("expected 2 active routes for gw1, got %d", len(gw1Routes))
	}
	// Sorted by name ASC.
	if gw1Routes[0].Name != "alpha" {
		t.Errorf("expected first route alpha, got %s", gw1Routes[0].Name)
	}
	if gw1Routes[1].Name != "bravo" {
		t.Errorf("expected second route bravo, got %s", gw1Routes[1].Name)
	}

	// List routes for gw2: should return 1 (charlie).
	gw2Routes, err := store.ListRoutesByGateway(ctx, gw2)
	if err != nil {
		t.Fatalf("list routes by gateway 2: %v", err)
	}
	if len(gw2Routes) != 1 {
		t.Fatalf("expected 1 active route for gw2, got %d", len(gw2Routes))
	}
	if gw2Routes[0].Name != "charlie" {
		t.Errorf("expected route charlie, got %s", gw2Routes[0].Name)
	}

	// List routes for non-existent gateway: should return empty.
	emptyRoutes, err := store.ListRoutesByGateway(ctx, domain.NewID())
	if err != nil {
		t.Fatalf("list routes by nonexistent gateway: %v", err)
	}
	if len(emptyRoutes) != 0 {
		t.Errorf("expected 0 routes for nonexistent gateway, got %d", len(emptyRoutes))
	}
}

func TestRouteWithNodeGroup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gatewayID := createTestGatewayForRoute(t, store, tenantID)

	// Create a node group.
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "edge-nodes",
	}
	if err := store.CreateNodeGroup(ctx, group); err != nil {
		t.Fatalf("create node group: %v", err)
	}

	route := &domain.Route{
		ID:              domain.NewID(),
		TenantID:        tenantID,
		Name:            "grouped-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gatewayID,
		DestinationIP:   "10.0.1.100",
		DestinationPort: intPtr(8443),
		NodeGroupID:     &group.ID,
	}

	if err := store.CreateRoute(ctx, route); err != nil {
		t.Fatalf("create route with node group: %v", err)
	}

	got, err := store.GetRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if got.NodeGroupID == nil {
		t.Fatal("expected non-nil node_group_id")
	}
	if *got.NodeGroupID != group.ID {
		t.Errorf("expected node_group_id %s, got %s", group.ID, *got.NodeGroupID)
	}

	// Update to clear node group.
	got.NodeGroupID = nil
	if err := store.UpdateRoute(ctx, got); err != nil {
		t.Fatalf("update route to clear node group: %v", err)
	}
	cleared, err := store.GetRoute(ctx, route.ID)
	if err != nil {
		t.Fatalf("get cleared route: %v", err)
	}
	if cleared.NodeGroupID != nil {
		t.Errorf("expected nil node_group_id after clear, got %v", cleared.NodeGroupID)
	}
}
