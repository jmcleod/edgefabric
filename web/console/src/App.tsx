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
import GatewaysPage from "./pages/GatewaysPage";
import DNSZonesPage from "./pages/DNSZonesPage";
import CDNServicesPage from "./pages/CDNServicesPage";
import ProvisioningJobsPage from "./pages/ProvisioningJobsPage";
import AuditLogsPage from "./pages/AuditLogsPage";
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

              {/* Protected routes */}
              <Route path="/" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/nodes" element={<RequireAuth><NodesPage /></RequireAuth>} />
              <Route path="/nodes/:id" element={<RequireAuth><NodeDetailPage /></RequireAuth>} />
              <Route path="/tenants" element={<RequireAuth><TenantsPage /></RequireAuth>} />
              <Route path="/gateways" element={<RequireAuth><GatewaysPage /></RequireAuth>} />
              <Route path="/jobs" element={<RequireAuth><ProvisioningJobsPage /></RequireAuth>} />
              <Route path="/audit" element={<RequireAuth><AuditLogsPage /></RequireAuth>} />

              {/* Tenant-scoped routes */}
              <Route path="/tenant/dns/zones" element={<RequireAuth><DNSZonesPage /></RequireAuth>} />
              <Route path="/tenant/cdn/services" element={<RequireAuth><CDNServicesPage /></RequireAuth>} />

              {/* Placeholder routes for nav completeness */}
              <Route path="/fleet-health" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/node-groups" element={<RequireAuth><NodesPage /></RequireAuth>} />
              <Route path="/networking/wireguard" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/networking/bgp" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/networking/ips" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/users" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />
              <Route path="/settings" element={<RequireAuth><GlobalDashboard /></RequireAuth>} />

              <Route path="*" element={<NotFound />} />
            </Routes>
          </AuthProvider>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  </ThemeProvider>
);

export default App;
