// Zod validation schemas for all entity forms in the EdgeFabric console.

import { z } from 'zod';

// --- Tenants ---

export const tenantSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  slug: z.string().min(1, 'Slug is required').max(50).regex(/^[a-z0-9-]+$/, 'Slug must be lowercase alphanumeric with hyphens'),
  status: z.enum(['active', 'suspended']).default('active'),
});
export type TenantFormData = z.infer<typeof tenantSchema>;

// --- Users ---

export const userSchema = z.object({
  email: z.string().email('Invalid email address'),
  name: z.string().min(1, 'Name is required').max(100),
  role: z.enum(['superuser', 'admin', 'readonly']),
  tenant_id: z.string().optional(),
  password: z.string().min(8, 'Password must be at least 8 characters').optional(),
});
export type UserFormData = z.infer<typeof userSchema>;

// --- Nodes ---

export const nodeSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  hostname: z.string().min(1, 'Hostname is required'),
  public_ip: z.string().min(1, 'Public IP is required'),
  ssh_port: z.coerce.number().int().min(1).max(65535).default(22),
  ssh_user: z.string().min(1, 'SSH user is required').default('root'),
  ssh_key_id: z.string().optional(),
  region: z.string().optional(),
  provider: z.string().optional(),
  tenant_id: z.string().optional(),
});
export type NodeFormData = z.infer<typeof nodeSchema>;

// --- Gateways ---

export const gatewaySchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  tenant_id: z.string().min(1, 'Tenant is required'),
  public_ip: z.string().optional(),
});
export type GatewayFormData = z.infer<typeof gatewaySchema>;

// --- Node Groups ---

export const nodeGroupSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  description: z.string().optional(),
  tenant_id: z.string().min(1, 'Tenant is required'),
});
export type NodeGroupFormData = z.infer<typeof nodeGroupSchema>;

// --- DNS Zones ---

export const dnsZoneSchema = z.object({
  name: z.string().min(1, 'Zone name is required').regex(/^[a-zA-Z0-9.-]+$/, 'Invalid zone name'),
  ttl: z.coerce.number().int().min(60).max(86400).default(3600),
  node_group_id: z.string().optional(),
  transfer_allowed_ips: z.string().optional().default(''),
});
export type DNSZoneFormData = z.infer<typeof dnsZoneSchema>;

// --- DNS Records ---

export const dnsRecordSchema = z.object({
  name: z.string().min(1, 'Record name is required'),
  type: z.enum(['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA', 'PTR']),
  value: z.string().min(1, 'Value is required'),
  ttl: z.coerce.number().int().min(60).max(86400).optional(),
  priority: z.coerce.number().int().min(0).max(65535).optional(),
  weight: z.coerce.number().int().min(0).optional(),
  port: z.coerce.number().int().min(1).max(65535).optional(),
});
export type DNSRecordFormData = z.infer<typeof dnsRecordSchema>;

// --- CDN Sites ---

export const cdnSiteSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  domains: z.string().min(1, 'At least one domain is required'),
  tls_mode: z.enum(['auto', 'manual', 'disabled']).default('auto'),
  cache_enabled: z.boolean().default(true),
  cache_ttl: z.coerce.number().int().min(0).default(3600),
  compression_enabled: z.boolean().default(true),
  rate_limit_rps: z.coerce.number().int().min(0).optional(),
  waf_enabled: z.boolean().default(false),
  waf_mode: z.enum(['detect', 'block']).default('detect'),
  node_group_id: z.string().optional(),
});
export type CDNSiteFormData = z.infer<typeof cdnSiteSchema>;

// --- CDN Origins ---

export const cdnOriginSchema = z.object({
  address: z.string().min(1, 'Origin address is required'),
  scheme: z.enum(['http', 'https']).default('https'),
  weight: z.coerce.number().int().min(0).max(100).default(100),
  health_check_path: z.string().optional(),
  health_check_interval: z.coerce.number().int().min(5).optional(),
});
export type CDNOriginFormData = z.infer<typeof cdnOriginSchema>;

// --- Routes ---

export const routeSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  protocol: z.enum(['tcp', 'udp', 'icmp', 'all']).default('tcp'),
  entry_ip: z.string().min(1, 'Entry IP is required'),
  entry_port: z.coerce.number().int().min(1).max(65535).optional(),
  gateway_id: z.string().min(1, 'Gateway is required'),
  destination_ip: z.string().min(1, 'Destination IP is required'),
  destination_port: z.coerce.number().int().min(1).max(65535).optional(),
  node_group_id: z.string().optional(),
});
export type RouteFormData = z.infer<typeof routeSchema>;

// --- SSH Keys ---

export const sshKeySchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
});
export type SSHKeyFormData = z.infer<typeof sshKeySchema>;

// --- API Keys ---

export const apiKeySchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  role: z.enum(['superuser', 'admin', 'readonly']).default('readonly'),
  expires_at: z.string().optional(),
});
export type APIKeyFormData = z.infer<typeof apiKeySchema>;

// --- BGP Peers ---

export const bgpPeerSchema = z.object({
  peer_asn: z.coerce.number().int().min(1, 'Peer ASN is required'),
  peer_address: z.string().min(1, 'Peer address is required'),
  local_asn: z.coerce.number().int().min(1, 'Local ASN is required'),
  announced_prefixes: z.string().optional(),
  import_policy: z.string().optional(),
  export_policy: z.string().optional(),
});
export type BGPPeerFormData = z.infer<typeof bgpPeerSchema>;

// --- IP Allocations ---

export const ipAllocationSchema = z.object({
  prefix: z.string().min(1, 'IP prefix is required').regex(/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,2}$/, 'Must be a valid CIDR (e.g. 192.168.1.0/24)'),
  type: z.enum(['anycast', 'unicast']),
  purpose: z.enum(['dns', 'cdn', 'route']),
});
export type IPAllocationFormData = z.infer<typeof ipAllocationSchema>;

// --- Cache Purge ---

export const cachePurgeSchema = z.object({
  paths: z.string().min(1, 'At least one path is required'),
});
export type CachePurgeFormData = z.infer<typeof cachePurgeSchema>;
