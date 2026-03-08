# EdgeFabric Architecture

## Overview

EdgeFabric is a distributed edge networking platform that orchestrates a global fleet of nodes to deliver anycast IP services, authoritative DNS, basic CDN, and edge-to-private-network routing. It ships as a single static Go binary with subcommands for each role.

## System Roles

```
┌─────────────────────────────────────────────────────────────┐
│                    EdgeFabric Controller                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ REST API │ │  Web UI  │ │ Auth/RBAC│ │  Provisioning │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Tenancy  │ │  Fleet   │ │ WireGuard│ │   Storage     │  │
│  │ Manager  │ │ Manager  │ │ Manager  │ │ (SQLite/PG)   │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
└────────────────────────┬────────────────────────────────────┘
                         │ WireGuard Hub
              ┌──────────┼──────────┐
              │          │          │
     ┌────────▼──┐ ┌────▼─────┐ ┌─▼──────────┐
     │   Node A  │ │  Node B  │ │  Gateway X  │
     │  ┌─────┐  │ │  ┌─────┐ │ │  ┌───────┐ │
     │  │ BGP │  │ │  │ BGP │ │ │  │ Route  │ │
     │  │ DNS │  │ │  │ DNS │ │ │  │Forward │ │
     │  │ CDN │  │ │  │ CDN │ │ │  └───┬───┘ │
     │  └─────┘  │ │  └─────┘ │ │      │     │
     └───────────┘ └──────────┘ └──────┼─────┘
                                       │
                                ┌──────▼──────┐
                                │   Private    │
                                │   Network    │
                                └─────────────┘
```

### Controller
- Central source of truth and management plane
- Hosts the REST API (versioned, OpenAPI-documented) and embedded SPA
- Manages tenants, fleet, provisioning, configuration distribution
- Acts as WireGuard hub in the overlay network
- Single instance for v1, architecture supports future HA

### Node
- Edge runtime deployed on Linux VPS/bare-metal
- Receives configuration from Controller over WireGuard
- Runs BGP (GoBGP) to announce anycast prefixes to upstream peers
- Serves authoritative DNS for tenant zones
- Operates as CDN reverse proxy with caching
- Forwards routed traffic to Gateways

### Gateway
- Connects EdgeFabric overlay to private/internal networks
- Receives forwarded traffic from Nodes over WireGuard
- Routes to private destinations (RFC1918, TCP/UDP/ICMP)
- Deployed inside tenant infrastructure

## Network Architecture

**Overlay**: Hub-and-spoke WireGuard mesh. Controller is the hub; Nodes and Gateways are spokes. All control-plane and forwarded-traffic flows through this overlay in v1.

**Anycast**: Nodes announce tenant-assigned IP prefixes via BGP to their upstream peers. Multiple Nodes announcing the same prefix creates anycast behavior — nearest Node handles traffic.

**Traffic flow for Route/Gateway**:
```
Client → (anycast IP) → Node → (WireGuard) → Gateway → Private destination
```

**Traffic flow for CDN**:
```
Client → (anycast IP) → Node CDN (cache check → origin fetch if miss) → Response
```

**Traffic flow for DNS**:
```
Client → (anycast IP:53) → Node DNS → Authoritative response from synced zone data
```

## Binary & Subcommands

```
edgefabric controller   # Run as controller
edgefabric node         # Run as edge node
edgefabric gateway      # Run as gateway
edgefabric cli          # Management CLI client
edgefabric version      # Print version info
```

Each mode reads a shared config file format (YAML) with role-specific sections.

## Internal Architecture

The monolith is structured as loosely-coupled internal packages with explicit service interfaces. Each package exposes a `Service` interface that can later be extracted behind a network boundary.

### Service Boundaries

| Package | Responsibility | Key Interface |
|---------|---------------|---------------|
| `internal/auth` | Authentication (local, 2FA, API keys) | `Authenticator` |
| `internal/rbac` | Role-based access control | `Authorizer` |
| `internal/tenant` | Tenant lifecycle and isolation | `TenantService` |
| `internal/fleet` | Node/Gateway inventory and health | `FleetService` |
| `internal/provisioning` | SSH-based deployment and config push | `Provisioner` |
| `internal/wireguard` | WireGuard key management and tunnel config | `WireGuardManager` |
| `internal/bgp` | BGP session management via GoBGP | `BGPService` |
| `internal/dns` | Authoritative DNS zone management and server | `DNSService` |
| `internal/cdn` | Reverse proxy, caching, TLS termination | `CDNService` |
| `internal/route` | Traffic routing rules and forwarding | `RouteService` |
| `internal/storage` | Persistence abstraction (SQLite/PostgreSQL) | `Store` |
| `internal/audit` | Audit log recording and querying | `AuditLogger` |
| `internal/observability` | Metrics, health, structured logging | `MetricsCollector` |
| `internal/crypto` | Key generation, encryption utilities | — |
| `internal/secrets` | Encrypted secret storage | `SecretStore` |
| `internal/config` | Configuration loading and validation | — |

### Dependency Direction

```
cmd/ → internal/app/ → service packages → internal/storage/
                     → internal/config/
                     → internal/observability/
```

Services depend on storage and config but NOT on each other's implementations. Cross-service communication goes through defined interfaces, making future extraction straightforward.

## Persistence

- Pluggable `Store` interface with SQLite and PostgreSQL drivers
- SQLite is default for single-instance deployments
- PostgreSQL required for future HA Controller
- Migrations managed in Go (no external tools)

## Authentication & Authorization

- Local username + password (bcrypt)
- TOTP-based 2FA
- API keys for programmatic access
- Roles: SuperUser (global), Admin (tenant-scoped), ReadOnly (tenant-scoped)
- RBAC enforced at API middleware layer

## Security Baseline

- All secrets encrypted at rest (AES-256-GCM)
- WireGuard keys generated and rotated by Controller
- Signed enrollment tokens for Node/Gateway bootstrap
- TLS on all external-facing endpoints
- No anonymous admin access
- Audit log for all state-changing operations

## Configuration

Single YAML config file per instance with role-specific sections:

```yaml
role: controller  # controller | node | gateway

controller:
  listen_addr: ":8443"
  storage:
    driver: sqlite
    dsn: "edgefabric.db"
  # ...

node:
  controller_addr: "wg://10.100.0.1:8443"
  # ...

gateway:
  controller_addr: "wg://10.100.0.1:8443"
  # ...
```

## Future Considerations (NOT in v1)

- HA Controller (Raft consensus or external coordination)
- IPv6 support
- Full mesh WireGuard topology
- WAF
- Dynamic routing protocols for Gateway
- Kubernetes operator
- Plugin/extension system
