// Backend-aligned API types — match the exact JSON field names from the Go domain structs.
// These are transformed into the SPA view-model types (types/index.ts) by lib/transforms.ts.

export interface ApiUser {
  id: string;
  tenant_id?: string;
  email: string;
  name: string;
  totp_enabled: boolean;
  role: 'superuser' | 'admin' | 'readonly';
  status: 'active' | 'disabled';
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiTenant {
  id: string;
  name: string;
  slug: string;
  status: 'active' | 'suspended' | 'deleted';
  settings?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ApiNode {
  id: string;
  tenant_id?: string;
  name: string;
  hostname: string;
  public_ip: string;
  wireguard_ip: string;
  status: 'pending' | 'enrolling' | 'online' | 'offline' | 'error' | 'decommissioned';
  region?: string;
  provider?: string;
  ssh_port: number;
  ssh_user: string;
  ssh_key_id?: string;
  binary_version?: string;
  last_heartbeat?: string;
  last_config_sync?: string;
  capabilities: string[];
  metadata?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface ApiNodeGroup {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiGateway {
  id: string;
  tenant_id: string;
  name: string;
  public_ip?: string;
  wireguard_ip: string;
  status: 'pending' | 'enrolling' | 'online' | 'offline' | 'error';
  last_heartbeat?: string;
  last_config_sync?: string;
  metadata?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface ApiDNSZone {
  id: string;
  tenant_id: string;
  name: string;
  status: 'active' | 'disabled';
  serial: number;
  ttl: number;
  node_group_id?: string;
  transfer_allowed_ips?: string[];
  created_at: string;
  updated_at: string;
}

export interface ApiDNSRecord {
  id: string;
  zone_id: string;
  name: string;
  type: 'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT' | 'NS' | 'SRV' | 'CAA' | 'PTR';
  value: string;
  ttl?: number;
  priority?: number;
  weight?: number;
  port?: number;
  created_at: string;
  updated_at: string;
}

export interface ApiCDNSite {
  id: string;
  tenant_id: string;
  name: string;
  domains: string[];
  tls_mode: 'auto' | 'manual' | 'disabled';
  tls_cert_id?: string;
  cache_enabled: boolean;
  cache_ttl: number;
  compression_enabled: boolean;
  rate_limit_rps?: number;
  node_group_id?: string;
  header_rules?: unknown;
  waf_enabled: boolean;
  waf_mode?: string;
  status: 'active' | 'disabled';
  created_at: string;
  updated_at: string;
}

export interface ApiCDNOrigin {
  id: string;
  site_id: string;
  address: string;
  scheme: 'http' | 'https';
  weight: number;
  health_check_path?: string;
  health_check_interval?: number;
  status: 'healthy' | 'unhealthy' | 'unknown';
  created_at: string;
  updated_at: string;
}

export interface ApiRoute {
  id: string;
  tenant_id: string;
  name: string;
  protocol: 'tcp' | 'udp' | 'icmp' | 'all';
  entry_ip: string;
  entry_port?: number;
  gateway_id: string;
  destination_ip: string;
  destination_port?: number;
  node_group_id?: string;
  status: 'active' | 'disabled';
  created_at: string;
  updated_at: string;
}

export interface ApiAuditEvent {
  id: string;
  tenant_id?: string;
  user_id?: string;
  api_key_id?: string;
  action: string;
  resource: string;
  details?: Record<string, unknown>;
  source_ip: string;
  timestamp: string;
}

export interface ApiProvisioningJob {
  id: string;
  node_id: string;
  tenant_id?: string;
  action: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  current_step: string;
  steps?: ApiStepResult[];
  error?: string;
  initiated_by: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiStepResult {
  step: string;
  status: string;
  output?: string;
  error?: string;
  started_at: string;
  duration_ms: number;
}

export interface ApiBGPSession {
  id: string;
  node_id: string;
  peer_asn: number;
  peer_address: string;
  local_asn: number;
  status: 'configured' | 'established' | 'idle' | 'error';
  announced_prefixes: string[];
  import_policy?: string;
  export_policy?: string;
  metadata?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface ApiWireGuardPeer {
  id: string;
  owner_type: 'node' | 'gateway' | 'controller';
  owner_id: string;
  public_key: string;
  allowed_ips: string[];
  endpoint?: string;
  last_rotated_at: string;
  created_at: string;
  updated_at: string;
}

export interface ApiAPIKey {
  id: string;
  tenant_id: string;
  user_id: string;
  name: string;
  key_prefix: string;
  role: 'superuser' | 'admin' | 'readonly';
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
}

export interface ApiSSHKey {
  id: string;
  name: string;
  public_key: string;
  fingerprint: string;
  created_at: string;
  last_rotated_at: string;
}

export interface ApiIPAllocation {
  id: string;
  tenant_id: string;
  prefix: string;
  type: 'anycast' | 'unicast';
  purpose: 'dns' | 'cdn' | 'route';
  status: 'active' | 'withdrawn' | 'pending';
  created_at: string;
  updated_at: string;
}

export interface ApiStatusResponse {
  version: string;
  commit: string;
  build_time: string;
  tenant_count?: number;
  user_count: number;
  node_count: number;
  nodes_by_status: Record<string, number>;
  gateway_count: number;
  gateways_by_status: Record<string, number>;
  stale_node_count: number;
  stale_gateway_count: number;
  route_count: number;
  dns_zone_count: number;
  cdn_site_count: number;
  schema_version: number;
  is_leader: boolean;
}
