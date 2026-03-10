import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ThemeProvider } from "@/hooks/useTheme";
import { AuthProvider, RequireAuth } from "@/hooks/useAuth";
import LoginPage from "./pages/LoginPage";
import GlobalDashboard from "./pages/GlobalDashboard";
import NodesPage from "./pages/NodesPage";
import NodeDetailPage from "./pages/NodeDetailPage";
import TenantsPage from "./pages/TenantsPage";
import TenantDetailPage from "./pages/TenantDetailPage";
import GatewaysPage from "./pages/GatewaysPage";
import GatewayDetailPage from "./pages/GatewayDetailPage";
import UsersPage from "./pages/UsersPage";
import NodeGroupsPage from "./pages/NodeGroupsPage";
import WireGuardPage from "./pages/WireGuardPage";
import BGPPage from "./pages/BGPPage";
import IPAllocationsPage from "./pages/IPAllocationsPage";
import DNSZonesPage from "./pages/DNSZonesPage";
import DNSZoneDetailPage from "./pages/DNSZoneDetailPage";
import CDNServicesPage from "./pages/CDNServicesPage";
import CDNSiteDetailPage from "./pages/CDNSiteDetailPage";
import RoutesPage from "./pages/RoutesPage";
import RouteDetailPage from "./pages/RouteDetailPage";
import ProvisioningJobsPage from "./pages/ProvisioningJobsPage";
import SSHKeysPage from "./pages/SSHKeysPage";
import APIKeysPage from "./pages/APIKeysPage";
import AuditLogsPage from "./pages/AuditLogsPage";
import ProfilePage from "./pages/ProfilePage";
import FleetHealthPage from "./pages/FleetHealthPage";
import SettingsPage from "./pages/SettingsPage";
import { CommandPalette } from "./components/CommandPalette";
import TenantDashboardPage from "./pages/tenant/TenantDashboardPage";
import TenantNodesPage from "./pages/tenant/TenantNodesPage";
import TenantNodeGroupsPage from "./pages/tenant/TenantNodeGroupsPage";
import TenantDNSRecordsPage from "./pages/tenant/TenantDNSRecordsPage";
import TenantCDNDomainsPage from "./pages/tenant/TenantCDNDomainsPage";
import TenantCDNOriginsPage from "./pages/tenant/TenantCDNOriginsPage";
import TenantCDNCachePage from "./pages/tenant/TenantCDNCachePage";
import TenantAuditPage from "./pages/tenant/TenantAuditPage";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,       // Consider data fresh for 30s
      refetchOnWindowFocus: true,
    },
  },
});

const App = () => (
  <ThemeProvider>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter>
          <AuthProvider>
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
            <CommandPalette />
          </AuthProvider>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  </ThemeProvider>
);

export default App;
