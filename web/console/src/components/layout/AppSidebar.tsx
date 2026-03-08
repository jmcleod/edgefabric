import { useLocation } from 'react-router-dom';
import { NavLink } from '@/components/NavLink';
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarHeader,
  SidebarFooter,
  useSidebar,
} from '@/components/ui/sidebar';
import {
  LayoutDashboard,
  Building2,
  Server,
  Network,
  Globe,
  Shield,
  Settings,
  Users,
  FileText,
  Activity,
  HardDrive,
  Boxes,
  Route,
  Key,
  Layers,
  Radio,
  Waypoints,
  RefreshCw,
  Disc,
} from 'lucide-react';
import { currentUser } from '@/data/mockData';

interface NavItem {
  title: string;
  url: string;
  icon: React.ElementType;
}

interface NavSection {
  label: string;
  items: NavItem[];
}

const globalNavigation: NavSection[] = [
  {
    label: 'Overview',
    items: [
      { title: 'Dashboard', url: '/', icon: LayoutDashboard },
      { title: 'Fleet Health', url: '/fleet-health', icon: Activity },
    ],
  },
  {
    label: 'Infrastructure',
    items: [
      { title: 'Tenants', url: '/tenants', icon: Building2 },
      { title: 'Nodes', url: '/nodes', icon: Server },
      { title: 'Node Groups', url: '/node-groups', icon: Boxes },
      { title: 'Gateways', url: '/gateways', icon: Waypoints },
    ],
  },
  {
    label: 'Networking',
    items: [
      { title: 'WireGuard', url: '/networking/wireguard', icon: Shield },
      { title: 'BGP Peers', url: '/networking/bgp', icon: Radio },
      { title: 'Advertised IPs', url: '/networking/ips', icon: Globe },
    ],
  },
  {
    label: 'Operations',
    items: [
      { title: 'Provisioning Jobs', url: '/jobs', icon: RefreshCw },
      { title: 'Audit Logs', url: '/audit', icon: FileText },
    ],
  },
  {
    label: 'Administration',
    items: [
      { title: 'Users & Access', url: '/users', icon: Users },
      { title: 'System Settings', url: '/settings', icon: Settings },
    ],
  },
];

const tenantNavigation: NavSection[] = [
  {
    label: 'Overview',
    items: [
      { title: 'Dashboard', url: '/tenant/dashboard', icon: LayoutDashboard },
    ],
  },
  {
    label: 'Infrastructure',
    items: [
      { title: 'Assigned Nodes', url: '/tenant/nodes', icon: Server },
      { title: 'Node Groups', url: '/tenant/node-groups', icon: Boxes },
    ],
  },
  {
    label: 'DNS',
    items: [
      { title: 'Zones', url: '/tenant/dns/zones', icon: Globe },
      { title: 'Records', url: '/tenant/dns/records', icon: FileText },
    ],
  },
  {
    label: 'CDN',
    items: [
      { title: 'Services', url: '/tenant/cdn/services', icon: Layers },
      { title: 'Domains', url: '/tenant/cdn/domains', icon: Network },
      { title: 'Origins', url: '/tenant/cdn/origins', icon: HardDrive },
      { title: 'Cache & Purge', url: '/tenant/cdn/cache', icon: Disc },
    ],
  },
  {
    label: 'Routes',
    items: [
      { title: 'Routes', url: '/tenant/routes', icon: Route },
    ],
  },
  {
    label: 'Access',
    items: [
      { title: 'API Keys', url: '/tenant/api-keys', icon: Key },
      { title: 'Audit Logs', url: '/tenant/audit', icon: FileText },
    ],
  },
];

export function AppSidebar() {
  const { state } = useSidebar();
  const collapsed = state === 'collapsed';

  const isSuperUser = currentUser.role === 'superuser';
  const navigation = isSuperUser ? globalNavigation : tenantNavigation;

  return (
    <Sidebar collapsible="icon" className="border-r border-sidebar-border">
      <SidebarHeader className="border-b border-sidebar-border px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sidebar-primary">
            <Network className="h-4 w-4 text-sidebar-primary-foreground" />
          </div>
          {!collapsed && (
            <div className="flex flex-col">
              <span className="text-sm font-semibold text-sidebar-foreground tracking-wide uppercase">EdgeFabric</span>
              <span className="text-xs text-sidebar-foreground/70">
                {isSuperUser ? 'Platform Admin' : 'Tenant Portal'}
              </span>
            </div>
          )}
        </div>
      </SidebarHeader>

      <SidebarContent className="scrollbar-thin">
        {navigation.map((section) => (
          <SidebarGroup key={section.label}>
            <SidebarGroupLabel className="text-xs uppercase tracking-wider text-sidebar-foreground/50">
              {section.label}
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {section.items.map((item) => (
                  <SidebarMenuItem key={item.url}>
                    <SidebarMenuButton asChild>
                      <NavLink
                        to={item.url}
                        end={item.url === '/' || item.url === '/tenant/dashboard'}
                        className="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                        activeClassName="bg-sidebar-accent text-white font-medium"
                      >
                        <item.icon className="h-4 w-4 shrink-0" />
                        {!collapsed && <span>{item.title}</span>}
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="border-t border-sidebar-border p-4">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-sidebar-accent text-xs font-medium text-sidebar-accent-foreground">
            {currentUser.name.split(' ').map((n) => n[0]).join('')}
          </div>
          {!collapsed && (
            <div className="flex flex-col overflow-hidden">
              <span className="truncate text-sm font-medium text-sidebar-foreground">{currentUser.name}</span>
              <span className="truncate text-xs text-sidebar-foreground/60">{currentUser.email}</span>
            </div>
          )}
        </div>
      </SidebarFooter>
    </Sidebar>
  );
}
