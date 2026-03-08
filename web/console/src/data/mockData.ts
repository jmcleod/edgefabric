import type {
  User,
  Tenant,
  Node,
  NodeGroup,
  Gateway,
  DNSZone,
  DNSRecord,
  CDNService,
  CDNDomain,
  CDNOrigin,
  Route,
  BGPPeer,
  WireGuardPeer,
  ProvisioningJob,
  AuditLogEntry,
  APIKey,
  GlobalStats,
  TenantStats,
} from '@/types';

// Current user context (simulated)
export const currentUser: User = {
  id: 'usr_001',
  email: 'admin@edgefabric.io',
  name: 'Platform Admin',
  role: 'superuser',
  createdAt: '2024-01-15T10:00:00Z',
  lastLogin: '2024-03-08T09:30:00Z',
};

export const tenants: Tenant[] = [
  { id: 'ten_001', name: 'Acme Corp', slug: 'acme', status: 'active', createdAt: '2024-01-20T12:00:00Z', nodeCount: 12, zoneCount: 5, cdnServiceCount: 3 },
  { id: 'ten_002', name: 'TechStart Inc', slug: 'techstart', status: 'active', createdAt: '2024-02-01T08:00:00Z', nodeCount: 6, zoneCount: 2, cdnServiceCount: 1 },
  { id: 'ten_003', name: 'GlobalNet Systems', slug: 'globalnet', status: 'active', createdAt: '2024-02-10T14:30:00Z', nodeCount: 24, zoneCount: 8, cdnServiceCount: 5 },
  { id: 'ten_004', name: 'CloudFirst Ltd', slug: 'cloudfirst', status: 'suspended', createdAt: '2024-01-05T09:00:00Z', nodeCount: 0, zoneCount: 1, cdnServiceCount: 0 },
  { id: 'ten_005', name: 'DataFlow Corp', slug: 'dataflow', status: 'active', createdAt: '2024-02-28T11:00:00Z', nodeCount: 8, zoneCount: 3, cdnServiceCount: 2 },
];

export const nodes: Node[] = [
  { id: 'node_001', name: 'edge-us-east-1a', hostname: 'edge-us-east-1a.edgefabric.net', location: 'Ashburn, VA', region: 'us-east-1', status: 'healthy', ipv4: '198.51.100.10', ipv6: '2001:db8::10', tenantId: 'ten_001', nodeGroupId: 'ng_001', lastSeen: '2024-03-08T10:00:00Z', version: 'v2.4.1', cpu: 23, memory: 45, uptime: '45d 12h' },
  { id: 'node_002', name: 'edge-us-east-1b', hostname: 'edge-us-east-1b.edgefabric.net', location: 'Ashburn, VA', region: 'us-east-1', status: 'healthy', ipv4: '198.51.100.11', tenantId: 'ten_001', nodeGroupId: 'ng_001', lastSeen: '2024-03-08T10:00:00Z', version: 'v2.4.1', cpu: 18, memory: 52, uptime: '45d 12h' },
  { id: 'node_003', name: 'edge-eu-west-1a', hostname: 'edge-eu-west-1a.edgefabric.net', location: 'Dublin, IE', region: 'eu-west-1', status: 'healthy', ipv4: '198.51.100.20', ipv6: '2001:db8::20', tenantId: 'ten_001', nodeGroupId: 'ng_002', lastSeen: '2024-03-08T09:59:00Z', version: 'v2.4.1', cpu: 31, memory: 38, uptime: '30d 8h' },
  { id: 'node_004', name: 'edge-ap-south-1a', hostname: 'edge-ap-south-1a.edgefabric.net', location: 'Mumbai, IN', region: 'ap-south-1', status: 'warning', ipv4: '198.51.100.30', tenantId: 'ten_003', nodeGroupId: 'ng_003', lastSeen: '2024-03-08T09:55:00Z', version: 'v2.4.0', cpu: 78, memory: 82, uptime: '15d 4h' },
  { id: 'node_005', name: 'edge-us-west-2a', hostname: 'edge-us-west-2a.edgefabric.net', location: 'Portland, OR', region: 'us-west-2', status: 'critical', ipv4: '198.51.100.40', tenantId: 'ten_003', lastSeen: '2024-03-08T08:30:00Z', version: 'v2.4.1', cpu: 95, memory: 91, uptime: '2d 1h' },
  { id: 'node_006', name: 'edge-eu-central-1a', hostname: 'edge-eu-central-1a.edgefabric.net', location: 'Frankfurt, DE', region: 'eu-central-1', status: 'healthy', ipv4: '198.51.100.50', ipv6: '2001:db8::50', tenantId: 'ten_002', lastSeen: '2024-03-08T10:00:00Z', version: 'v2.4.1', cpu: 12, memory: 28, uptime: '60d 0h' },
  { id: 'node_007', name: 'edge-ap-northeast-1a', hostname: 'edge-ap-northeast-1a.edgefabric.net', location: 'Tokyo, JP', region: 'ap-northeast-1', status: 'syncing', ipv4: '198.51.100.60', tenantId: 'ten_003', lastSeen: '2024-03-08T09:58:00Z', version: 'v2.4.1', cpu: 5, memory: 15, uptime: '0d 2h' },
  { id: 'node_008', name: 'edge-sa-east-1a', hostname: 'edge-sa-east-1a.edgefabric.net', location: 'São Paulo, BR', region: 'sa-east-1', status: 'healthy', ipv4: '198.51.100.70', tenantId: 'ten_005', lastSeen: '2024-03-08T09:59:00Z', version: 'v2.4.1', cpu: 35, memory: 42, uptime: '22d 6h' },
];

export const nodeGroups: NodeGroup[] = [
  { id: 'ng_001', name: 'US East Production', description: 'Primary US East nodes', tenantId: 'ten_001', nodeCount: 2, createdAt: '2024-01-25T10:00:00Z' },
  { id: 'ng_002', name: 'EU West Production', description: 'Primary EU West nodes', tenantId: 'ten_001', nodeCount: 1, createdAt: '2024-01-25T10:00:00Z' },
  { id: 'ng_003', name: 'APAC Production', description: 'Asia Pacific nodes', tenantId: 'ten_003', nodeCount: 1, createdAt: '2024-02-15T08:00:00Z' },
];

export const gateways: Gateway[] = [
  { id: 'gw_001', name: 'gw-us-east-1', hostname: 'gw-us-east-1.edgefabric.net', location: 'Ashburn, VA', status: 'healthy', publicIp: '203.0.113.10', wireguardPubkey: 'aGVsbG8gd29ybGQhIHRoaXMgaXMgYSB0ZXN0IGtleQ==', tenantId: 'ten_001', connectedNodes: 4, lastSeen: '2024-03-08T10:00:00Z' },
  { id: 'gw_002', name: 'gw-eu-west-1', hostname: 'gw-eu-west-1.edgefabric.net', location: 'Dublin, IE', status: 'healthy', publicIp: '203.0.113.20', wireguardPubkey: 'YW5vdGhlciB0ZXN0IGtleSBmb3IgZXhhbXBsZSE=', tenantId: 'ten_001', connectedNodes: 2, lastSeen: '2024-03-08T10:00:00Z' },
  { id: 'gw_003', name: 'gw-ap-south-1', hostname: 'gw-ap-south-1.edgefabric.net', location: 'Mumbai, IN', status: 'warning', publicIp: '203.0.113.30', wireguardPubkey: 'dGhpcmQgdGVzdCBrZXkgZm9yIGdhdGV3YXk=', tenantId: 'ten_003', connectedNodes: 3, lastSeen: '2024-03-08T09:50:00Z' },
];

export const dnsZones: DNSZone[] = [
  { id: 'zone_001', name: 'acme.com', tenantId: 'ten_001', recordCount: 24, status: 'healthy', serial: 2024030801, createdAt: '2024-01-25T12:00:00Z', lastModified: '2024-03-07T15:30:00Z' },
  { id: 'zone_002', name: 'acme.io', tenantId: 'ten_001', recordCount: 12, status: 'healthy', serial: 2024030501, createdAt: '2024-01-25T12:00:00Z', lastModified: '2024-03-05T10:00:00Z' },
  { id: 'zone_003', name: 'techstart.dev', tenantId: 'ten_002', recordCount: 8, status: 'syncing', serial: 2024030802, createdAt: '2024-02-05T09:00:00Z', lastModified: '2024-03-08T09:45:00Z' },
  { id: 'zone_004', name: 'globalnet.systems', tenantId: 'ten_003', recordCount: 45, status: 'healthy', serial: 2024030701, createdAt: '2024-02-12T14:00:00Z', lastModified: '2024-03-07T18:00:00Z' },
];

export const dnsRecords: DNSRecord[] = [
  { id: 'rec_001', zoneId: 'zone_001', name: '@', type: 'A', value: '198.51.100.10', ttl: 300, createdAt: '2024-01-25T12:00:00Z' },
  { id: 'rec_002', zoneId: 'zone_001', name: 'www', type: 'CNAME', value: 'acme.com', ttl: 3600, createdAt: '2024-01-25T12:00:00Z' },
  { id: 'rec_003', zoneId: 'zone_001', name: 'api', type: 'A', value: '198.51.100.11', ttl: 300, createdAt: '2024-01-26T10:00:00Z' },
  { id: 'rec_004', zoneId: 'zone_001', name: '@', type: 'MX', value: 'mail.acme.com', ttl: 3600, priority: 10, createdAt: '2024-01-25T12:00:00Z' },
  { id: 'rec_005', zoneId: 'zone_001', name: '@', type: 'TXT', value: 'v=spf1 include:_spf.acme.com ~all', ttl: 3600, createdAt: '2024-01-25T12:00:00Z' },
];

export const cdnServices: CDNService[] = [
  { id: 'cdn_001', name: 'Acme Main Site', tenantId: 'ten_001', status: 'healthy', domainCount: 3, originCount: 2, bandwidthGb: 1240, requestsM: 45.2, createdAt: '2024-01-28T10:00:00Z' },
  { id: 'cdn_002', name: 'Acme Static Assets', tenantId: 'ten_001', status: 'healthy', domainCount: 1, originCount: 1, bandwidthGb: 890, requestsM: 120.5, createdAt: '2024-02-01T14:00:00Z' },
  { id: 'cdn_003', name: 'TechStart App', tenantId: 'ten_002', status: 'warning', domainCount: 2, originCount: 2, bandwidthGb: 320, requestsM: 8.7, createdAt: '2024-02-10T09:00:00Z' },
  { id: 'cdn_004', name: 'GlobalNet Portal', tenantId: 'ten_003', status: 'healthy', domainCount: 5, originCount: 3, bandwidthGb: 3200, requestsM: 210.0, createdAt: '2024-02-15T11:00:00Z' },
];

export const cdnDomains: CDNDomain[] = [
  { id: 'dom_001', serviceId: 'cdn_001', domain: 'www.acme.com', status: 'active', sslStatus: 'valid', createdAt: '2024-01-28T10:00:00Z' },
  { id: 'dom_002', serviceId: 'cdn_001', domain: 'acme.com', status: 'active', sslStatus: 'valid', createdAt: '2024-01-28T10:00:00Z' },
  { id: 'dom_003', serviceId: 'cdn_002', domain: 'static.acme.com', status: 'active', sslStatus: 'valid', createdAt: '2024-02-01T14:00:00Z' },
];

export const cdnOrigins: CDNOrigin[] = [
  { id: 'orig_001', serviceId: 'cdn_001', name: 'Primary Origin', hostname: 'origin.acme.internal', port: 443, protocol: 'https', weight: 100, status: 'healthy' },
  { id: 'orig_002', serviceId: 'cdn_001', name: 'Backup Origin', hostname: 'origin-backup.acme.internal', port: 443, protocol: 'https', weight: 50, status: 'healthy' },
];

export const routes: Route[] = [
  { id: 'route_001', tenantId: 'ten_001', name: 'API Gateway Route', exposedIp: '198.51.100.100', gatewayId: 'gw_001', privateDestination: '10.0.1.50:8080', status: 'healthy', createdAt: '2024-02-01T10:00:00Z' },
  { id: 'route_002', tenantId: 'ten_001', name: 'Database Proxy', exposedIp: '198.51.100.101', gatewayId: 'gw_001', privateDestination: '10.0.2.10:5432', status: 'healthy', createdAt: '2024-02-05T14:00:00Z' },
  { id: 'route_003', tenantId: 'ten_003', name: 'Internal Services', exposedIp: '198.51.100.200', gatewayId: 'gw_003', privateDestination: '10.100.0.0/24', status: 'warning', createdAt: '2024-02-20T09:00:00Z' },
];

export const bgpPeers: BGPPeer[] = [
  { id: 'bgp_001', nodeId: 'node_001', peerIp: '169.254.169.1', peerAsn: 64512, localAsn: 65001, status: 'established', prefixesReceived: 15, prefixesAdvertised: 8, uptime: '45d 12h 30m' },
  { id: 'bgp_002', nodeId: 'node_001', peerIp: '169.254.169.2', peerAsn: 64513, localAsn: 65001, status: 'established', prefixesReceived: 22, prefixesAdvertised: 8, uptime: '45d 12h 30m' },
  { id: 'bgp_003', nodeId: 'node_003', peerIp: '169.254.170.1', peerAsn: 64520, localAsn: 65002, status: 'active', prefixesReceived: 0, prefixesAdvertised: 0, uptime: '0s' },
];

export const wireguardPeers: WireGuardPeer[] = [
  { id: 'wg_001', nodeId: 'node_001', publicKey: 'xTIBA5rboUvnH4htodjb60Y7YAf21J7YQMlNGC8HQ14=', endpoint: '203.0.113.10:51820', allowedIps: ['10.0.0.0/24', '10.0.1.0/24'], lastHandshake: '2024-03-08T09:59:45Z', rxBytes: 1024000000, txBytes: 512000000 },
  { id: 'wg_002', nodeId: 'node_002', publicKey: 'HIgo9xNzJMWLKASShiTqIybxZ0U3wGLiUeJ1PKf8ykw=', endpoint: '203.0.113.10:51820', allowedIps: ['10.0.0.0/24'], lastHandshake: '2024-03-08T09:59:30Z', rxBytes: 890000000, txBytes: 445000000 },
];

export const provisioningJobs: ProvisioningJob[] = [
  { id: 'job_001', type: 'node_provision', targetId: 'node_007', targetName: 'edge-ap-northeast-1a', status: 'running', progress: 65, createdAt: '2024-03-08T09:30:00Z', startedAt: '2024-03-08T09:31:00Z', logs: ['Starting node provisioning...', 'Installing base packages...', 'Configuring WireGuard...', 'Waiting for BGP session...'] },
  { id: 'job_002', type: 'node_upgrade', targetId: 'node_004', targetName: 'edge-ap-south-1a', status: 'pending', progress: 0, createdAt: '2024-03-08T10:00:00Z', logs: [] },
  { id: 'job_003', type: 'config_deploy', targetId: 'ng_001', targetName: 'US East Production', status: 'completed', progress: 100, createdAt: '2024-03-07T15:00:00Z', startedAt: '2024-03-07T15:01:00Z', completedAt: '2024-03-07T15:05:00Z', logs: ['Deploying configuration...', 'Validating config on node_001...', 'Validating config on node_002...', 'Configuration deployed successfully.'] },
  { id: 'job_004', type: 'certificate_renewal', targetId: 'cdn_001', targetName: 'Acme Main Site', status: 'failed', progress: 30, createdAt: '2024-03-06T12:00:00Z', startedAt: '2024-03-06T12:01:00Z', completedAt: '2024-03-06T12:03:00Z', logs: ['Starting certificate renewal...', 'Requesting certificate from CA...', 'Error: DNS validation failed'] },
];

export const auditLogs: AuditLogEntry[] = [
  { id: 'audit_001', timestamp: '2024-03-08T09:45:00Z', userId: 'usr_001', userEmail: 'admin@edgefabric.io', action: 'node.create', resource: 'node', resourceId: 'node_007', details: { name: 'edge-ap-northeast-1a' }, ipAddress: '192.0.2.1' },
  { id: 'audit_002', timestamp: '2024-03-08T09:30:00Z', userId: 'usr_002', userEmail: 'ops@acme.com', action: 'dns.record.update', resource: 'dns_record', resourceId: 'rec_001', tenantId: 'ten_001', details: { field: 'ttl', old: 600, new: 300 }, ipAddress: '192.0.2.50' },
  { id: 'audit_003', timestamp: '2024-03-08T09:00:00Z', userId: 'usr_001', userEmail: 'admin@edgefabric.io', action: 'tenant.suspend', resource: 'tenant', resourceId: 'ten_004', details: { reason: 'Payment overdue' }, ipAddress: '192.0.2.1' },
  { id: 'audit_004', timestamp: '2024-03-07T18:30:00Z', userId: 'usr_003', userEmail: 'admin@globalnet.systems', action: 'cdn.purge', resource: 'cdn_service', resourceId: 'cdn_004', tenantId: 'ten_003', details: { paths: ['/*'] }, ipAddress: '192.0.2.100' },
  { id: 'audit_005', timestamp: '2024-03-07T16:00:00Z', userId: 'usr_001', userEmail: 'admin@edgefabric.io', action: 'gateway.restart', resource: 'gateway', resourceId: 'gw_003', details: { reason: 'Scheduled maintenance' }, ipAddress: '192.0.2.1' },
];

export const apiKeys: APIKey[] = [
  { id: 'key_001', name: 'Production API', tenantId: 'ten_001', prefix: 'ef_live_', scopes: ['dns:read', 'dns:write', 'cdn:read'], createdAt: '2024-02-01T10:00:00Z', lastUsed: '2024-03-08T09:30:00Z' },
  { id: 'key_002', name: 'CI/CD Pipeline', tenantId: 'ten_001', prefix: 'ef_live_', scopes: ['cdn:purge'], createdAt: '2024-02-15T14:00:00Z', lastUsed: '2024-03-07T22:00:00Z' },
  { id: 'key_003', name: 'Read-only Monitoring', tenantId: 'ten_001', prefix: 'ef_live_', scopes: ['*:read'], createdAt: '2024-01-30T09:00:00Z', expiresAt: '2024-06-30T09:00:00Z' },
];

export const globalStats: GlobalStats = {
  totalNodes: 48,
  healthyNodes: 44,
  totalTenants: 5,
  activeTenants: 4,
  totalZones: 18,
  totalCDNServices: 12,
  bandwidthTbMonth: 12.4,
  requestsBMonth: 2.8,
};

export const tenantStats: TenantStats = {
  assignedNodes: 12,
  healthyNodes: 11,
  zones: 5,
  records: 48,
  cdnServices: 3,
  routes: 4,
  bandwidthGbMonth: 2130,
  requestsMMonth: 165.7,
};
