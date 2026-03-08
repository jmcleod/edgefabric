# EdgeFabric v1 Roadmap

## Implementation Plan

Work proceeds in milestones. Each milestone should result in a working (if incomplete) system that builds on the previous one. Milestones are ordered by dependency — later milestones depend on earlier ones.

---

### Milestone 0: Project Foundation
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

### Milestone 1: Controller Core
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

### Milestone 2: Fleet Management
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

### Milestone 3: WireGuard Overlay
**Goal**: Controller can generate WireGuard configs, nodes/gateways can connect.

- [x] WireGuard key generation (controller-side)
- [x] WireGuard peer configuration management
- [x] Overlay IP allocation (10.100.0.0/16 default)
- [x] WireGuard interface setup on node/gateway
- [x] Hub-spoke topology: controller is hub
- [x] Key rotation API
- [ ] Connectivity health checks over overlay

---

### Milestone 4: Provisioning
**Goal**: Controller can deploy the binary and config to nodes over SSH.

- [x] SSH client with key auth
- [x] Binary upload via SCP/SFTP
- [x] Config generation and push
- [x] systemd unit generation and installation
- [x] Enrollment token generation and validation
- [x] Node bootstrap flow (enroll → configure → start)
- [ ] Binary version tracking per node
- [ ] Rolling update support

---

### Milestone 5: BGP
**Goal**: Nodes can announce anycast prefixes to upstream peers.

- [x] GoBGP integration as library
- [x] BGP session configuration from controller
- [x] Prefix announcement management
- [x] BGP session status reporting
- [x] Session lifecycle (start, stop, reconfigure)
- [ ] BGP session monitoring and alerting

---

### Milestone 6: Anycast DNS
**Goal**: Nodes serve authoritative DNS for tenant zones.

- [ ] DNS zone CRUD API on controller
- [ ] DNS record CRUD API
- [ ] Zone data sync from controller to nodes
- [ ] Authoritative DNS server on nodes (miekg/dns)
- [ ] SOA serial management
- [ ] Zone transfer/sync protocol (push-based over WireGuard)
- [ ] DNS query logging

---

### Milestone 7: CDN
**Goal**: Nodes act as caching reverse proxies for tenant sites.

- [ ] CDN site configuration API
- [ ] Origin management API
- [ ] Reverse proxy with httputil.ReverseProxy
- [ ] Disk-based response cache
- [ ] Cache key generation
- [ ] Cache TTL and invalidation
- [ ] Cache purge API
- [ ] TLS termination (auto-cert or manual)
- [ ] Origin health checks
- [ ] Header manipulation (add/remove/rewrite)
- [ ] Compression (gzip, brotli)
- [ ] Rate limiting per-site
- [ ] CDN config sync from controller to nodes

---

### Milestone 8: Route/Gateway
**Goal**: Traffic routing from anycast entry through nodes to gateways into private networks.

- [ ] Route CRUD API on controller
- [ ] Route config sync to nodes and gateways
- [ ] TCP/UDP proxy listener on nodes
- [ ] ICMP forwarding
- [ ] Packet forwarding over WireGuard to gateway
- [ ] Gateway forwarding to private destinations
- [ ] RFC1918 routing
- [ ] Connection tracking / NAT for return traffic
- [ ] Route health monitoring

---

### Milestone 9: Observability & Audit
**Goal**: Production-grade operational visibility.

- [ ] Prometheus metrics for all services
- [ ] Structured logging throughout
- [ ] Audit log API (query, filter, export)
- [ ] Alerting hooks (webhook-based)
- [ ] Node health dashboard data
- [ ] Per-tenant usage metrics
- [ ] Log aggregation guidance

---

### Milestone 10: Packaging & Demo
**Goal**: Ready for external users.

- [ ] Dockerfile (multi-stage, static binary)
- [ ] docker-compose demo environment
- [ ] systemd unit templates
- [ ] Sample configuration files
- [ ] Getting started guide
- [ ] API documentation
- [ ] Web UI (placeholder/minimal SPA)
- [ ] README with badges

---

## Current Status

**Active Milestone**: 6 (Anycast DNS) — Milestones 0–5 are substantially complete (see items above for remaining work).

## Priority Order

Milestones 0-4 are the critical path — they establish the platform that all services run on. Milestones 5-8 can be partially parallelized once the foundation is solid. Milestone 9 is continuous (observability should be added incrementally). Milestone 10 wraps up.

## Out of Scope for v1

- HA Controller
- IPv6
- WAF
- Full mesh WireGuard
- Dynamic routing on gateways
- Kubernetes anything
- Plugin system
- Web UI beyond placeholder SPA
