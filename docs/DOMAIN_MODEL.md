# EdgeFabric v1 Domain Model

## Entity Relationship Overview

```
Tenant ──┬── has many ─── Node (assigned)
         ├── has many ─── Gateway
         ├── has many ─── NodeGroup
         ├── has many ─── DNSZone ── has many ── DNSRecord
         ├── has many ─── CDNSite ── has many ── CDNOrigin
         ├── has many ─── Route
         ├── has many ─── IPAllocation
         ├── has many ─── User
         └── has many ─── APIKey

User ──── has one ────── Role (SuperUser | Admin | ReadOnly)

Node ──── has one ────── WireGuardPeer
     ──── has many ───── BGPSession
     ──── belongs to ─── NodeGroup (many-to-many)

Gateway ── has one ───── WireGuardPeer
        ── has many ──── PrivateRoute

Controller ── has one ── WireGuardHub
```

## Core Entities

### Tenant

Multi-tenant isolation boundary. All resources belong to a tenant.

```
Tenant {
    ID          uuid
    Name        string          (unique)
    Slug        string          (unique, URL-safe)
    Status      enum            (active, suspended, deleted)
    Settings    json            (tenant-level configuration overrides)
    CreatedAt   timestamp
    UpdatedAt   timestamp
}
```

### User

Human operator with authentication credentials.

```
User {
    ID          uuid
    TenantID    uuid?           (null for SuperUser)
    Email       string          (unique)
    Name        string
    PasswordHash string
    TOTPSecret  string?         (encrypted, null if 2FA not enabled)
    TOTPEnabled bool
    Role        enum            (superuser, admin, readonly)
    Status      enum            (active, disabled)
    LastLoginAt timestamp?
    CreatedAt   timestamp
    UpdatedAt   timestamp
}
```

### APIKey

Programmatic access credential scoped to a tenant and role.

```
APIKey {
    ID          uuid
    TenantID    uuid
    UserID      uuid            (creating user)
    Name        string
    KeyHash     string          (bcrypt hash of key)
    KeyPrefix   string          (first 8 chars for identification)
    Role        enum            (admin, readonly)
    ExpiresAt   timestamp?
    LastUsedAt  timestamp?
    CreatedAt   timestamp
}
```

### Node

An edge server managed by EdgeFabric.

```
Node {
    ID              uuid
    TenantID        uuid?           (null = unassigned)
    Name            string
    Hostname        string
    PublicIP        string          (public IP for SSH and BGP)
    WireGuardIP     string          (overlay IP assigned by controller)
    Status          enum            (pending, enrolling, online, offline, error, decommissioned)
    Region          string?         (operator-defined region label)
    Provider        string?         (e.g., "vultr", "hetzner")
    SSHPort         int             (default 22)
    SSHUser         string
    SSHKeyID        uuid            (reference to stored SSH key)
    BinaryVersion   string?         (deployed edgefabric version)
    LastHeartbeat   timestamp?
    Capabilities    []string        (bgp, dns, cdn, route)
    Metadata        json
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### NodeGroup

Logical grouping of nodes for targeting configuration.

```
NodeGroup {
    ID          uuid
    TenantID    uuid
    Name        string
    Description string?
    CreatedAt   timestamp
    UpdatedAt   timestamp
}

NodeGroupMembership {
    NodeGroupID uuid
    NodeID      uuid
}
```

### Gateway

A gateway connecting EdgeFabric to a private network.

```
Gateway {
    ID              uuid
    TenantID        uuid
    Name            string
    PublicIP         string?         (may be behind NAT)
    WireGuardIP      string          (overlay IP)
    Status          enum            (pending, enrolling, online, offline, error)
    EnrollmentToken string?         (one-time, signed)
    LastHeartbeat   timestamp?
    Metadata        json
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### WireGuardPeer

WireGuard peer configuration managed by the controller.

```
WireGuardPeer {
    ID              uuid
    OwnerType       enum            (node, gateway, controller)
    OwnerID         uuid
    PublicKey        string
    PrivateKey      string          (encrypted at rest)
    PresharedKey    string?         (encrypted at rest)
    AllowedIPs      []string
    Endpoint        string?         (public endpoint for direct connectivity)
    LastRotatedAt   timestamp
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### IPAllocation

IP addresses/prefixes assigned to tenants for anycast.

```
IPAllocation {
    ID          uuid
    TenantID    uuid
    Prefix      string          (CIDR notation, e.g., "203.0.113.0/24")
    Type        enum            (anycast, unicast)
    Purpose     enum            (dns, cdn, route)
    Status      enum            (active, withdrawn, pending)
    CreatedAt   timestamp
    UpdatedAt   timestamp
}
```

### BGPSession

A BGP peering session on a node.

```
BGPSession {
    ID              uuid
    NodeID          uuid
    PeerASN         uint32
    PeerAddress     string
    LocalASN        uint32
    Status          enum            (configured, established, idle, error)
    AnnouncedPrefixes []string      (CIDRs being announced)
    ImportPolicy    string?
    ExportPolicy    string?
    Metadata        json
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### DNSZone

An authoritative DNS zone managed by a tenant.

```
DNSZone {
    ID          uuid
    TenantID    uuid
    Name        string          (e.g., "example.com")
    Status      enum            (active, disabled)
    Serial      uint32          (SOA serial, auto-incremented)
    TTL         int             (default TTL)
    NodeGroupID uuid?           (target node group, null = all tenant nodes)
    CreatedAt   timestamp
    UpdatedAt   timestamp
}
```

### DNSRecord

Individual DNS records within a zone.

```
DNSRecord {
    ID          uuid
    ZoneID      uuid
    Name        string          (relative name, e.g., "www" or "@")
    Type        enum            (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, PTR)
    Value       string
    TTL         int?            (override zone default)
    Priority    int?            (for MX, SRV)
    Weight      int?            (for SRV)
    Port        int?            (for SRV)
    CreatedAt   timestamp
    UpdatedAt   timestamp
}
```

### CDNSite

A CDN site configuration (reverse proxy).

```
CDNSite {
    ID              uuid
    TenantID        uuid
    Name            string
    Domains         []string        (hostnames to serve)
    TLSMode         enum            (auto, manual, disabled)
    TLSCertID       uuid?           (reference to stored cert if manual)
    CacheEnabled    bool
    CacheTTL        int             (default cache TTL in seconds)
    CompressionEnabled bool
    RateLimitRPS    int?            (requests per second, null = unlimited)
    NodeGroupID     uuid?           (target node group)
    HeaderRules     json            (request/response header manipulation)
    Status          enum            (active, disabled)
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### CDNOrigin

Backend origin server for a CDN site.

```
CDNOrigin {
    ID              uuid
    SiteID          uuid
    Address         string          (host:port)
    Scheme          enum            (http, https)
    Weight          int             (load balancing weight)
    HealthCheckPath string?
    HealthCheckInterval int?        (seconds)
    Status          enum            (healthy, unhealthy, unknown)
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### Route

A traffic routing rule that forwards traffic from anycast IPs through Nodes to Gateways.

```
Route {
    ID              uuid
    TenantID        uuid
    Name            string
    Protocol        enum            (tcp, udp, icmp, all)
    EntryIP         string          (anycast IP that receives traffic)
    EntryPort       int?            (null for ICMP / all-ports)
    GatewayID       uuid            (target gateway)
    DestinationIP   string          (private destination IP)
    DestinationPort int?
    NodeGroupID     uuid?           (which nodes accept this traffic)
    Status          enum            (active, disabled)
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### SSHKey

SSH credentials used by the controller for provisioning.

```
SSHKey {
    ID              uuid
    Name            string
    PublicKey        string
    PrivateKey       string          (encrypted at rest)
    Fingerprint     string
    CreatedAt       timestamp
    LastRotatedAt   timestamp
}
```

### TLSCertificate

TLS certificates stored for CDN or API usage.

```
TLSCertificate {
    ID              uuid
    TenantID        uuid
    Domains         []string
    CertPEM         string          (encrypted at rest)
    KeyPEM          string          (encrypted at rest)
    Issuer          string
    ExpiresAt       timestamp
    AutoRenew       bool
    CreatedAt       timestamp
    UpdatedAt       timestamp
}
```

### AuditEvent

Immutable audit log entry.

```
AuditEvent {
    ID          uuid
    TenantID    uuid?
    UserID      uuid?
    APIKeyID    uuid?
    Action      string          (e.g., "node.create", "dns.record.update")
    Resource    string          (e.g., "node:uuid")
    Details     json
    SourceIP    string
    Timestamp   timestamp
}
```

### EnrollmentToken

One-time signed token for Node/Gateway bootstrap.

```
EnrollmentToken {
    ID              uuid
    TenantID        uuid
    TargetType      enum            (node, gateway)
    TargetID        uuid
    Token           string          (signed JWT or HMAC)
    ExpiresAt       timestamp
    UsedAt          timestamp?
    CreatedAt       timestamp
}
```

## Key Invariants

1. **Tenant isolation**: Nodes assigned to a tenant are exclusive. IPs assigned to a tenant are exclusive. No cross-tenant resource access.
2. **Node lifecycle**: pending → enrolling → online ↔ offline → decommissioned. Only forward transitions to decommissioned.
3. **IP exclusivity**: An anycast IP prefix is assigned to exactly one tenant at a time.
4. **WireGuard key management**: All key generation happens on the controller. Private keys are encrypted at rest. Rotation is controller-initiated.
5. **DNS consistency**: Zone serial increments on every record change. Zone data is synced to target nodes after each change.
6. **Audit completeness**: Every state-changing API call produces an audit event.
