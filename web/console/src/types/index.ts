// EdgeFabric Mock Data Types

export type UserRole = 'superuser' | 'admin' | 'readonly';

export type HealthStatus = 'healthy' | 'warning' | 'critical' | 'unknown' | 'syncing';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  tenantId?: string;
  createdAt: string;
  lastLogin?: string;
}

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  status: 'active' | 'suspended' | 'pending';
  createdAt: string;
  nodeCount: number;
  zoneCount: number;
  cdnServiceCount: number;
}

export interface Node {
  id: string;
  name: string;
  hostname: string;
  location: string;
  region: string;
  status: HealthStatus;
  ipv4: string;
  ipv6?: string;
  tenantId?: string;
  nodeGroupId?: string;
  lastSeen: string;
  version: string;
  cpu: number;
  memory: number;
  uptime: string;
}

export interface NodeGroup {
  id: string;
  name: string;
  description?: string;
  tenantId?: string;
  nodeCount: number;
  createdAt: string;
}

export interface Gateway {
  id: string;
  name: string;
  hostname: string;
  location: string;
  status: HealthStatus;
  publicIp: string;
  wireguardPubkey: string;
  tenantId?: string;
  connectedNodes: number;
  lastSeen: string;
}

export interface DNSZone {
  id: string;
  name: string;
  tenantId: string;
  recordCount: number;
  status: HealthStatus;
  serial: number;
  createdAt: string;
  lastModified: string;
}

export interface DNSRecord {
  id: string;
  zoneId: string;
  name: string;
  type: 'A' | 'AAAA' | 'CNAME' | 'MX' | 'TXT' | 'NS' | 'SRV';
  value: string;
  ttl: number;
  priority?: number;
  createdAt: string;
}

export interface CDNService {
  id: string;
  name: string;
  tenantId: string;
  status: HealthStatus;
  domainCount: number;
  originCount: number;
  bandwidthGb: number;
  requestsM: number;
  createdAt: string;
}

export interface CDNDomain {
  id: string;
  serviceId: string;
  domain: string;
  status: 'active' | 'pending' | 'error';
  sslStatus: 'valid' | 'expired' | 'pending';
  createdAt: string;
}

export interface CDNOrigin {
  id: string;
  serviceId: string;
  name: string;
  hostname: string;
  port: number;
  protocol: 'http' | 'https';
  weight: number;
  status: HealthStatus;
}

export interface Route {
  id: string;
  tenantId: string;
  name: string;
  exposedIp: string;
  gatewayId: string;
  privateDestination: string;
  status: HealthStatus;
  createdAt: string;
}

export interface BGPPeer {
  id: string;
  nodeId: string;
  peerIp: string;
  peerAsn: number;
  localAsn: number;
  status: 'established' | 'active' | 'idle' | 'connect';
  prefixesReceived: number;
  prefixesAdvertised: number;
  uptime: string;
}

export interface WireGuardPeer {
  id: string;
  nodeId: string;
  publicKey: string;
  endpoint: string;
  allowedIps: string[];
  lastHandshake: string;
  rxBytes: number;
  txBytes: number;
}

export interface ProvisioningJob {
  id: string;
  type: 'node_provision' | 'node_upgrade' | 'config_deploy' | 'certificate_renewal';
  targetId: string;
  targetName: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress: number;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  logs: string[];
}

export interface AuditLogEntry {
  id: string;
  timestamp: string;
  userId: string;
  userEmail: string;
  action: string;
  resource: string;
  resourceId: string;
  tenantId?: string;
  details?: Record<string, unknown>;
  ipAddress: string;
}

export interface APIKey {
  id: string;
  name: string;
  tenantId: string;
  prefix: string;
  scopes: string[];
  createdAt: string;
  lastUsed?: string;
  expiresAt?: string;
}

export interface SystemSettings {
  maintenanceMode: boolean;
  nodeAutoRegistration: boolean;
  defaultNodeVersion: string;
  alertEmailRecipients: string[];
  retentionDays: number;
}

// Dashboard stats
export interface GlobalStats {
  totalNodes: number;
  healthyNodes: number;
  totalTenants: number;
  activeTenants: number;
  totalZones: number;
  totalCDNServices: number;
  bandwidthTbMonth: number;
  requestsBMonth: number;
}

export interface TenantStats {
  assignedNodes: number;
  healthyNodes: number;
  zones: number;
  records: number;
  cdnServices: number;
  routes: number;
  bandwidthGbMonth: number;
  requestsMMonth: number;
}
