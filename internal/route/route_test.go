package route_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

func newTestEnv(t *testing.T) (route.Service, *sqlite.SQLiteStore) {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := route.NewService(store, store, store, store)
	return svc, store
}

func createTestTenant(t *testing.T, store *sqlite.SQLiteStore) domain.ID {
	t.Helper()
	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "test-tenant-" + domain.NewID().String()[:8],
		Slug: "test-" + domain.NewID().String()[:8],
	}
	if err := store.CreateTenant(context.Background(), tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant.ID
}

func createTestGateway(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID, wireGuardIP string) domain.ID {
	t.Helper()
	gw := &domain.Gateway{
		ID:          domain.NewID(),
		TenantID:    tenantID,
		Name:        "test-gw-" + domain.NewID().String()[:8],
		WireGuardIP: wireGuardIP,
		Status:      domain.GatewayStatusOnline,
	}
	if err := store.CreateGateway(context.Background(), gw); err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	return gw.ID
}

func createTestNodeGroup(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	group := &domain.NodeGroup{
		ID:       domain.NewID(),
		TenantID: tenantID,
		Name:     "test-group-" + domain.NewID().String()[:8],
	}
	if err := store.CreateNodeGroup(context.Background(), group); err != nil {
		t.Fatalf("create node group: %v", err)
	}
	return group.ID
}

func createTestNode(t *testing.T, store *sqlite.SQLiteStore, tenantID domain.ID) domain.ID {
	t.Helper()
	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: &tenantID,
		Name:     "test-node-" + domain.NewID().String()[:8],
		Hostname: "node.example.com",
		PublicIP: "203.0.113.1",
		SSHPort:  22,
		SSHUser:  "root",
		Status:   domain.NodeStatusOnline,
	}
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.ID
}

func intPtr(v int) *int { return &v }

func TestCreateRoute_HappyPath(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "web-traffic",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if r.Name != "web-traffic" {
		t.Errorf("expected name web-traffic, got %s", r.Name)
	}
	if r.Status != domain.RouteStatusActive {
		t.Errorf("expected active status, got %s", r.Status)
	}
	if r.Protocol != domain.RouteProtocolTCP {
		t.Errorf("expected tcp protocol, got %s", r.Protocol)
	}
	if r.EntryPort == nil || *r.EntryPort != 443 {
		t.Errorf("expected entry port 443, got %v", r.EntryPort)
	}
	if r.DestinationPort == nil || *r.DestinationPort != 8443 {
		t.Errorf("expected destination port 8443, got %v", r.DestinationPort)
	}
	if r.GatewayID != gwID {
		t.Errorf("expected gateway_id %s, got %s", gwID, r.GatewayID)
	}

	// Verify via Get.
	got, err := svc.GetRoute(ctx, r.ID)
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if got.Name != "web-traffic" {
		t.Errorf("expected name web-traffic, got %s", got.Name)
	}
}

func TestCreateRoute_GatewayNotFound(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)

	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "bad-gateway",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       domain.NewID(), // Nonexistent gateway.
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent gateway")
	}
}

func TestCreateRoute_ValidationError(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "", // Invalid: empty name.
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
	})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestUpdateRoute(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "original",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	newName := "updated"
	newPort := 8080
	updated, err := svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{
		Name:            &newName,
		DestinationPort: &newPort,
	})
	if err != nil {
		t.Fatalf("update route: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name updated, got %s", updated.Name)
	}
	if updated.DestinationPort == nil || *updated.DestinationPort != 8080 {
		t.Errorf("expected destination port 8080, got %v", updated.DestinationPort)
	}
	// Protocol should not have changed.
	if updated.Protocol != domain.RouteProtocolTCP {
		t.Errorf("expected tcp unchanged, got %s", updated.Protocol)
	}
}

func TestUpdateRoute_ClearNodeGroup(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")
	groupID := createTestNodeGroup(t, store, tenantID)

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "grouped-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
		NodeGroupID:     &groupID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if r.NodeGroupID == nil || *r.NodeGroupID != groupID {
		t.Fatal("expected node group to be set after create")
	}

	// Clear node group.
	updated, err := svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{
		ClearNodeGroup: true,
	})
	if err != nil {
		t.Fatalf("update route: %v", err)
	}
	if updated.NodeGroupID != nil {
		t.Errorf("expected node group to be cleared, got %v", updated.NodeGroupID)
	}
}

func TestDeleteRoute(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "to-delete",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	if err := svc.DeleteRoute(ctx, r.ID); err != nil {
		t.Fatalf("delete route: %v", err)
	}

	_, err = svc.GetRoute(ctx, r.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListRoutes(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	for i := 0; i < 3; i++ {
		_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
			TenantID:        tenantID,
			Name:            "route-" + domain.NewID().String()[:8],
			Protocol:        domain.RouteProtocolTCP,
			EntryIP:         "198.51.100.1",
			EntryPort:       intPtr(8000 + i),
			GatewayID:       gwID,
			DestinationIP:   "10.0.1.1",
			DestinationPort: intPtr(9000 + i),
		})
		if err != nil {
			t.Fatalf("create route %d: %v", i, err)
		}
	}

	routes, total, err := svc.ListRoutes(ctx, tenantID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	if total != 3 || len(routes) != 3 {
		t.Errorf("expected 3 routes, got total=%d len=%d", total, len(routes))
	}
}

func TestGetNodeRouteConfig(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	// Add node to group.
	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create a route assigned to the group.
	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "config-test-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
		NodeGroupID:     &groupID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Get node route config.
	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 1 {
		t.Fatalf("expected 1 route in config, got %d", len(config.Routes))
	}
	if config.Routes[0].Route.ID != r.ID {
		t.Errorf("expected route ID %s, got %s", r.ID, config.Routes[0].Route.ID)
	}
	if config.Routes[0].GatewayWGIP != "10.100.0.5" {
		t.Errorf("expected gateway WG IP 10.100.0.5, got %s", config.Routes[0].GatewayWGIP)
	}
}

func TestGetNodeRouteConfig_NoRoutesForUnassignedNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	// Create route assigned to group, but don't add node to group.
	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "unassigned-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
		NodeGroupID:     &groupID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 0 {
		t.Errorf("expected 0 routes for unassigned node, got %d", len(config.Routes))
	}
}

func TestGetNodeRouteConfig_DisabledRouteExcluded(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "will-disable",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
		NodeGroupID:     &groupID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Disable the route.
	disabled := domain.RouteStatusDisabled
	_, err = svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{Status: &disabled})
	if err != nil {
		t.Fatalf("disable route: %v", err)
	}

	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 0 {
		t.Errorf("expected 0 routes (disabled excluded), got %d", len(config.Routes))
	}
}

func TestGetGatewayRouteConfig(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	// Create two routes for this gateway.
	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "gw-route-1",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
	})
	if err != nil {
		t.Fatalf("create route 1: %v", err)
	}
	_, err = svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "gw-route-2",
		Protocol:        domain.RouteProtocolUDP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(53),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.2",
		DestinationPort: intPtr(53),
	})
	if err != nil {
		t.Fatalf("create route 2: %v", err)
	}

	config, err := svc.GetGatewayRouteConfig(ctx, gwID)
	if err != nil {
		t.Fatalf("get gateway route config: %v", err)
	}
	if len(config.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(config.Routes))
	}
}

func TestGetGatewayRouteConfig_GatewayNotFound(t *testing.T) {
	svc, _ := newTestEnv(t)
	ctx := context.Background()

	_, err := svc.GetGatewayRouteConfig(ctx, domain.NewID())
	if err == nil {
		t.Fatal("expected error for nonexistent gateway")
	}
}

func TestGetGatewayRouteConfig_ExcludesDisabledRoutes(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "will-disable",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Disable the route.
	disabled := domain.RouteStatusDisabled
	_, err = svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{Status: &disabled})
	if err != nil {
		t.Fatalf("disable route: %v", err)
	}

	config, err := svc.GetGatewayRouteConfig(ctx, gwID)
	if err != nil {
		t.Fatalf("get gateway route config: %v", err)
	}
	if len(config.Routes) != 0 {
		t.Errorf("expected 0 routes (disabled excluded), got %d", len(config.Routes))
	}
}
