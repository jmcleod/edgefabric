# EdgeFabric Roadmap

## Implementation Plan

Work proceeds in milestones. Each milestone should result in a working (if incomplete) system that builds on the previous one. Milestones are ordered by dependency — later milestones depend on earlier ones.

**v1 (Milestones 0–10):** Foundation through packaging — all complete.
**v2 (Milestones 11–17):** Monitoring, CDN enhancements, HA completion, protocol extensions, tenant metrics, advanced networking, platform extensibility — all complete.

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
- [x] Connectivity health checks over overlay *(done — `internal/networking/health.go`)*

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
- [x] BGP session monitoring and alerting *(done — `internal/bgp/monitor.go`)*

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
- [x] DNS query logging *(done — structured slog per query, `DNSQueryDuration` histogram, `DNSQueriesByZone` counter)*
- [x] Zone transfer/AXFR protocol *(done — `internal/dnsserver/axfr.go`)*

---

### Milestone 7: CDN ✅
**Goal**: Nodes act as caching reverse proxies for tenant sites.

- [x] CDN site configuration API
- [x] Origin management API
- [x] Reverse proxy with httputil.ReverseProxy
- [x] In-memory LRU response cache
- [x] Cache key generation
- [x] Cache TTL and invalidation
- [x] Cache purge API
- [x] TLS termination (auto-cert) *(done — `autocert.Manager` in `internal/app/controller.go`)*
- [x] Origin health checks
- [x] Header manipulation (add/set/remove)
- [x] Compression (gzip + brotli) *(done — `internal/cdnserver/proxy.go`)*
- [x] Rate limiting per-site
- [x] CDN config sync from controller to nodes
- [x] Domain-based routing (Host header matching)
- [x] Weighted origin selection
- [x] Node-side noop CDN service for demo mode
- [x] Disk-based response cache *(done — `internal/cdnserver/diskcache.go`)*
- [x] WAF *(done — `internal/cdnserver/waf.go`, `waf_rules.go`)*

---

### Milestone 8: Route/Gateway ✅
**Goal**: Traffic routing from anycast entry through nodes to gateways into private networks.

- [x] Route CRUD API on controller
- [x] Route config sync to nodes and gateways
- [x] TCP/UDP proxy listener on nodes (userspace forwarding)
- [x] ICMP forwarding *(done — `internal/routeserver/icmp.go`)*
- [x] Packet forwarding over WireGuard to gateway
- [x] Gateway forwarding to private destinations
- [x] RFC1918 routing (gateway binds to WireGuard overlay IP, forwards to private destinations)
- [x] Connection tracking / NAT for return traffic (TCP bidirectional relay, UDP session map with idle timeout)
- [x] Route health monitoring *(done — `internal/routeserver/health.go`)*

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
- [x] Email event handler *(done — `internal/events/email_handler.go`)*
- [x] Per-tenant usage metrics *(done — tenant-labeled Prometheus counters in `internal/observability/metrics.go`)*

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
- [x] Web UI — full React SPA with dashboard, fleet management, DNS/CDN configuration
- [x] GitHub Actions release workflow

---

## v2 Milestones — All Complete ✅

### v2 Deferred Items — Status

Collected from v1 milestones. All items have been completed.

| Area | Item | Status |
|------|------|--------|
| **HA** | HA Controller (Postgres backend) | ✅ Done — `internal/storage/postgres/` |
| **HA** | Leader election | ✅ Done — `internal/ha/leader.go` |
| **Networking** | WireGuard connectivity health checks | ✅ Done — `internal/networking/health.go` |
| **Networking** | Full mesh WireGuard | ✅ Done — `internal/networking/mesh.go` |
| **Networking** | IPv6 support | ✅ Done — dual-stack overlay + IPv6 BGP |
| **Provisioning** | Binary version tracking per node | ✅ Done — `domain.Node.BinaryVersion` + provisioning jobs |
| **Provisioning** | Rolling update support | ✅ Done — backup, atomic swap, rollback in `internal/provisioning/steps.go` |
| **BGP** | BGP session monitoring and alerting | ✅ Done — `internal/bgp/monitor.go` |
| **DNS** | DNS query logging | ✅ Done — structured slog per query + Prometheus metrics |
| **DNS** | Zone transfer (AXFR) | ✅ Done — `internal/dnsserver/axfr.go` |
| **CDN** | Auto-cert TLS (ACME) | ✅ Done — `autocert.Manager` in `internal/app/controller.go` |
| **CDN** | Brotli compression | ✅ Done — `internal/cdnserver/proxy.go` |
| **CDN** | Disk-based response cache | ✅ Done — `internal/cdnserver/diskcache.go` |
| **CDN** | WAF | ✅ Done — `internal/cdnserver/waf.go` |
| **Routes** | ICMP forwarding | ✅ Done — `internal/routeserver/icmp.go` |
| **Routes** | Route health monitoring | ✅ Done — `internal/routeserver/health.go` |
| **Security** | Auth endpoint rate limiting | ✅ Done — `internal/api/middleware/ratelimit.go` |
| **Security** | Key rotation with versioned IDs | ✅ Done — `internal/crypto/crypto.go`, `internal/secrets/store.go` |
| **Events** | Webhook/Slack handlers | ✅ Done — `internal/events/webhook_handler.go`, `slack_handler.go` |
| **Events** | Email handler | ✅ Done — `internal/events/email_handler.go` |
| **Metrics** | Per-tenant usage metrics | ✅ Done — tenant-labeled Prometheus counters |
| **UI** | Full web dashboard | ✅ Done — React SPA with 30+ pages |
| **Infra** | Kubernetes operator | ✅ Done — `operator/` (CRDs, reconcilers, REST client) |
| **Infra** | Plugin system | ✅ Done — `internal/plugin/` (registry, types, built-in registrations) |

---

### Milestone 11: Monitoring & Health Checks ✅
**Goal:** Active health monitoring across the overlay, BGP, DNS, and routes with event bus alerting.

- [x] **11.1 WireGuard Connectivity Health Checks** — `internal/networking/health.go`
- [x] **11.2 BGP Session Monitoring & Alerting** — `internal/bgp/monitor.go`
- [x] **11.3 Route Health Monitoring** — `internal/routeserver/health.go`
- [x] **11.4 DNS Query Logging** — structured slog + Prometheus metrics in `internal/dnsserver/miekg.go`

---

### Milestone 12: CDN Enhancements ✅
**Goal:** Better CDN performance with brotli compression, persistent caching, and request filtering.

- [x] **12.1 Brotli Compression** — `internal/cdnserver/proxy.go`
- [x] **12.2 Disk-Based Response Cache** — `internal/cdnserver/diskcache.go`
- [x] **12.3 WAF (Web Application Firewall)** — `internal/cdnserver/waf.go`, `waf_rules.go`

---

### Milestone 13: HA & Notifications Completion ✅
**Goal:** Complete HA with leader election; finish notification system with email.

- [x] **13.1 Leader Election via PostgreSQL Advisory Locks** — `internal/ha/leader.go`
- [x] **13.2 Email Notification Handler** — `internal/events/email_handler.go`

---

### Milestone 14: DNS & Route Protocol Extensions ✅
**Goal:** Zone transfer support for secondary DNS and ICMP route forwarding.

- [x] **14.1 Zone Transfer (AXFR)** — `internal/dnsserver/axfr.go`
- [x] **14.2 ICMP Forwarding** — `internal/routeserver/icmp.go`

---

### Milestone 15: Per-Tenant Observability ✅
**Goal:** Tenant-scoped Prometheus metrics for bandwidth, requests, queries, and forwarded bytes.

- [x] **15.1 Per-Tenant Usage Metrics** — tenant-labeled CounterVecs in `internal/observability/metrics.go`, wired into CDN proxy, DNS server, route forwarder, and API middleware

---

### Milestone 16: Advanced Networking ✅
**Goal:** Evolve overlay from hub-spoke to full mesh; add IPv6 dual-stack.

- [x] **16.1 Full Mesh WireGuard** — `internal/networking/mesh.go`
- [x] **16.2 IPv6 Support** — dual-stack overlay (`fd00:ef::/48` ULA + `10.100.0.0/16`), IPv6 BGP, WireGuard

---

### Milestone 17: Platform Extensibility ✅
**Goal:** Kubernetes CRD operator and plugin architecture for custom service runtimes.

- [x] **17.1 Kubernetes Operator** — `operator/` (CRDs for Tenant, Node, Gateway, DNSZone, CDNSite, Route; reconcilers calling EdgeFabric REST API; status subresources)
- [x] **17.2 Plugin System** — `internal/plugin/` (typed registry, factory functions, built-in self-registration, dynamic config validation)

---

## Milestone Dependency Graph

```
M11 (Monitoring) ─────────┐
M12 (CDN) ────────────────┤── all complete
M13 (HA + Email) ─────────┤
M14 (DNS + ICMP) ─────────┘
M15 (Tenant Metrics) ──────── ✅
M16 (Networking) ──────────── ✅
M17 (Platform) ────────────── ✅
```

## Future Work

All planned milestones (0–17) are complete. Potential future directions:

- **Service mesh integration** — Istio/Linkerd sidecar support for node services
- **Multi-region controller federation** — Cross-region controller replication
- **WebAssembly plugins** — WASM-based plugin runtime for sandboxed extensions
- **Advanced traffic management** — Weighted routing, canary deployments, traffic splitting
- **Terraform provider** — Infrastructure-as-code support alongside the Kubernetes operator
