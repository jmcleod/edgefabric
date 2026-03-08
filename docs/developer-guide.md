# EdgeFabric Developer Guide

Everything you need to go from clone to running tests in under 10 minutes.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.22+ | CGO required (SQLite uses C bindings) |
| Node.js | 20+ | Required for SPA build (`web/console/`) |
| Task | 3.x | Task runner (`go install github.com/go-task/task/v3/cmd/task@latest`) |
| golangci-lint | 1.55+ | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| SQLite | 3.x | Typically pre-installed on macOS/Linux |
| Docker | 20.10+ | Optional — only for the demo environment |

On macOS:
```bash
brew install go node task golangci-lint
```

On Ubuntu/Debian:
```bash
# Go: follow https://go.dev/doc/install
# Node.js: follow https://nodejs.org/ or use nvm
# Task:
go install github.com/go-task/task/v3/cmd/task@latest
# golangci-lint:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
# SQLite:
sudo apt-get install -y libsqlite3-dev
```

## Quick Start

```bash
# Clone
git clone https://github.com/jmcleod/edgefabric.git
cd edgefabric

# Build
task build

# Run tests
task test

# Lint
task lint

# All checks (lint + vet + test)
task check
```

## Local Development Mode

### 1. Generate an encryption key

```bash
openssl rand -base64 32
```

### 2. Create a dev config

```bash
cat > config.dev.yaml <<EOF
role: controller
log_level: debug
controller:
  listen_addr: ":8443"
  external_url: "http://localhost:8443"
  storage:
    driver: sqlite
    dsn: "dev.db"
  wireguard:
    listen_port: 51820
    subnet: "10.100.0.0/16"
    address: "10.100.0.1/16"
  secrets:
    encryption_key: "<paste-your-key>"
EOF
```

### 3. Run the controller

```bash
task dev
```

Or build and run manually:
```bash
task build
./bin/edgefabric controller --config config.dev.yaml
```

On first start, a superuser is seeded and the password is logged:
```
level=WARN msg="seed superuser created — change this password immediately" email=admin@edgefabric.local password=XxXxXxXxXxXxXxXx
```

### 4. Test with curl

```bash
# Health check
curl http://localhost:8443/healthz

# Login
curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq
```

See the [API Guide](api-guide.md) for complete endpoint documentation.

## Project Structure

```
edgefabric/
├── cmd/edgefabric/        # Binary entrypoint (subcommands: controller, node, gateway)
├── internal/
│   ├── api/               # HTTP API layer
│   │   ├── middleware/     # Auth, RBAC, logging, metrics, security headers
│   │   ├── apiutil/       # Response helpers, ID parsing, JSON encoding
│   │   └── v1/            # API v1 handlers (one file per resource)
│   ├── app/               # Application wiring (controller.go, node.go, gateway.go, seed.go)
│   ├── audit/             # Audit event logging
│   ├── auth/              # Authentication: password hashing, JWT tokens, TOTP, API keys
│   ├── bgp/               # BGP reconciliation loop (node-side)
│   ├── cdn/               # CDN service: sites, origins (controller-side)
│   ├── cdnserver/         # CDN reverse proxy (node-side)
│   ├── config/            # YAML config loading + env var overrides
│   ├── controller/        # Controller-side agent coordination
│   ├── crypto/            # Cryptographic utilities
│   ├── dns/               # DNS service: zones, records (controller-side)
│   ├── dnsserver/         # DNS authoritative server (node-side)
│   ├── domain/            # Domain types: Node, Gateway, Tenant, User, etc.
│   ├── events/            # In-process event bus
│   ├── fleet/             # Fleet service: nodes, gateways, groups, SSH keys
│   ├── gateway/           # Gateway agent runtime
│   ├── gatewayrt/         # Gateway route forwarder (gateway-side)
│   ├── networking/        # Networking service: WireGuard, BGP sessions, IP allocations
│   ├── node/              # Node agent runtime
│   ├── observability/     # Structured logging, Prometheus metrics, health checks
│   ├── provisioning/      # Node provisioning: enrollment, lifecycle actions
│   ├── rbac/              # Role-based access control
│   ├── route/             # Route service: route CRUD, gateway config generation
│   ├── routeserver/       # Route forwarder (node-side)
│   ├── secrets/           # AES-256 encryption at rest
│   ├── ssh/               # SSH client for node provisioning
│   ├── storage/           # Storage interfaces
│   │   └── sqlite/        # SQLite implementation (all store interfaces)
│   ├── tenant/            # Tenant service
│   ├── user/              # User service
│   └── wireguard/         # WireGuard interface management
├── pkg/
│   └── version/           # Build-time version info (injected via ldflags)
├── web/
│   ├── console/           # SPA source (React + TypeScript + Vite)
│   └── static/            # SPA build output (//go:embed)
├── openapi/
│   └── v1.yaml            # OpenAPI 3.0.3 specification
├── deploy/
│   ├── docker/Dockerfile  # Multi-stage Docker build
│   └── systemd/           # Systemd unit files
├── demo/                  # Docker Compose demo environment
├── examples/              # Production-oriented example configs
├── docs/                  # Documentation
├── Taskfile.yml           # Build system
├── docker-compose.yml     # Demo environment
└── edgefabric.example.yaml
```

## Code Conventions

### Service Interfaces

Each domain area defines a `Service` interface in its own package. The default implementation lives in the same package:

```go
// dns/service.go
type Service interface {
    CreateZone(ctx context.Context, req CreateZoneRequest) (*domain.DNSZone, error)
    // ...
}

type DefaultService struct { /* store dependencies */ }
```

Controllers (API handlers) depend on service interfaces, enabling testing with mocks.

### Handler Registration

Each API handler follows the same pattern — a struct with a `Register` method:

```go
type NodeHandler struct {
    svc        fleet.Service
    authorizer rbac.Authorizer
    audit      audit.Logger
}

func (h *NodeHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
    requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceNode, ...)
    mux.Handle("POST /api/v1/nodes", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
    // ...
}
```

All handlers are registered in `internal/api/router.go`.

### Storage Interfaces

Storage is defined as fine-grained interfaces in `internal/storage/store.go`:

```go
type NodeStore interface {
    CreateNode(ctx context.Context, n *domain.Node) error
    GetNode(ctx context.Context, id domain.ID) (*domain.Node, error)
    // ...
}
```

The SQLite implementation in `internal/storage/sqlite/` satisfies all store interfaces through a single `Store` struct.

### Error Handling

- Wrap errors with context: `fmt.Errorf("create node: %w", err)`
- Use sentinel errors from `storage` package: `storage.ErrNotFound`, `storage.ErrAlreadyExists`
- API handlers map storage errors to HTTP status codes

### RBAC

Permissions are checked via middleware, not in service logic:

```go
requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceNode, middleware.TenantFromClaims())
```

Actions: `Create`, `Read`, `Update`, `Delete`. Resources: `Node`, `Gateway`, `Tenant`, `User`, etc.

### Audit Logging

State-changing API handlers log audit events:

```go
h.audit.Log(r.Context(), audit.Event{
    TenantID: claims.TenantID,
    UserID:   &claims.UserID,
    Action:   "create",
    Resource: "node",
    Details:  map[string]string{"node_id": node.ID.String()},
})
```

## Adding a New API Endpoint

Step-by-step example: adding a `Widget` resource.

### 1. Define domain types

```go
// internal/domain/widget.go
type Widget struct {
    ID       ID     `json:"id" db:"id"`
    TenantID ID     `json:"tenant_id" db:"tenant_id"`
    Name     string `json:"name" db:"name"`
    // ...
}
```

### 2. Add storage interface

```go
// internal/storage/store.go
type WidgetStore interface {
    CreateWidget(ctx context.Context, w *domain.Widget) error
    GetWidget(ctx context.Context, id domain.ID) (*domain.Widget, error)
    ListWidgets(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.Widget, int, error)
    DeleteWidget(ctx context.Context, id domain.ID) error
}
```

### 3. Implement SQLite storage

Create `internal/storage/sqlite/widget.go` following the pattern in existing files (e.g., `node.go`).

### 4. Add migration

Add a new migration in `internal/storage/sqlite/migrations.go`.

### 5. Create service

```go
// internal/widget/service.go
type Service interface { /* ... */ }
type DefaultService struct { widgets storage.WidgetStore }
```

### 6. Create API handler

```go
// internal/api/v1/widget.go
type WidgetHandler struct {
    svc        widget.Service
    authorizer rbac.Authorizer
    audit      audit.Logger
}
func (h *WidgetHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) { /* ... */ }
```

### 7. Wire it up

- Add `WidgetSvc` to `api.Services` in `internal/api/router.go`
- Register the handler in `NewRouter`
- Initialize the service in `internal/app/controller.go`

### 8. Add RBAC resource

Add the resource constant in `internal/rbac/rbac.go`.

### 9. Update OpenAPI spec

Add the endpoint to `openapi/v1.yaml`.

## Frontend Development

The web dashboard is a React 18 SPA built with TypeScript, Vite, Tailwind CSS, and shadcn-ui. Source lives in `web/console/`, and it builds to `web/static/` which is embedded into the Go binary via `//go:embed`.

```
web/
├── console/          # SPA source (React + TypeScript + Vite)
│   ├── src/
│   │   ├── components/   # UI components (shadcn-ui based)
│   │   ├── hooks/        # React Query data-fetching hooks
│   │   ├── lib/          # API client, transforms
│   │   ├── pages/        # Page components (one per route)
│   │   └── types/        # TypeScript types (SPA view-model + API types)
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.ts
├── static/           # Build output (embedded by Go)
└── embed.go          # //go:embed static
```

### Quick Start

```bash
# Install SPA dependencies
task install-spa

# Run Vite dev server with hot-reload (proxies /api to Go backend on :8443)
task dev-spa

# In another terminal, run the Go backend
task dev
```

The Vite dev server runs on `http://localhost:8080` and proxies API requests to the Go backend at `http://localhost:8443`. Changes to the SPA source are reflected instantly.

### Build

```bash
# Build SPA only (outputs to web/static/)
task build-spa

# Build everything (SPA + Go binary)
task build

# TypeScript typecheck
task check-spa
```

### SPA Architecture

**Data fetching** uses [TanStack React Query](https://tanstack.com/query) with one hook per API resource. Hooks live in `web/console/src/hooks/` and handle fetching, caching (30s stale time), and type transformation.

**Transform layer** in `web/console/src/lib/transforms.ts` converts backend API types (snake_case JSON) to SPA view-model types (camelCase). Status enums are mapped (e.g., backend `online` becomes SPA `healthy`). Fields not available from the backend (CPU, memory, bandwidth) display "\u2014".

**API client** in `web/console/src/lib/api.ts` wraps `fetch()` with JWT auth headers, response envelope unwrapping, and automatic redirect to `/login` on 401.

**Auth flow**: Login page posts to `/api/v1/auth/login`, stores the JWT token, then fetches user profile from `/api/v1/auth/me`. The `AuthProvider` context manages user state and the `RequireAuth` wrapper protects routes.

## Task Commands

| Command | Description |
|---------|-------------|
| `task build` | Build Go binary (includes SPA build) |
| `task build-go` | Build Go binary only (skip SPA) |
| `task build-spa` | Build SPA to web/static/ |
| `task build-static` | Build a static Linux binary |
| `task install-spa` | Install SPA npm dependencies |
| `task check-spa` | Run TypeScript type checks on SPA |
| `task dev-spa` | Run Vite dev server with hot-reload |
| `task test` | Run all Go tests with race detector |
| `task test-coverage` | Run tests with coverage report |
| `task lint` | Run golangci-lint |
| `task fmt` | Format code with gofumpt |
| `task vet` | Run go vet |
| `task tidy` | Tidy go modules |
| `task dev` | Build and run controller in dev mode |
| `task check` | Run all checks (SPA typecheck + lint + vet + test) |
| `task clean` | Remove build artifacts |

## Further Reading

- [Architecture](../ARCHITECTURE.md) — System architecture and design
- [API Guide](api-guide.md) — REST API with curl examples
- [Security Model](security-model.md) — Auth, RBAC, secrets
- [Domain Model](DOMAIN_MODEL.md) — Entity relationships
- [Networking Model](networking-model.md) — WireGuard overlay, BGP, IP allocations
- [Provisioning Model](provisioning-model.md) — Node enrollment and lifecycle
