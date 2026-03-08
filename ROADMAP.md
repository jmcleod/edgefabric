# EdgeFabric v1 Roadmap

## Implementation Plan

Work proceeds in milestones. Each milestone should result in a working (if incomplete) system that builds on the previous one. Milestones are ordered by dependency — later milestones depend on earlier ones.

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
- [ ] Connectivity health checks over overlay *(v2)*

---

### Milestone 4: Provisioning ✅
**Goal**: Controller can deploy the binary and config to nodes over SSH.

- [x] SSH client with key auth
- [x] Binary upload via SCP/SFTP
- [x] Config generation and push
- [x] systemd unit generation and installation
- [x] Enrollment token generation and validation
- [x] Node bootstrap flow (enroll → configure → start)
- [ ] Binary version tracking per node *(v2)*
- [ ] Rolling update support *(v2)*

---

### Milestone 5: BGP ✅
**Goal**: Nodes can announce anycast prefixes to upstream peers.

- [x] GoBGP integration as library
- [x] BGP session configuration from controller
- [x] Prefix announcement management
- [x] BGP session status reporting
- [x] Session lifecycle (start, stop, reconfigure)
- [ ] BGP session monitoring and alerting *(v2)*

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
- [ ] DNS query logging *(v2)*
- [ ] Zone transfer/AXFR protocol *(v2)*

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
- [ ] TLS termination (auto-cert) — TLS mode stored/synced, auto-cert deferred *(v2)*
- [x] Origin health checks
- [x] Header manipulation (add/set/remove)
- [x] Compression (gzip) — brotli deferred *(v2)*
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
- [ ] ICMP forwarding — requires raw sockets / elevated privileges *(v2)*
- [x] Packet forwarding over WireGuard to gateway
- [x] Gateway forwarding to private destinations
- [x] RFC1918 routing (gateway binds to WireGuard overlay IP, forwards to private destinations)
- [x] Connection tracking / NAT for return traffic (TCP bidirectional relay, UDP session map with idle timeout)
- [ ] Route health monitoring — basic status tracking in v1, alerting *(v2)*

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
- [ ] Rate limiting on auth endpoints *(v2)*
- [ ] Key rotation with versioned key IDs *(v2)*
- [ ] Event bus webhook/Slack/email handlers *(v2)*
- [ ] Per-tenant usage metrics *(v2)*

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
- [ ] Web UI beyond placeholder SPA *(v2)*

---

## Current Status

**All v1 milestones (0–10) are complete.** The platform is ready for local evaluation and early production use. See the [Demo](demo/README.md) for a quick start, or the [Deployment Guide](docs/deployment-guide.md) for production setup.

## v2 Deferred Items

Collected from v1 milestones — these are the natural next steps:

| Area | Item | Notes |
|------|------|-------|
| **HA** | HA Controller | Postgres backend + leader election |
| **Networking** | WireGuard connectivity health checks | Ping over overlay, alert on unreachable peers |
| **Networking** | Full mesh WireGuard | Direct node-to-node tunnels |
| **Networking** | IPv6 support | Dual-stack overlay and BGP |
| **Provisioning** | Binary version tracking per node | Track deployed version, flag drift |
| **Provisioning** | Rolling update support | Canary/blue-green upgrades |
| **BGP** | BGP session monitoring and alerting | Session state → event bus → notifications |
| **DNS** | DNS query logging | Per-zone query metrics |
| **DNS** | Zone transfer (AXFR) | Secondary DNS support |
| **CDN** | Auto-cert TLS (ACME) | Automatic Let's Encrypt certificates |
| **CDN** | Brotli compression | In addition to gzip |
| **CDN** | Disk-based response cache | Survive restarts, larger cache capacity |
| **Routes** | ICMP forwarding | Requires raw sockets / elevated privileges |
| **Routes** | Route health monitoring | Active probe + alerting |
| **Security** | Auth endpoint rate limiting | Brute-force protection |
| **Security** | Key rotation with versioned IDs | Graceful key rollover |
| **Events** | Webhook/Slack/email handlers | External event delivery |
| **Metrics** | Per-tenant usage metrics | Bandwidth, request counts per tenant |
| **UI** | Full web dashboard | Replace placeholder SPA |
| **Infra** | Kubernetes operator | CRD-based management |
| **Infra** | Plugin system | Extensible service runtimes |
| **Security** | WAF | Web application firewall for CDN |
