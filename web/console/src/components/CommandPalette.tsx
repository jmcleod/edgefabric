import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import {
  LayoutDashboard,
  Server,
  Building2,
  Boxes,
  Waypoints,
  Shield,
  Radio,
  Globe,
  Layers,
  ArrowRightLeft,
  RefreshCw,
  FileText,
  Users,
  Settings,
  Activity,
  Key,
  KeyRound,
  UserCircle,
  Network,
  HardDrive,
  Disc,
} from 'lucide-react';

interface CommandRoute {
  label: string;
  path: string;
  icon: React.ElementType;
  keywords?: string;
}

const superuserRoutes: CommandRoute[] = [
  { label: 'Dashboard', path: '/', icon: LayoutDashboard, keywords: 'home overview' },
  { label: 'Fleet Health', path: '/fleet-health', icon: Activity, keywords: 'status nodes health' },
  { label: 'Tenants', path: '/tenants', icon: Building2, keywords: 'organizations' },
  { label: 'Nodes', path: '/nodes', icon: Server, keywords: 'servers infrastructure' },
  { label: 'Node Groups', path: '/node-groups', icon: Boxes, keywords: 'clusters groups' },
  { label: 'Gateways', path: '/gateways', icon: Waypoints, keywords: 'wireguard vpn' },
  { label: 'WireGuard Peers', path: '/networking/wireguard', icon: Shield, keywords: 'vpn tunnel' },
  { label: 'BGP Peers', path: '/networking/bgp', icon: Radio, keywords: 'routing sessions' },
  { label: 'IP Allocations', path: '/networking/ips', icon: Globe, keywords: 'anycast unicast' },
  { label: 'Provisioning Jobs', path: '/jobs', icon: RefreshCw, keywords: 'deploy provision' },
  { label: 'SSH Keys', path: '/ssh-keys', icon: KeyRound, keywords: 'deploy keys' },
  { label: 'Audit Logs', path: '/audit', icon: FileText, keywords: 'events history' },
  { label: 'Users & Access', path: '/users', icon: Users, keywords: 'accounts roles' },
  { label: 'System Settings', path: '/settings', icon: Settings, keywords: 'configuration' },
  { label: 'Profile', path: '/profile', icon: UserCircle, keywords: 'account password totp' },
];

const tenantRoutes: CommandRoute[] = [
  { label: 'Dashboard', path: '/tenant/dashboard', icon: LayoutDashboard, keywords: 'home overview' },
  { label: 'Assigned Nodes', path: '/tenant/nodes', icon: Server, keywords: 'servers' },
  { label: 'Node Groups', path: '/tenant/node-groups', icon: Boxes, keywords: 'clusters' },
  { label: 'DNS Zones', path: '/tenant/dns/zones', icon: Globe, keywords: 'domains dns' },
  { label: 'DNS Records', path: '/tenant/dns/records', icon: FileText, keywords: 'a aaaa cname mx' },
  { label: 'CDN Services', path: '/tenant/cdn/services', icon: Layers, keywords: 'cdn cache' },
  { label: 'CDN Domains', path: '/tenant/cdn/domains', icon: Network, keywords: 'ssl tls' },
  { label: 'CDN Origins', path: '/tenant/cdn/origins', icon: HardDrive, keywords: 'backend servers' },
  { label: 'Cache & Purge', path: '/tenant/cdn/cache', icon: Disc, keywords: 'purge invalidate' },
  { label: 'Routes', path: '/tenant/routes', icon: ArrowRightLeft, keywords: 'routing proxy' },
  { label: 'API Keys', path: '/tenant/api-keys', icon: Key, keywords: 'tokens access' },
  { label: 'Audit Logs', path: '/tenant/audit', icon: FileText, keywords: 'events history' },
  { label: 'Profile', path: '/profile', icon: UserCircle, keywords: 'account password totp' },
];

// Module-level opener so other components (e.g. AppHeader) can trigger the palette.
let openPaletteFn: (() => void) | null = null;
export function openCommandPalette() {
  openPaletteFn?.();
}

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const { user } = useAuth();

  // Register the opener so external callers can open the palette.
  useEffect(() => {
    openPaletteFn = () => setOpen(true);
    return () => { openPaletteFn = null; };
  }, []);

  const isSuperUser = user?.role === 'superuser';
  const routes = isSuperUser ? superuserRoutes : tenantRoutes;

  // Listen for Cmd+K / Ctrl+K
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((prev) => !prev);
      }
    };
    document.addEventListener('keydown', down);
    return () => document.removeEventListener('keydown', down);
  }, []);

  const handleSelect = useCallback(
    (path: string) => {
      setOpen(false);
      navigate(path);
    },
    [navigate],
  );

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="Search pages..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigation">
          {routes.map((route) => (
            <CommandItem
              key={route.path}
              value={`${route.label} ${route.keywords || ''}`}
              onSelect={() => handleSelect(route.path)}
            >
              <route.icon className="mr-2 h-4 w-4" />
              <span>{route.label}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
