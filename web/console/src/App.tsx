import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ThemeProvider } from "@/hooks/useTheme";
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

const queryClient = new QueryClient();

const App = () => (
  <ThemeProvider>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter>
          <Routes>
            {/* Global/SuperUser Routes */}
            <Route path="/" element={<GlobalDashboard />} />
            <Route path="/nodes" element={<NodesPage />} />
            <Route path="/nodes/:id" element={<NodeDetailPage />} />
            <Route path="/tenants" element={<TenantsPage />} />
            <Route path="/gateways" element={<GatewaysPage />} />
            <Route path="/jobs" element={<ProvisioningJobsPage />} />
            <Route path="/audit" element={<AuditLogsPage />} />
            
            {/* Tenant Routes */}
            <Route path="/tenant/dns/zones" element={<DNSZonesPage />} />
            <Route path="/tenant/cdn/services" element={<CDNServicesPage />} />
            
            {/* Placeholder routes for nav completeness */}
            <Route path="/fleet-health" element={<GlobalDashboard />} />
            <Route path="/node-groups" element={<NodesPage />} />
            <Route path="/networking/wireguard" element={<GlobalDashboard />} />
            <Route path="/networking/bgp" element={<GlobalDashboard />} />
            <Route path="/networking/ips" element={<GlobalDashboard />} />
            <Route path="/users" element={<GlobalDashboard />} />
            <Route path="/settings" element={<GlobalDashboard />} />
            
            {/* ADD ALL CUSTOM ROUTES ABOVE THE CATCH-ALL "*" ROUTE */}
            <Route path="*" element={<NotFound />} />
          </Routes>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  </ThemeProvider>
);

export default App;
