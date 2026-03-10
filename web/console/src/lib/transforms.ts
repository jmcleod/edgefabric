// Transform functions: convert backend API types (snake_case) → SPA view-model types (camelCase).
// Missing fields are filled with sensible defaults.

import type {
  Node, Gateway, Tenant, DNSZone, DNSRecord, CDNService, CDNOrigin, Route,
  AuditLogEntry, APIKey, BGPPeer, WireGuardPeer, ProvisioningJob, NodeGroup,
  HealthStatus, GlobalStats, User, SSHKey, IPAllocation,
} from '@/types';
import type {
  ApiNode, ApiGateway, ApiTenant, ApiDNSZone, ApiDNSRecord, ApiCDNSite, ApiCDNOrigin,
  ApiRoute, ApiAuditEvent, ApiAPIKey, ApiBGPSession, ApiWireGuardPeer, ApiProvisioningJob,
  ApiNodeGroup, ApiStatusResponse, ApiUser, ApiSSHKey, ApiIPAllocation,
} from '@/types/api';

// --- Status Mapping ---

const nodeStatusMap: Record<string, HealthStatus> = {
  online: 'healthy',
  offline: 'critical',
  error: 'critical',
  pending: 'syncing',
  enrolling: 'syncing',
  decommissioned: 'unknown',
};

const gatewayStatusMap: Record<string, HealthStatus> = {
  online: 'healthy',
  offline: 'critical',
  error: 'critical',
  pending: 'syncing',
  enrolling: 'syncing',
};

export function mapNodeStatus(backendStatus: string): HealthStatus {
  return nodeStatusMap[backendStatus] || 'unknown';
}

export function mapGatewayStatus(backendStatus: string): HealthStatus {
  return gatewayStatusMap[backendStatus] || 'unknown';
}

// --- Entity Transforms ---

export function transformUser(api: ApiUser): User {
  return {
    id: api.id,
    email: api.email,
    name: api.name,
    role: api.role,
    tenantId: api.tenant_id,
    totpEnabled: api.totp_enabled,
    status: api.status,
    createdAt: api.created_at,
    lastLogin: api.last_login_at,
  };
}

export function transformNode(api: ApiNode): Node {
  return {
    id: api.id,
    name: api.name,
    hostname: api.hostname,
    location: api.metadata?.location || api.region || '\u2014',
    region: api.region || '\u2014',
    status: mapNodeStatus(api.status),
    ipv4: api.public_ip,
    ipv6: api.wireguard_ip || undefined,
    tenantId: api.tenant_id,
    nodeGroupId: undefined, // Not on node object directly
    lastSeen: api.last_heartbeat || api.updated_at,
    version: api.binary_version || '\u2014',
    cpu: 0,      // No runtime metrics in backend yet
    memory: 0,   // No runtime metrics in backend yet
    uptime: '\u2014', // No runtime metrics in backend yet
  };
}

export function transformGateway(api: ApiGateway): Gateway {
  return {
    id: api.id,
    name: api.name,
    hostname: api.metadata?.hostname || '\u2014',
    location: api.metadata?.location || '\u2014',
    status: mapGatewayStatus(api.status),
    publicIp: api.public_ip || '\u2014',
    wireguardPubkey: '\u2014', // Not exposed in API
    tenantId: api.tenant_id,
    connectedNodes: 0, // Not tracked in backend
    lastSeen: api.last_heartbeat || api.updated_at,
  };
}

export function transformTenant(api: ApiTenant): Tenant {
  return {
    id: api.id,
    name: api.name,
    slug: api.slug,
    status: api.status === 'deleted' ? 'suspended' : api.status as 'active' | 'suspended' | 'pending',
    createdAt: api.created_at,
    nodeCount: 0,       // Computed aggregate — not in backend
    zoneCount: 0,       // Computed aggregate — not in backend
    cdnServiceCount: 0, // Computed aggregate — not in backend
  };
}

export function transformNodeGroup(api: ApiNodeGroup): NodeGroup {
  return {
    id: api.id,
    name: api.name,
    description: api.description,
    tenantId: api.tenant_id,
    nodeCount: 0, // Would need a separate query
    createdAt: api.created_at,
  };
}

export function transformDNSZone(api: ApiDNSZone): DNSZone {
  return {
    id: api.id,
    name: api.name,
    tenantId: api.tenant_id,
    recordCount: 0, // Would need a separate query per zone
    status: api.status === 'active' ? 'healthy' : 'critical',
    serial: api.serial,
    createdAt: api.created_at,
    lastModified: api.updated_at,
  };
}

export function transformDNSRecord(api: ApiDNSRecord): DNSRecord {
  return {
    id: api.id,
    zoneId: api.zone_id,
    name: api.name,
    type: api.type as DNSRecord['type'],
    value: api.value,
    ttl: api.ttl || 3600,
    priority: api.priority,
    createdAt: api.created_at,
  };
}

export function transformCDNSite(api: ApiCDNSite): CDNService {
  return {
    id: api.id,
    name: api.name,
    tenantId: api.tenant_id,
    status: api.status === 'active' ? 'healthy' : 'critical',
    domainCount: api.domains?.length || 0,
    originCount: 0, // Would need a separate query
    bandwidthGb: 0,  // Not tracked in backend
    requestsM: 0,    // Not tracked in backend
    createdAt: api.created_at,
  };
}

export function transformCDNOrigin(api: ApiCDNOrigin): CDNOrigin {
  // Backend stores address as "host:port" or just "host"
  const parts = api.address.split(':');
  const hostname = parts[0] || api.address;
  const port = parts.length > 1 ? parseInt(parts[1], 10) : (api.scheme === 'https' ? 443 : 80);

  return {
    id: api.id,
    serviceId: api.site_id,
    name: hostname,
    hostname,
    port,
    protocol: api.scheme,
    weight: api.weight,
    status: api.status === 'healthy' ? 'healthy' : api.status === 'unhealthy' ? 'critical' : 'unknown',
  };
}

export function transformRoute(api: ApiRoute): Route {
  const exposedIp = api.entry_port
    ? `${api.entry_ip}:${api.entry_port}`
    : api.entry_ip;
  const privateDestination = api.destination_port
    ? `${api.destination_ip}:${api.destination_port}`
    : api.destination_ip;

  return {
    id: api.id,
    tenantId: api.tenant_id,
    name: api.name,
    exposedIp,
    gatewayId: api.gateway_id,
    privateDestination,
    status: api.status === 'active' ? 'healthy' : 'critical',
    createdAt: api.created_at,
  };
}

export function transformAuditEvent(api: ApiAuditEvent): AuditLogEntry {
  return {
    id: api.id,
    timestamp: api.timestamp,
    userId: api.user_id || '\u2014',
    userEmail: '\u2014', // Not available in API — would need user lookup
    action: api.action,
    resource: api.resource,
    resourceId: '', // Not a separate field in backend
    tenantId: api.tenant_id,
    details: api.details,
    ipAddress: api.source_ip,
  };
}

export function transformAPIKey(api: ApiAPIKey): APIKey {
  return {
    id: api.id,
    name: api.name,
    tenantId: api.tenant_id,
    prefix: api.key_prefix,
    scopes: [api.role], // Backend has single role, SPA expects array
    createdAt: api.created_at,
    lastUsed: api.last_used_at,
    expiresAt: api.expires_at,
  };
}

export function transformBGPSession(api: ApiBGPSession): BGPPeer {
  return {
    id: api.id,
    nodeId: api.node_id,
    peerIp: api.peer_address,
    peerAsn: api.peer_asn,
    localAsn: api.local_asn,
    status: api.status === 'established' ? 'established' :
            api.status === 'idle' ? 'idle' :
            api.status === 'configured' ? 'active' : 'idle',
    prefixesReceived: api.announced_prefixes?.length || 0,
    prefixesAdvertised: api.announced_prefixes?.length || 0,
    uptime: '\u2014', // Not tracked in backend
  };
}

export function transformWireGuardPeer(api: ApiWireGuardPeer): WireGuardPeer {
  return {
    id: api.id,
    nodeId: api.owner_id,
    publicKey: api.public_key,
    endpoint: api.endpoint || '\u2014',
    allowedIps: api.allowed_ips || [],
    lastHandshake: api.last_rotated_at,
    rxBytes: 0, // Not tracked in backend
    txBytes: 0, // Not tracked in backend
  };
}

export function transformProvisioningJob(api: ApiProvisioningJob): ProvisioningJob {
  // Derive progress from steps
  const totalSteps = api.steps?.length || 0;
  const completedSteps = api.steps?.filter(s => s.status === 'completed').length || 0;
  const progress = totalSteps > 0 ? Math.round((completedSteps / totalSteps) * 100) : 0;

  // Map action to SPA job type
  const typeMap: Record<string, ProvisioningJob['type']> = {
    enroll: 'node_provision',
    reprovision: 'config_deploy',
    upgrade: 'node_upgrade',
    start: 'config_deploy',
    stop: 'config_deploy',
    restart: 'config_deploy',
    decommission: 'config_deploy',
  };

  return {
    id: api.id,
    type: typeMap[api.action] || 'config_deploy',
    targetId: api.node_id,
    targetName: api.node_id, // Would need node lookup for name
    status: api.status,
    progress,
    createdAt: api.created_at,
    startedAt: api.started_at,
    completedAt: api.completed_at,
    logs: api.steps?.map(s => `[${s.step}] ${s.status}${s.error ? ': ' + s.error : ''}`) || [],
  };
}

export function transformSSHKey(api: ApiSSHKey): SSHKey {
  return {
    id: api.id,
    name: api.name,
    publicKey: api.public_key,
    fingerprint: api.fingerprint,
    createdAt: api.created_at,
    lastRotatedAt: api.last_rotated_at,
  };
}

export function transformIPAllocation(api: ApiIPAllocation): IPAllocation {
  return {
    id: api.id,
    tenantId: api.tenant_id,
    prefix: api.prefix,
    type: api.type,
    purpose: api.purpose,
    status: api.status,
    createdAt: api.created_at,
    updatedAt: api.updated_at,
  };
}

export function transformStatus(api: ApiStatusResponse): GlobalStats {
  const onlineNodes = api.nodes_by_status?.['online'] || 0;

  return {
    totalNodes: api.node_count,
    healthyNodes: onlineNodes,
    totalTenants: api.tenant_count || 0,
    activeTenants: api.tenant_count || 0, // All counted tenants are active
    totalZones: api.dns_zone_count,
    totalCDNServices: api.cdn_site_count,
    bandwidthTbMonth: 0,   // Not tracked in backend
    requestsBMonth: 0,     // Not tracked in backend
  };
}
