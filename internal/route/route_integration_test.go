package route_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// TestRouteIntegration_FullLifecycle exercises the full route lifecycle through
// the service layer: create, read, config sync, update, disable, re-enable, delete.
func TestRouteIntegration_FullLifecycle(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// Step 1: Create tenant, node, gateway (with WireGuardIP), node group, add node to group.
	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Step 2: Create route assigned to group.
	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "full-lifecycle-route",
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
	if r.Status != domain.RouteStatusActive {
		t.Errorf("expected active status, got %s", r.Status)
	}

	// Step 3: Get node route config → verify route present with gateway WG IP.
	nodeConfig, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(nodeConfig.Routes) != 1 {
		t.Fatalf("expected 1 route in node config, got %d", len(nodeConfig.Routes))
	}
	if nodeConfig.Routes[0].Route.Name != "full-lifecycle-route" {
		t.Errorf("expected route name full-lifecycle-route, got %s", nodeConfig.Routes[0].Route.Name)
	}
	if nodeConfig.Routes[0].GatewayWGIP != "10.100.0.5" {
		t.Errorf("expected gateway WG IP 10.100.0.5, got %s", nodeConfig.Routes[0].GatewayWGIP)
	}

	// Step 4: Get gateway route config → verify route present.
	gwConfig, err := svc.GetGatewayRouteConfig(ctx, gwID)
	if err != nil {
		t.Fatalf("get gateway route config: %v", err)
	}
	if len(gwConfig.Routes) != 1 {
		t.Fatalf("expected 1 route in gateway config, got %d", len(gwConfig.Routes))
	}
	if gwConfig.Routes[0].Name != "full-lifecycle-route" {
		t.Errorf("expected route name, got %s", gwConfig.Routes[0].Name)
	}

	// Step 5: Update route (change name, destination, status to disabled).
	newName := "updated-lifecycle-route"
	newDestIP := "10.0.2.1"
	newDestPort := 9443
	disabled := domain.RouteStatusDisabled
	updated, err := svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{
		Name:            &newName,
		DestinationIP:   &newDestIP,
		DestinationPort: &newDestPort,
		Status:          &disabled,
	})
	if err != nil {
		t.Fatalf("update route: %v", err)
	}
	if updated.Name != "updated-lifecycle-route" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	if updated.DestinationIP != "10.0.2.1" {
		t.Errorf("expected updated destination IP, got %s", updated.DestinationIP)
	}
	if updated.Status != domain.RouteStatusDisabled {
		t.Errorf("expected disabled status, got %s", updated.Status)
	}

	// Verify disabled route is not in configs.
	nodeConfig2, _ := svc.GetNodeRouteConfig(ctx, nodeID)
	if len(nodeConfig2.Routes) != 0 {
		t.Errorf("expected 0 routes in node config (disabled), got %d", len(nodeConfig2.Routes))
	}
	gwConfig2, _ := svc.GetGatewayRouteConfig(ctx, gwID)
	if len(gwConfig2.Routes) != 0 {
		t.Errorf("expected 0 routes in gateway config (disabled), got %d", len(gwConfig2.Routes))
	}

	// Step 6: Re-enable route → verify back in config.
	active := domain.RouteStatusActive
	_, err = svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{Status: &active})
	if err != nil {
		t.Fatalf("re-enable route: %v", err)
	}

	nodeConfig3, _ := svc.GetNodeRouteConfig(ctx, nodeID)
	if len(nodeConfig3.Routes) != 1 {
		t.Errorf("expected 1 route after re-enable, got %d", len(nodeConfig3.Routes))
	}
	gwConfig3, _ := svc.GetGatewayRouteConfig(ctx, gwID)
	if len(gwConfig3.Routes) != 1 {
		t.Errorf("expected 1 route in gateway config after re-enable, got %d", len(gwConfig3.Routes))
	}

	// Step 7: Delete route → verify removed from both configs.
	if err := svc.DeleteRoute(ctx, r.ID); err != nil {
		t.Fatalf("delete route: %v", err)
	}

	_, err = svc.GetRoute(ctx, r.ID)
	if err == nil {
		t.Error("expected error getting deleted route")
	}

	nodeConfig4, _ := svc.GetNodeRouteConfig(ctx, nodeID)
	if len(nodeConfig4.Routes) != 0 {
		t.Errorf("expected 0 routes after delete, got %d", len(nodeConfig4.Routes))
	}
	gwConfig4, _ := svc.GetGatewayRouteConfig(ctx, gwID)
	if len(gwConfig4.Routes) != 0 {
		t.Errorf("expected 0 routes in gateway config after delete, got %d", len(gwConfig4.Routes))
	}
}

// TestRouteIntegration_ValidationEdgeCases tests various validation failures.
func TestRouteIntegration_ValidationEdgeCases(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	tests := []struct {
		name string
		req  route.CreateRouteRequest
	}{
		{
			"bad protocol",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "bad-proto", Protocol: "ftp",
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"missing port for TCP",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "no-port", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1",
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"port set for ICMP",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "icmp-port", Protocol: domain.RouteProtocolICMP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: gwID, DestinationIP: "10.0.1.1",
			},
		},
		{
			"invalid entry IP",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "bad-ip", Protocol: domain.RouteProtocolTCP,
				EntryIP: "not-an-ip", EntryPort: intPtr(80),
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"port out of range (high)",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "big-port", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(70000),
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"port out of range (zero)",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "zero-port", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(0),
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"empty name",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: gwID, DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
		},
		{
			"invalid destination IP",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "bad-dest", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: gwID, DestinationIP: "invalid", DestinationPort: intPtr(80),
			},
		},
		{
			"missing destination port for UDP",
			route.CreateRouteRequest{
				TenantID: tenantID, Name: "no-dest-port", Protocol: domain.RouteProtocolUDP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(53),
				GatewayID: gwID, DestinationIP: "10.0.1.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateRoute(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestRouteIntegration_TenantIsolation verifies that routes from one tenant
// are not visible in the config for another tenant's nodes.
func TestRouteIntegration_TenantIsolation(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	// Tenant A: create node, gateway, group, route.
	tenantA := createTestTenant(t, store)
	nodeA := createTestNode(t, store, tenantA)
	groupA := createTestNodeGroup(t, store, tenantA)
	gwA := createTestGateway(t, store, tenantA, "10.100.0.10")
	if err := store.AddNodeToGroup(ctx, groupA, nodeA); err != nil {
		t.Fatalf("add node A to group: %v", err)
	}
	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantA,
		Name:            "tenant-a-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwA,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
		NodeGroupID:     &groupA,
	})
	if err != nil {
		t.Fatalf("create route A: %v", err)
	}

	// Tenant B: create node, gateway, group, route.
	tenantB := createTestTenant(t, store)
	nodeB := createTestNode(t, store, tenantB)
	groupB := createTestNodeGroup(t, store, tenantB)
	gwB := createTestGateway(t, store, tenantB, "10.100.0.20")
	if err := store.AddNodeToGroup(ctx, groupB, nodeB); err != nil {
		t.Fatalf("add node B to group: %v", err)
	}
	_, err = svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantB,
		Name:            "tenant-b-route",
		Protocol:        domain.RouteProtocolUDP,
		EntryIP:         "198.51.100.2",
		EntryPort:       intPtr(53),
		GatewayID:       gwB,
		DestinationIP:   "10.0.2.1",
		DestinationPort: intPtr(53),
		NodeGroupID:     &groupB,
	})
	if err != nil {
		t.Fatalf("create route B: %v", err)
	}

	// Node A should only see tenant A's route.
	configA, err := svc.GetNodeRouteConfig(ctx, nodeA)
	if err != nil {
		t.Fatalf("get node A config: %v", err)
	}
	if len(configA.Routes) != 1 {
		t.Fatalf("expected 1 route for node A, got %d", len(configA.Routes))
	}
	if configA.Routes[0].Route.Name != "tenant-a-route" {
		t.Errorf("expected tenant-a-route, got %s", configA.Routes[0].Route.Name)
	}

	// Node B should only see tenant B's route.
	configB, err := svc.GetNodeRouteConfig(ctx, nodeB)
	if err != nil {
		t.Fatalf("get node B config: %v", err)
	}
	if len(configB.Routes) != 1 {
		t.Fatalf("expected 1 route for node B, got %d", len(configB.Routes))
	}
	if configB.Routes[0].Route.Name != "tenant-b-route" {
		t.Errorf("expected tenant-b-route, got %s", configB.Routes[0].Route.Name)
	}

	// Gateway A should only see tenant A's route.
	gwConfigA, err := svc.GetGatewayRouteConfig(ctx, gwA)
	if err != nil {
		t.Fatalf("get gateway A config: %v", err)
	}
	if len(gwConfigA.Routes) != 1 {
		t.Fatalf("expected 1 route for gateway A, got %d", len(gwConfigA.Routes))
	}

	// Gateway B should only see tenant B's route.
	gwConfigB, err := svc.GetGatewayRouteConfig(ctx, gwB)
	if err != nil {
		t.Fatalf("get gateway B config: %v", err)
	}
	if len(gwConfigB.Routes) != 1 {
		t.Fatalf("expected 1 route for gateway B, got %d", len(gwConfigB.Routes))
	}
}

// TestRouteIntegration_RouteWithoutNodeGroup verifies that a route without
// a NodeGroupID doesn't appear in any node's config.
func TestRouteIntegration_RouteWithoutNodeGroup(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create route without NodeGroupID.
	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "ungrouped-route",
		Protocol:        domain.RouteProtocolTCP,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(80),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(80),
		// No NodeGroupID.
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Node should see no routes.
	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 0 {
		t.Errorf("expected 0 routes for ungrouped route, got %d", len(config.Routes))
	}
}

// TestRouteIntegration_MultipleRoutesPerNode verifies that a node sees
// multiple routes from different protocols and gateways.
func TestRouteIntegration_MultipleRoutesPerNode(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID1 := createTestGateway(t, store, tenantID, "10.100.0.5")
	gwID2 := createTestGateway(t, store, tenantID, "10.100.0.6")

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// TCP route via gateway 1.
	_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID: tenantID, Name: "tcp-route",
		Protocol: domain.RouteProtocolTCP,
		EntryIP: "198.51.100.1", EntryPort: intPtr(443),
		GatewayID: gwID1, DestinationIP: "10.0.1.1", DestinationPort: intPtr(8443),
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create TCP route: %v", err)
	}

	// UDP route via gateway 2.
	_, err = svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID: tenantID, Name: "udp-route",
		Protocol: domain.RouteProtocolUDP,
		EntryIP: "198.51.100.1", EntryPort: intPtr(53),
		GatewayID: gwID2, DestinationIP: "10.0.2.1", DestinationPort: intPtr(53),
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create UDP route: %v", err)
	}

	// ICMP route (no ports).
	_, err = svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID: tenantID, Name: "icmp-route",
		Protocol:  domain.RouteProtocolICMP,
		EntryIP:   "198.51.100.1",
		GatewayID: gwID1, DestinationIP: "10.0.3.1",
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create ICMP route: %v", err)
	}

	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(config.Routes))
	}

	// Verify different gateway WG IPs.
	gwIPs := make(map[string]bool)
	for _, rwg := range config.Routes {
		gwIPs[rwg.GatewayWGIP] = true
	}
	if !gwIPs["10.100.0.5"] || !gwIPs["10.100.0.6"] {
		t.Errorf("expected both gateway IPs in config, got %v", gwIPs)
	}
}

// TestRouteIntegration_ClearNodeGroup_RemovesFromConfig verifies that clearing
// a route's node group removes it from node config.
func TestRouteIntegration_ClearNodeGroup_RemovesFromConfig(t *testing.T) {
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
		Name:            "clear-group-route",
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

	// Verify route is in config.
	config, _ := svc.GetNodeRouteConfig(ctx, nodeID)
	if len(config.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(config.Routes))
	}

	// Clear node group.
	_, err = svc.UpdateRoute(ctx, r.ID, route.UpdateRouteRequest{ClearNodeGroup: true})
	if err != nil {
		t.Fatalf("clear node group: %v", err)
	}

	// Route should no longer be in config.
	config2, _ := svc.GetNodeRouteConfig(ctx, nodeID)
	if len(config2.Routes) != 0 {
		t.Errorf("expected 0 routes after clearing group, got %d", len(config2.Routes))
	}

	// But route still exists in gateway config (gateway doesn't depend on node groups).
	gwConfig, _ := svc.GetGatewayRouteConfig(ctx, gwID)
	if len(gwConfig.Routes) != 1 {
		t.Errorf("expected 1 route in gateway config (unaffected by group clear), got %d", len(gwConfig.Routes))
	}
}

// TestRouteIntegration_Pagination tests list pagination for routes.
func TestRouteIntegration_Pagination(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	for i := 0; i < 5; i++ {
		_, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
			TenantID:        tenantID,
			Name:            "page-route-" + domain.NewID().String()[:8],
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

	// Page 1: limit 2.
	routes, total, err := svc.ListRoutes(ctx, tenantID, storage.ListParams{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list routes page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(routes) != 2 {
		t.Errorf("expected 2 routes on page 1, got %d", len(routes))
	}

	// Page 2: offset 2, limit 2.
	routes2, total2, err := svc.ListRoutes(ctx, tenantID, storage.ListParams{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list routes page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("expected total 5, got %d", total2)
	}
	if len(routes2) != 2 {
		t.Errorf("expected 2 routes on page 2, got %d", len(routes2))
	}

	// No overlap between pages.
	if routes[0].ID == routes2[0].ID {
		t.Error("pages should not overlap")
	}

	// Last page.
	routes3, _, err := svc.ListRoutes(ctx, tenantID, storage.ListParams{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("list routes page 3: %v", err)
	}
	if len(routes3) != 1 {
		t.Errorf("expected 1 route on last page, got %d", len(routes3))
	}
}

// TestRouteIntegration_ICMPRouteCreateAndConfig verifies ICMP routes (no ports)
// work through the full flow.
func TestRouteIntegration_ICMPRouteCreateAndConfig(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, tenantID)
	groupID := createTestNodeGroup(t, store, tenantID)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	if err := store.AddNodeToGroup(ctx, groupID, nodeID); err != nil {
		t.Fatalf("add node to group: %v", err)
	}

	// Create ICMP route — no ports.
	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:    tenantID,
		Name:        "icmp-ping-route",
		Protocol:    domain.RouteProtocolICMP,
		EntryIP:     "198.51.100.1",
		GatewayID:   gwID,
		DestinationIP: "10.0.1.1",
		NodeGroupID: &groupID,
	})
	if err != nil {
		t.Fatalf("create icmp route: %v", err)
	}
	if r.EntryPort != nil {
		t.Errorf("expected nil entry port for ICMP, got %v", r.EntryPort)
	}
	if r.DestinationPort != nil {
		t.Errorf("expected nil destination port for ICMP, got %v", r.DestinationPort)
	}

	// Verify in node config.
	config, err := svc.GetNodeRouteConfig(ctx, nodeID)
	if err != nil {
		t.Fatalf("get node route config: %v", err)
	}
	if len(config.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(config.Routes))
	}
	if config.Routes[0].Route.Protocol != domain.RouteProtocolICMP {
		t.Errorf("expected ICMP protocol, got %s", config.Routes[0].Route.Protocol)
	}
}

// TestRouteIntegration_AllProtocolRoute verifies the "all" protocol creates routes
// correctly.
func TestRouteIntegration_AllProtocolRoute(t *testing.T) {
	svc, store := newTestEnv(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	gwID := createTestGateway(t, store, tenantID, "10.100.0.5")

	r, err := svc.CreateRoute(ctx, route.CreateRouteRequest{
		TenantID:        tenantID,
		Name:            "all-protocol-route",
		Protocol:        domain.RouteProtocolAll,
		EntryIP:         "198.51.100.1",
		EntryPort:       intPtr(443),
		GatewayID:       gwID,
		DestinationIP:   "10.0.1.1",
		DestinationPort: intPtr(8443),
	})
	if err != nil {
		t.Fatalf("create all-protocol route: %v", err)
	}
	if r.Protocol != domain.RouteProtocolAll {
		t.Errorf("expected all protocol, got %s", r.Protocol)
	}
}

// TestRouteIntegration_DeleteRouteNotFound verifies error on deleting nonexistent route.
func TestRouteIntegration_DeleteRouteNotFound(t *testing.T) {
	svc, _ := newTestEnv(t)
	ctx := context.Background()

	if err := svc.DeleteRoute(ctx, domain.NewID()); err == nil {
		t.Error("expected error deleting nonexistent route")
	}
}

// TestRouteIntegration_GetRouteNotFound verifies error on getting nonexistent route.
func TestRouteIntegration_GetRouteNotFound(t *testing.T) {
	svc, _ := newTestEnv(t)
	ctx := context.Background()

	_, err := svc.GetRoute(ctx, domain.NewID())
	if err == nil {
		t.Error("expected error getting nonexistent route")
	}
}
