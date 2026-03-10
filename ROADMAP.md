# EdgeFabric Roadmap

## Implementation Plan

Work proceeds in milestones. Each milestone should result in a working (if incomplete) system that builds on the previous one. Milestones are ordered by dependency — later milestones depend on earlier ones.

**v1 (Milestones 0–10):** Foundation through packaging — all complete.
**v2 (Milestones 11–17):** Monitoring, CDN enhancements, HA completion, protocol extensions, tenant metrics, advanced networking, platform extensibility.

---

### Milestone 0: Project Foundation ✅
**Goal**: Buildable binary, CI, tooling, project structure.

- [x] Repository structure and Go module
- [x] Architecture documentation
- [x] Domain model documentation
- [x] Roadmap
- [x] LICENSE (MIT)
- [x] Taskfile.yml (build, lint, test, generate)
- [x] golangci-lint configuration
- [x] GitHub Actions CI (build, lint, test)
- [x] CLI entrypoints with cobra (controller, node, gateway, cli, version)
- [x] Configuration loading (YAML) with validation
- [x] Structured logging (slog)
- [x] Domain types as Go structs
- [x] Storage interface definition
- [x] SQLite storage driver (schema + migrations, CRUD stubs)
- [x] Health endpoint
- [x] Basic metrics (Prometheus)
- [x] Taskfile builds static binary
- [x] SQLite CRUD implementations (M1-scoped: tenant, user, API key, audit)

---

### Milestone 1: Controller Core ✅
**Goal**: Controller starts, serves API, manages tenants and users.

- [x] REST API router with versioned prefix (/api/v1/)
- [x] OpenAPI spec (hand-authored, served at /api/v1/openapi.yaml)
- [x] Authentication middleware (password + API key)
- [x] RBAC middleware
- [x] Tenant CRUD API
- [x] User CRUD API (with password hashing)
- [x] API Key management
- [x] TOTP 2FA enrollment and verification
- [x] Audit logging for all mutations
- [x] Encrypted secrets storage (AES-256-GCM)
- [x] SPA static file embedding and serving
- [x] Seed SuperUser on first run
- [x] Unit tests (storage, auth, tokens, tenant, user services)

---

### Milestone 2: Fleet Management ✅
**Goal**: Controller can manage inventory of nodes and gateways.

- [x] Node CRUD API
- [x] Gateway CRUD API
- [x] Node group management
- [x] IP allocation management
- [x] SSH key storage and management
- [x] Node/Gateway assignment to tenants
- [x] Health/heartbeat tracking
- [x] Fleet status dashboard API

---

### Milestone 3: WireGuard Overlay ✅
**Goal**: Controller can generate WireGuard configs, nodes/gateways can connect.

- [x] WireGuard key generation (controller-side)
- [x] WireGuard peer configuration management
- [x] Overlay IP allocation (10.100.0.0/16 default)
- [x] WireGuard interface setup on node/gateway
- [x] Hub-spoke topology: controller is hub
- [x] Key rotation API
- [ ] Connectivity health checks over overlay → *Milestone 11*

---

### Milestone 4: Provisioning ✅
**Goal**: Controller can deploy the binary and config to nodes over SSH.

- [x] SSH client with key auth
- [x] Binary upload via SCP/SFTP
- [x] Config generation and push
- [x] systemd unit generation and installation
- [x] Enrollment token generation and validation
- [x] Node bootstrap flow (enroll → configure → start)
- [x] Binary version tracking per node *(done — `domain.Node.BinaryVersion` + provisioning job tracking)*
- [x] Rolling update support *(done — backup, atomic swap, rollback in `internal/provisioning/steps.go`)*

---

### Milestone 5: BGP ✅
**Goal**: Nodes can announce anycast prefixes to upstream peers.

- [x] GoBGP integration as library
- [x] BGP session configuration from controller
- [x] Prefix announcement management
- [x] BGP session status reporting
- [x] Session lifecycle (start, stop, reconfigure)
- [ ] BGP session monitoring and alerting → *Milestone 11*

---

### Milestone 6: Anycast DNS ✅
**Goal**: Nodes serve authoritative DNS for tenant zones.

- [x] DNS zone CRUD API on controller
- [x] DNS record CRUD API (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, PTR)
- [x] Type-specific record validation + CNAME exclusivity enforcement
- [x] Zone serial auto-management (increment on every record mutation)
- [x] Zone-to-node assignment via NodeGroups
- [x] Zone data sync from controller to nodes (polling-based, GET /nodes/{id}/config/dns)
- [x] Authoritative DNS server on nodes (miekg/dns) — UDP + TCP
- [x] SOA auto-generation from zone metadata
- [x] Noop DNS service for demo/test mode
- [x] Node DNS config, startup, and 30-second reconciliation loop
- [x] NXDOMAIN for unknown names, REFUSED for unserved zones
- [ ] DNS query logging → *Milestone 11*
- [ ] Zone transfer/AXFR protocol → *Milestone 14*

---

### Milestone 7: CDN ✅
**Goal**: Nodes act as caching reverse proxies for tenant sites.

- [x] CDN site configuration API
- [x] Origin management API
- [x] Reverse proxy with httputil.ReverseProxy
- [x] In-memory LRU response cache (disk-based deferred to v2)
- [x] Cache key generation
- [x] Cache TTL and invalidation
- [x] Cache purge API
- [x] TLS termination (auto-cert) *(done — `autocert.Manager` in `internal/app/controller.go`)*
- [x] Origin health checks
- [x] Header manipulation (add/set/remove)
- [x] Compression (gzip) — brotli → *Milestone 12*
- [x] Rate limiting per-site
- [x] CDN config sync from controller to nodes
- [x] Domain-based routing (Host header matching)
- [x] Weighted origin selection
- [x] Node-side noop CDN service for demo mode

---

### Milestone 8: Route/Gateway ✅
**Goal**: Traffic routing from anycast entry through nodes to gateways into private networks.

- [x] Route CRUD API on controller
- [x] Route config sync to nodes and gateways
- [x] TCP/UDP proxy listener on nodes (userspace forwarding)
- [ ] ICMP forwarding → *Milestone 14*
- [x] Packet forwarding over WireGuard to gateway
- [x] Gateway forwarding to private destinations
- [x] RFC1918 routing (gateway binds to WireGuard overlay IP, forwards to private destinations)
- [x] Connection tracking / NAT for return traffic (TCP bidirectional relay, UDP session map with idle timeout)
- [ ] Route health monitoring → *Milestone 11*

---

### Milestone 9: Observability & Hardening ✅
**Goal**: Production-grade operational visibility and security hardening.

- [x] Prometheus metrics for all services (system gauges, service-level counters, HTTP request metrics)
- [x] Health/readiness/liveness endpoints for controller, node, and gateway
- [x] Audit trail completeness (failed logins, TOTP enrollment/failure)
- [x] Security headers middleware (CSP, X-Frame-Options, X-Content-Type-Options)
- [x] Config validation (encryption key format, service modes, WireGuard IPs)
- [x] Error model cleanup (HandleServiceError, ErrValidation)
- [x] Status endpoint with full resource counts and build info
- [x] Event notification scaffolding (in-process event bus)
- [x] Schema versioning for SQLite migrations
- [x] Token signing key separation from encryption key
- [x] Security model documentation
- [x] Operations documentation (backup/restore, upgrades, monitoring)
- [x] Config sync tracking for drift visibility
- [x] Event bus wired into fleet heartbeat for status transitions
- [x] Rate limiting on auth endpoints *(done — `internal/api/middleware/ratelimit.go`)*
- [x] Key rotation with versioned key IDs *(done — `internal/crypto/crypto.go`, `internal/secrets/store.go`)*
- [x] Webhook/Slack event handlers *(done — `internal/events/webhook_handler.go`, `slack_handler.go`)*
- [ ] Email event handler → *Milestone 13*
- [ ] Per-tenant usage metrics → *Milestone 15*

---

### Milestone 10: Packaging & Demo ✅
**Goal**: Ready for external evaluation and open-source consumption.

- [x] Dockerfile (multi-stage, static binary)
- [x] Docker Compose demo environment (controller + curl-based setup)
- [x] systemd unit templates (controller, node, gateway)
- [x] Example configurations for all roles (examples/)
- [x] API usage guide with curl examples (docs/api-guide.md)
- [x] Developer setup guide (docs/developer-guide.md)
- [x] Deployment / operator guide (docs/deployment-guide.md)
- [x] README overhaul with badges and documentation hub
- [x] Web UI (placeholder SPA)
- [x] GitHub Actions release workflow
- [ ] Web UI beyond placeholder SPA *(future)*

---

## Current Status

**All v1 milestones (0–10) are complete.** The initial v2 implementation push delivered: PostgreSQL HA backend, rate limiting, key rotation, webhook/Slack notifications, rolling updates, TLS/ACME, CORS, and cursor-based pagination. Milestones 11–17 cover the remaining v2 work.

See the [Demo](demo/README.md) for a quick start, or the [Deployment Guide](docs/deployment-guide.md) for production setup.

---

## v2 Milestones

### v2 Deferred Items — Status

Collected from v1 milestones. Items marked ✅ were completed in the initial v2 push; remaining items are assigned to milestones below.

| Area | Item | Status |
|------|------|--------|
| **HA** | HA Controller (Postgres backend) | ✅ Done — `internal/storage/postgres/` (20 files) |
| **HA** | Leader election | Milestone 13 |
| **Networking** | WireGuard connectivity health checks | Milestone 11 |
| **Networking** | Full mesh WireGuard | Milestone 16 |
| **Networking** | IPv6 support | Milestone 16 |
| **Provisioning** | Binary version tracking per node | ✅ Done — `domain.Node.BinaryVersion` + provisioning jobs |
| **Provisioning** | Rolling update support | ✅ Done — backup, atomic swap, rollback in `internal/provisioning/steps.go` |
| **BGP** | BGP session monitoring and alerting | Milestone 11 |
| **DNS** | DNS query logging | Milestone 11 |
| **DNS** | Zone transfer (AXFR) | Milestone 14 |
| **CDN** | Auto-cert TLS (ACME) | ✅ Done — `autocert.Manager` in `internal/app/controller.go` |
| **CDN** | Brotli compression | Milestone 12 |
| **CDN** | Disk-based response cache | Milestone 12 |
| **CDN** | WAF | Milestone 12 |
| **Routes** | ICMP forwarding | Milestone 14 |
| **Routes** | Route health monitoring | Milestone 11 |
| **Security** | Auth endpoint rate limiting | ✅ Done — `internal/api/middleware/ratelimit.go` |
| **Security** | Key rotation with versioned IDs | ✅ Done — `internal/crypto/crypto.go`, `internal/secrets/store.go` |
| **Events** | Webhook/Slack handlers | ✅ Done — `internal/events/webhook_handler.go`, `slack_handler.go` |
| **Events** | Email handler | Milestone 13 |
| **Metrics** | Per-tenant usage metrics | Milestone 15 |
| **UI** | Full web dashboard | Future |
| **Infra** | Kubernetes operator | Milestone 17 |
| **Infra** | Plugin system | Milestone 17 |

---

### Milestone 11: Monitoring & Health Checks
**Goal:** Active health monitoring across the overlay, BGP, DNS, and routes with event bus alerting.

Complexity: **M** | Dependencies: None — can start immediately.

- [ ] **11.1 WireGuard Connectivity Health Checks** [M]
  - Periodic probe (TCP connect or ICMP) over overlay to each peer
  - Fire `OverlayPeerUnreachable` event on consecutive failures
  - Configurable probe interval and failure threshold
  - **New:** `internal/networking/health.go`
  - **Modify:** `internal/events/event.go`, `internal/app/controller.go`, `internal/config/config.go`

- [ ] **11.2 BGP Session Monitoring & Alerting** [S]
  - Poll GoBGP for session state transitions
  - Fire `BGPSessionDown` / `BGPSessionEstablished` events via event bus
  - Existing `GoBGPService.GetStatus()` returns peer states — just needs wiring
  - **New:** `internal/bgp/monitor.go`
  - **Modify:** `internal/events/event.go`, `internal/app/node.go`

- [ ] **11.3 Route Health Monitoring** [M]
  - Active TCP/UDP probe to route destinations at configurable interval
  - Fire `RouteHealthCheckFailed` event after threshold failures
  - Pattern: follow CDN origin `HealthChecker` in `internal/cdnserver/health.go`
  - **New:** `internal/routeserver/health.go`
  - **Modify:** `internal/events/event.go`, `internal/domain/route.go`

- [ ] **11.4 DNS Query Logging** [S]
  - Structured slog per query: zone, qname, qtype, rcode, latency, answer count
  - Add `DNSQueryDuration` histogram and `DNSQueriesByZone` counter vec to metrics
  - **Modify:** `internal/dnsserver/miekg.go`, `internal/observability/metrics.go`

---

### Milestone 12: CDN Enhancements
**Goal:** Better CDN performance with brotli compression, persistent caching, and request filtering.

Complexity: **M–L** | Dependencies: None — can start immediately.

- [ ] **12.1 Brotli Compression** [S]
  - Add brotli as preferred encoding over gzip (Accept-Encoding negotiation)
  - New `brotliResponseWriter` mirroring existing `gzipResponseWriter` pattern
  - **Dependency:** `github.com/andybalholm/brotli`
  - **Modify:** `internal/cdnserver/proxy.go`, `go.mod`

- [ ] **12.2 Disk-Based Response Cache** [M]
  - Hybrid cache: memory LRU (hot) + content-addressable disk storage (overflow)
  - Extract `CacheBackend` interface from existing `Cache` struct
  - SHA256-hashed file paths, metadata file for disk usage tracking
  - **New:** `internal/cdnserver/diskcache.go`
  - **Modify:** `internal/cdnserver/cache.go`, `internal/cdnserver/proxy.go`, `internal/config/config.go`

- [ ] **12.3 WAF (Web Application Firewall)** [L]
  - Rule-based request filtering: SQL injection, XSS, path traversal
  - Two modes: `detect` (log only) and `block` (403 response)
  - Compiled regex rule sets (OWASP-inspired patterns)
  - **New:** `internal/cdnserver/waf.go`, `waf_rules.go`, `waf_test.go`
  - **Modify:** `internal/cdnserver/proxy.go`, `internal/domain/cdn.go`, `internal/observability/metrics.go`

---

### Milestone 13: HA & Notifications Completion
**Goal:** Complete HA with leader election; finish notification system with email.

Complexity: **M** | Dependencies: PostgreSQL driver (already done).

- [ ] **13.1 Leader Election via PostgreSQL Advisory Locks** [M]
  - `pg_try_advisory_lock()` for session-scoped leader election
  - Only leader runs background tasks (gauge updater, health checks, reconcile)
  - `onElected` / `onDemoted` callbacks to start/stop background goroutines
  - No-op in SQLite mode (single instance by definition)
  - **New:** `internal/ha/leader.go`
  - **Modify:** `internal/app/controller.go`, `internal/config/config.go`, `internal/storage/postgres/postgres.go`

- [ ] **13.2 Email Notification Handler** [S]
  - SMTP-based email handler on event bus (follows webhook/Slack handler pattern)
  - HTML email templates via `html/template`, retry with exponential backoff
  - Config: `notifications.email.smtp_host`, `from_addr`, `recipients`
  - **New:** `internal/events/email_handler.go`, `email_handler_test.go`
  - **Modify:** `internal/config/config.go`, `internal/app/controller.go`

---

### Milestone 14: DNS & Route Protocol Extensions
**Goal:** Zone transfer support for secondary DNS and ICMP route forwarding.

Complexity: **M–L** | Dependencies: None.

- [ ] **14.1 Zone Transfer (AXFR)** [M]
  - Serve full zone transfers over TCP for secondary DNS replication
  - ACL: configurable allowed transfer IPs
  - Zone data already materialized in memory — AXFR is iteration of `zoneData.records`
  - miekg/dns has built-in `dns.Transfer` support
  - **Modify:** `internal/dnsserver/miekg.go`, `internal/config/config.go`

- [ ] **14.2 ICMP Forwarding** [L]
  - Raw socket ICMP proxy via `golang.org/x/net/icmp`
  - Requires `CAP_NET_RAW` capability (or root)
  - Echo ID → client mapping for return traffic (similar to UDP session map)
  - Only ICMP echo (ping) initially; other types logged but not forwarded
  - **New:** `internal/routeserver/icmp.go`
  - **Modify:** `internal/routeserver/forwarder.go`

---

### Milestone 15: Per-Tenant Observability
**Goal:** Tenant-scoped Prometheus metrics for bandwidth, requests, queries, and forwarded bytes.

Complexity: **L** | Dependencies: Milestone 11 (monitoring infrastructure).

- [ ] **15.1 Per-Tenant Usage Metrics** [L]
  - Tenant-labeled CounterVecs: `TenantHTTPRequests`, `TenantCDNBandwidth`, `TenantDNSQueries`, `TenantRouteBytesForwarded`
  - CDN: resolve tenant from `siteRuntime.site.TenantID`
  - DNS: resolve tenant from `zoneData.tenantID` (add field)
  - Routes: resolve tenant from `routeRuntime.route.TenantID`
  - API: extract tenant from auth context
  - **Modify:** `internal/observability/metrics.go`, `internal/cdnserver/proxy.go`, `internal/dnsserver/miekg.go`, `internal/routeserver/forwarder.go`, `internal/api/middleware/metrics.go`

---

### Milestone 16: Advanced Networking
**Goal:** Evolve overlay from hub-spoke to full mesh; add IPv6 dual-stack.

Complexity: **L** | Dependencies: Soft dependency on Milestone 11 (overlay health checks for mesh monitoring).

- [ ] **16.1 Full Mesh WireGuard** [L]
  - Direct node-to-node WireGuard tunnels based on node group membership
  - `GenerateMeshConfig(nodeID)` returns config with controller hub + all mesh peers
  - Support mixed mode (some nodes mesh, others hub-spoke)
  - Topology config: `wireguard.topology: "hub-spoke" | "mesh"`
  - **New:** `internal/networking/mesh.go`
  - **Modify:** `internal/networking/wireguard_config.go`, `internal/networking/service.go`, `internal/config/config.go`, `internal/provisioning/steps.go`

- [ ] **16.2 IPv6 Support** [L]
  - Dual-stack overlay: `fd00:ef::/48` ULA alongside `10.100.0.0/16`
  - IPv6 BGP: add `AFI_IP6` AfiSafi to GoBGP reconcile
  - IPv6 routing: Go's `net.Dial` handles transparently
  - AAAA records already supported in DNS server
  - **Modify:** `internal/networking/wireguard_config.go`, `internal/config/config.go`, `internal/bgp/gobgp.go`, `internal/provisioning/wireguard.go`

---

### Milestone 17: Platform Extensibility
**Goal:** Kubernetes CRD operator and plugin architecture for custom service runtimes.

Complexity: **L** | Dependencies: Benefits from all prior milestones being stable.

- [ ] **17.1 Kubernetes Operator** [L]
  - Separate Go module in `operator/` with kubebuilder scaffolding
  - CRDs: Tenant, Node, Gateway, DNSZone, CDNSite, Route
  - Reconcile loops call EdgeFabric controller REST API
  - Status subresource reflects sync state
  - **New:** `operator/` directory tree (api, controllers, config)

- [ ] **17.2 Plugin System** [L]
  - Interface-based plugins (not Go `plugin` package — too fragile)
  - Plugin interfaces for DNS resolver, CDN middleware, route processor
  - Built-in implementations register at init; custom ones compiled in at build time
  - Registry maps plugin names to implementations
  - **New:** `internal/plugin/registry.go`, `internal/plugin/types.go`
  - **Modify:** `internal/dnsserver/service.go`, `internal/cdnserver/service.go`, `internal/routeserver/service.go`, `internal/config/config.go`

---

## Milestone Dependency Graph

```
M11 (Monitoring) ─────────┐
M12 (CDN) ────────────────┤── all independent, can start in parallel
M13 (HA + Email) ─────────┤
M14 (DNS + ICMP) ─────────┘
M15 (Tenant Metrics) ──────── depends on M11
M16 (Networking) ──────────── soft dep on M11 (overlay health for mesh)
M17 (Platform) ────────────── benefits from all prior milestones stable
```
