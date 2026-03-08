# EdgeFabric

[![CI](https://github.com/jmcleod/edgefabric/actions/workflows/ci.yml/badge.svg)](https://github.com/jmcleod/edgefabric/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A distributed edge networking platform that orchestrates a global fleet of nodes to deliver anycast IP services, authoritative DNS, CDN, and edge-to-private-network routing — all managed from a single control plane.

## Features

- **Fleet Management** — Register, provision, and monitor edge nodes and gateways with heartbeat tracking and config drift detection
- **WireGuard Mesh** — Automatic overlay network between controller, nodes, and gateways with encrypted tunnels and key management
- **BGP Anycast** — Announce IP prefixes via BGP from globally distributed nodes for geographic load distribution
- **Authoritative DNS** — Centrally managed DNS zones (A, AAAA, CNAME, MX, TXT, SRV, CAA, NS, PTR) served from edge nodes
- **CDN** — Reverse proxy with caching, TLS termination (auto/manual/disabled), compression, rate limiting, and origin health checks
- **Route Forwarding** — TCP/UDP port forwarding through gateways into private networks
- **Multi-tenant** — Isolated tenants with role-based access control (superuser, admin, readonly)
- **Provisioning** — SSH-based node enrollment, start/stop/restart/upgrade/decommission lifecycle
- **Authentication** — JWT tokens, TOTP two-factor, API keys for programmatic access
- **Audit Logging** — Every state-changing operation is recorded
- **Observability** — Structured logging, Prometheus metrics, health/readiness/liveness endpoints
- **Single Binary** — One Go binary runs as controller, node, or gateway

## Architecture

```
┌──────────────┐     WireGuard      ┌──────────┐
│  Controller  │◄──────────────────►│  Node A  │──► BGP, DNS, CDN
│  (API + UI)  │◄──────────┐       └──────────┘
└──────────────┘           │        ┌──────────┐
                           ├───────►│  Node B  │──► BGP, DNS, CDN
                           │        └──────────┘
                           │        ┌──────────┐     ┌─────────────┐
                           └───────►│ Gateway  │────►│ Private Net │
                                    └──────────┘     └─────────────┘
```

The **controller** is the central management plane: API server, WireGuard hub, database (SQLite or Postgres). **Nodes** are edge compute agents that run DNS, CDN, BGP, and route forwarding services. **Gateways** are traffic entry points that forward inbound connections into private networks.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## Quick Start

### Docker (Recommended for Evaluation)

```bash
docker compose up --build
```

This builds the controller, starts it, and seeds it with a sample tenant, users, nodes, gateways, DNS zones, CDN sites, and routes. See [demo/README.md](demo/README.md) for details.

After setup completes:

```bash
# Health check
curl http://localhost:8443/healthz

# Login (password is printed in the demo output)
curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq
```

Cleanup:

```bash
docker compose down -v
```

### Build from Source

```bash
# Prerequisites: Go 1.22+, Task (https://taskfile.dev)
go install github.com/go-task/task/v3/cmd/task@latest

# Build
task build
./bin/edgefabric version

# Generate encryption key
openssl rand -base64 32

# Create config (see examples/controller.yaml for all options)
cp edgefabric.example.yaml edgefabric.yaml
# Edit edgefabric.yaml: set encryption_key

# Run controller
./bin/edgefabric controller --config edgefabric.yaml
```

On first boot, the admin password is logged once to stdout.

## Subcommands

```
edgefabric controller   # Run the control plane (API + WireGuard hub)
edgefabric node         # Run an edge node (DNS, CDN, BGP, routing)
edgefabric gateway      # Run a gateway (private network bridge)
edgefabric version      # Print version info
```

## Configuration

Copy `edgefabric.example.yaml` and edit for your deployment. Detailed, commented configs for each role are in `examples/`:

- [`examples/controller.yaml`](examples/controller.yaml) — Controller with all options documented
- [`examples/node.yaml`](examples/node.yaml) — Node with all service modes (DNS, CDN, BGP, routing)
- [`examples/gateway.yaml`](examples/gateway.yaml) — Gateway with route forwarding

All settings support `EF_`-prefixed environment variable overrides for container deployments.

## Documentation

| Document | Audience | Description |
|----------|----------|-------------|
| [API Guide](docs/api-guide.md) | Developers | REST API with curl examples for every endpoint |
| [Developer Guide](docs/developer-guide.md) | Contributors | Clone → test in 10 minutes, code conventions, adding endpoints |
| [Deployment Guide](docs/deployment-guide.md) | Operators | Production install, systemd, Docker, security checklist |
| [Architecture](ARCHITECTURE.md) | Everyone | System design and component overview |
| [Security Model](docs/security-model.md) | Operators | Authentication, RBAC, encryption at rest |
| [Tenancy & RBAC](docs/tenancy-and-rbac.md) | Operators | Multi-tenant isolation and role model |
| [Networking Model](docs/networking-model.md) | Operators | WireGuard overlay, BGP, IP allocations |
| [Provisioning Model](docs/provisioning-model.md) | Operators | Node enrollment and lifecycle |
| [Domain Model](docs/DOMAIN_MODEL.md) | Contributors | Entity relationships |
| [Operations](docs/operations.md) | Operators | Backup, restore, upgrades |
| [OpenAPI Spec](openapi/v1.yaml) | Developers | Full API specification (3,300+ lines) |
| [Roadmap](ROADMAP.md) | Everyone | Milestone status and future plans |
| [Demo](demo/README.md) | Evaluators | Docker Compose demo environment |

## Project Structure

```
cmd/edgefabric/           CLI entrypoint (controller, node, gateway subcommands)
internal/
  api/                    HTTP API layer (router, middleware, v1 handlers)
  app/                    Application wiring per role
  domain/                 Domain types (entities, value objects)
  config/                 YAML config + env var overrides
  storage/sqlite/         SQLite persistence (all store interfaces)
  auth/                   JWT, TOTP, API keys, password hashing
  rbac/                   Role-based access control
  tenant/                 Tenant management
  user/                   User management
  fleet/                  Node/gateway/group/SSH key inventory
  provisioning/           SSH-based node enrollment + lifecycle
  networking/             WireGuard, BGP sessions, IP allocations
  dns/                    DNS zone + record management
  cdn/                    CDN site + origin management
  route/                  Route CRUD + gateway config generation
  audit/                  Audit event logging
  events/                 In-process event bus
  observability/          Structured logging, Prometheus, health checks
  secrets/                AES-256 encryption at rest
  ssh/                    SSH client for provisioning
  wireguard/              WireGuard interface management
  bgp/, dnsserver/, cdnserver/, routeserver/, gatewayrt/
                          Node/gateway-side service runtimes
pkg/version/              Build-time version info
web/static/               Embedded SPA (served at /)
deploy/docker/            Dockerfile (multi-stage alpine build)
deploy/systemd/           Hardened systemd units
demo/                     Docker Compose demo environment
examples/                 Production example configs
docs/                     Documentation
openapi/                  OpenAPI 3.0.3 specification
```

## Development

```bash
task check    # lint + vet + test (full CI equivalent)
task test     # tests with race detector
task lint     # golangci-lint
task dev      # build and run controller with config.dev.yaml
```

See the [Developer Guide](docs/developer-guide.md) for the full setup walkthrough.

## License

MIT — see [LICENSE](LICENSE).
