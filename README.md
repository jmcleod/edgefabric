# EdgeFabric

A distributed edge networking platform that orchestrates a global fleet of nodes to deliver anycast IP services, authoritative DNS, basic CDN, and edge-to-private-network routing.

## Features

- **Anycast IP Services** — Announce IP prefixes via BGP from globally distributed nodes
- **Authoritative DNS** — Centrally managed DNS zones served from edge nodes
- **CDN** — Reverse proxy with caching, TLS termination, compression, and rate limiting
- **Edge-to-Private Routing** — Route public traffic through gateways into private networks
- **Multi-tenant** — Isolated tenants with RBAC and API key management
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

See [ARCHITECTURE.md](ARCHITECTURE.md) for full details.

## Quick Start

### Build

```bash
# Install task runner (https://taskfile.dev)
go install github.com/go-task/task/v3/cmd/task@latest

# Build
task build

# Run
./bin/edgefabric version
./bin/edgefabric controller --config edgefabric.example.yaml
```

### Development

```bash
task check    # Run lint + vet + tests
task test     # Run tests only
task lint     # Run golangci-lint
task fmt      # Format code
```

## Subcommands

```
edgefabric controller   # Run as controller (API server + management)
edgefabric node         # Run as edge node (BGP, DNS, CDN, routing)
edgefabric gateway      # Run as gateway (private network bridge)
edgefabric cli          # Management CLI client
edgefabric version      # Print version info
```

## Configuration

Copy `edgefabric.example.yaml` to `edgefabric.yaml` and edit for your deployment. See the example file for all available options.

## Project Structure

```
cmd/edgefabric/         CLI entrypoint
internal/
  app/                  Application wiring per role
  domain/               Domain types (entities, value objects)
  config/               Configuration loading
  storage/              Persistence abstraction
    sqlite/             SQLite driver
    postgres/           PostgreSQL driver (planned)
  auth/                 Authentication
  rbac/                 Role-based access control
  tenant/               Tenant management
  fleet/                Node/Gateway inventory
  provisioning/         SSH-based deployment
  wireguard/            WireGuard overlay management
  bgp/                  BGP via GoBGP
  dns/                  Authoritative DNS
  cdn/                  CDN reverse proxy
  route/                Traffic routing
  audit/                Audit logging
  observability/        Logging, metrics, health
  crypto/               Encryption utilities
  secrets/              Secret storage
pkg/version/            Build version info
web/                    Embedded SPA
deploy/
  docker/               Dockerfile
  systemd/              systemd unit files
docs/                   Documentation
openapi/                OpenAPI specs
```

## Documentation

- [Architecture](ARCHITECTURE.md)
- [Domain Model](docs/DOMAIN_MODEL.md)
- [Roadmap](ROADMAP.md)

## License

MIT — see [LICENSE](LICENSE).
