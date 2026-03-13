import { lazy, Suspense } from "react";
import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ThemeProvider } from "@/hooks/useTheme";
import { AuthProvider, RequireAuth } from "@/hooks/useAuth";
import { CommandPalette } from "./components/CommandPalette";

// Eagerly loaded — first-paint pages
import LoginPage from "./pages/LoginPage";
import NotFound from "./pages/NotFound";

// Lazy-loaded route pages
const GlobalDashboard = lazy(() => import("./pages/GlobalDashboard"));
const NodesPage = lazy(() => import("./pages/NodesPage"));
const NodeDetailPage = lazy(() => import("./pages/NodeDetailPage"));
const TenantsPage = lazy(() => import("./pages/TenantsPage"));
const TenantDetailPage = lazy(() => import("./pages/TenantDetailPage"));
const GatewaysPage = lazy(() => import("./pages/GatewaysPage"));
const GatewayDetailPage = lazy(() => import("./pages/GatewayDetailPage"));
const UsersPage = lazy(() => import("./pages/UsersPage"));
const NodeGroupsPage = lazy(() => import("./pages/NodeGroupsPage"));
const WireGuardPage = lazy(() => import("./pages/WireGuardPage"));
const BGPPage = lazy(() => import("./pages/BGPPage"));
const IPAllocationsPage = lazy(() => import("./pages/IPAllocationsPage"));
const DNSZonesPage = lazy(() => import("./pages/DNSZonesPage"));
const DNSZoneDetailPage = lazy(() => import("./pages/DNSZoneDetailPage"));
const CDNServicesPage = lazy(() => import("./pages/CDNServicesPage"));
const CDNSiteDetailPage = lazy(() => import("./pages/CDNSiteDetailPage"));
const RoutesPage = lazy(() => import("./pages/RoutesPage"));
const RouteDetailPage = lazy(() => import("./pages/RouteDetailPage"));
const ProvisioningJobsPage = lazy(() => import("./pages/ProvisioningJobsPage"));
const SSHKeysPage = lazy(() => import("./pages/SSHKeysPage"));
const APIKeysPage = lazy(() => import("./pages/APIKeysPage"));
const AuditLogsPage = lazy(() => import("./pages/AuditLogsPage"));
const ProfilePage = lazy(() => import("./pages/ProfilePage"));
const FleetHealthPage = lazy(() => import("./pages/FleetHealthPage"));
const SettingsPage = lazy(() => import("./pages/SettingsPage"));
const TenantDashboardPage = lazy(() => import("./pages/tenant/TenantDashboardPage"));
const TenantNodesPage = lazy(() => import("./pages/tenant/TenantNodesPage"));
const TenantNodeGroupsPage = lazy(() => import("./pages/tenant/TenantNodeGroupsPage"));
const TenantDNSRecordsPage = lazy(() => import("./pages/tenant/TenantDNSRecordsPage"));
const TenantCDNDomainsPage = lazy(() => import("./pages/tenant/TenantCDNDomainsPage"));
const TenantCDNOriginsPage = lazy(() => import("./pages/tenant/TenantCDNOriginsPage"));
const TenantCDNCachePage = lazy(() => import("./pages/tenant/TenantCDNCachePage"));
const TenantAuditPage = lazy(() => import("./pages/tenant/TenantAuditPage"));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,       // Consider data fresh for 30s
      refetchOnWindowFocus: true,
    },
  },
});

function RouteLoadingFallback() {
  return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
    </div>
  );
}

const App = () => (
  <ThemeProvider>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter>
          <AuthProvider>
            <Suspense fallback={<RouteLoadingFallback />}>
              <Routes>
                {/* Public route */}
                <Route path="/login" element={<LoginPage />} />

                {/* Superuser routes */}
                <Route path="/" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
                <Route path="/nodes" element={<RequireAuth><NodesPage /></RequireAuth>} />
                <Route path="/nodes/:id" element={<RequireAuth><NodeDetailPage /></RequireAuth>} />
                <Route path="/tenants" element={<RequireAuth><TenantsPage /></RequireAuth>} />
                <Route path="/tenants/:id" element={<RequireAuth><TenantDetailPage /></RequireAuth>} />
                <Route path="/gateways" element={<RequireAuth><GatewaysPage /></RequireAuth>} />
                <Route path="/gateways/:id" element={<RequireAuth><GatewayDetailPage /></RequireAuth>} />
                <Route path="/users" element={<RequireAuth><UsersPage /></RequireAuth>} />
                <Route path="/node-groups" element={<RequireAuth><NodeGroupsPage /></RequireAuth>} />
                <Route path="/networking/wireguard" element={<RequireAuth><WireGuardPage /></RequireAuth>} />
                <Route path="/networking/bgp" element={<RequireAuth><BGPPage /></RequireAuth>} />
                <Route path="/networking/ips" element={<RequireAuth><IPAllocationsPage /></RequireAuth>} />
                <Route path="/jobs" element={<RequireAuth><ProvisioningJobsPage /></RequireAuth>} />
                <Route path="/ssh-keys" element={<RequireAuth><SSHKeysPage /></RequireAuth>} />
                <Route path="/audit" element={<RequireAuth><AuditLogsPage /></RequireAuth>} />
                <Route path="/fleet-health" element={<RequireAuth><FleetHealthPage /></RequireAuth>} />
                <Route path="/settings" element={<RequireAuth><SettingsPage /></RequireAuth>} />
                <Route path="/profile" element={<RequireAuth><ProfilePage /></RequireAuth>} />

                {/* Tenant-scoped routes */}
                <Route path="/tenant/dashboard" element={<RequireAuth><TenantDashboardPage /></RequireAuth>} />
                <Route path="/tenant/nodes" element={<RequireAuth><TenantNodesPage /></RequireAuth>} />
                <Route path="/tenant/node-groups" element={<RequireAuth><TenantNodeGroupsPage /></RequireAuth>} />
                <Route path="/tenant/dns/zones" element={<RequireAuth><DNSZonesPage /></RequireAuth>} />
                <Route path="/tenant/dns/zones/:id" element={<RequireAuth><DNSZoneDetailPage /></RequireAuth>} />
                <Route path="/tenant/dns/records" element={<RequireAuth><TenantDNSRecordsPage /></RequireAuth>} />
                <Route path="/tenant/cdn/services" element={<RequireAuth><CDNServicesPage /></RequireAuth>} />
                <Route path="/tenant/cdn/services/:id" element={<RequireAuth><CDNSiteDetailPage /></RequireAuth>} />
                <Route path="/tenant/cdn/domains" element={<RequireAuth><TenantCDNDomainsPage /></RequireAuth>} />
                <Route path="/tenant/cdn/origins" element={<RequireAuth><TenantCDNOriginsPage /></RequireAuth>} />
                <Route path="/tenant/cdn/cache" element={<RequireAuth><TenantCDNCachePage /></RequireAuth>} />
                <Route path="/tenant/routes" element={<RequireAuth><RoutesPage /></RequireAuth>} />
                <Route path="/tenant/routes/:id" element={<RequireAuth><RouteDetailPage /></RequireAuth>} />
                <Route path="/tenant/api-keys" element={<RequireAuth><APIKeysPage /></RequireAuth>} />
                <Route path="/tenant/audit" element={<RequireAuth><TenantAuditPage /></RequireAuth>} />

                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
            <CommandPalette />
          </AuthProvider>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  </ThemeProvider>
);

export default App;
