# EdgeFabric Comprehensive Demo

A self-contained Docker Compose environment that runs the full EdgeFabric platform: controller, 2 edge nodes with real CDN and DNS, a gateway, an origin server, and a VPS for enrollment demos. Two tenants are provisioned with DNS zones, CDN sites, routes, BGP sessions, and IP allocations.

## Architecture

```
                        ┌─────────────────────────────────────────────────────┐
                        │                Docker Network (172.20.0.0/24)       │
                        │                                                     │
  :8443 ───────────────►│  ┌──────────────┐                                   │
                        │  │  Controller   │◄──── polls ─────┐                │
                        │  │  172.20.0.2   │◄──── polls ────┐│                │
                        │  │  + SPA + API  │                ││                │
                        │  └──────┬────────┘                ││                │
                        │         │ seeds data              ││                │
                        │  ┌──────▼────────┐                ││                │
                        │  │  demo-setup   │                ││                │
                        │  │  (exits)      │                ││                │
                        │  └──────┬────────┘                ││                │
                        │         │ writes tokens           ││                │
                        │         ▼                         ││                │
  :8081 (CDN) ─────────►│  ┌──────────────┐  ┌──────────────┤│                │
  :5354 (DNS) ─────────►│  │   Node 1     │  │   Node 2     ││                │
                        │  │  172.20.0.10  │  │  172.20.0.11 ││                │
                        │  │  CDN+DNS+BGP  │  │  CDN+DNS+BGP │┘                │
  :8082 (CDN) ─────────►│  └──────┬───────┘  └──────┬───────┘                │
  :5355 (DNS) ─────────►│         │ proxy            │ proxy                  │
                        │         ▼                  ▼                        │
  :8888 ───────────────►│  ┌──────────────────────────────┐                   │
                        │  │  Origin Server (Nginx)       │                   │
                        │  │  172.20.0.100                │                   │
                        │  └──────────────────────────────┘                   │
                        │                                                     │
  :9190 ───────────────►│  ┌──────────────┐  ┌──────────────┐                 │
                        │  │   Gateway     │  │     VPS      │                 │
                        │  │  172.20.0.50  │  │  172.20.0.12 │◄──── :2222 SSH │
                        │  │  (noop mode)  │  │  enrollment  │                 │
                        │  └──────────────┘  └──────────────┘                 │
                        └─────────────────────────────────────────────────────┘
```

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) 20.10+
- [Docker Compose](https://docs.docker.com/compose/) v2+

## Quick Start

From the project root:

```bash
docker compose up --build
```

Wait for the `demo-setup` container to print the summary (~45 seconds), then all services are ready.

To tear everything down:

```bash
docker compose down -v
```

## Port Map

| Port  | Service           | Protocol | Description                        |
|-------|-------------------|----------|------------------------------------|
| 8443  | Controller + SPA  | HTTP     | API server and web console         |
| 8081  | Node 1 CDN        | HTTP     | Reverse proxy (use Host header)    |
| 8082  | Node 2 CDN        | HTTP     | Reverse proxy (use Host header)    |
| 5354  | Node 1 DNS        | TCP/UDP  | Authoritative DNS server           |
| 5355  | Node 2 DNS        | TCP/UDP  | Authoritative DNS server           |
| 8888  | Origin            | HTTP     | Direct access to origin (Nginx)    |
| 9190  | Gateway           | HTTP     | Health check and metrics           |
| 2222  | VPS               | SSH      | Enrollment demo (root/demo)        |

## What Gets Created

The setup script seeds ~35 resources across 2 tenants:

### Tenants & Users

| Tenant      | User                    | Password          | Role  |
|-------------|-------------------------|--------------------|-------|
| Acme Corp   | ops@acme.example        | demo-password-123  | admin |
| Globex Inc  | admin@globex.example    | demo-password-456  | admin |

The superuser account (`admin@edgefabric.local`) has a randomly generated password printed to the setup summary.

### Infrastructure

| Resource     | Name              | Details                                              |
|--------------|-------------------|------------------------------------------------------|
| Node         | edge-us-east-1    | 172.20.0.10, us-east-1 (Acme)                       |
| Node         | edge-eu-west-1    | 172.20.0.11, eu-west-1 (Acme)                       |
| Node         | vps-staging       | 172.20.0.12, us-west-2 (enrollment demo)            |
| Node Group   | acme-global-edge  | node-1 + node-2 (Acme Corp)                         |
| Node Group   | globex-edge       | node-1 + node-2 (Globex — shared infrastructure)    |
| Gateway      | gw-hq             | 172.20.0.50 (Acme Corp)                             |
| Gateway      | gw-branch         | 192.0.2.60 (Globex Inc)                             |

### Services

| Resource     | Tenant  | Name/Domain                          | Details                    |
|--------------|---------|--------------------------------------|----------------------------|
| DNS Zone     | Acme    | acme.example                         | 5 records (A, CNAME, MX, TXT) |
| DNS Zone     | Globex  | globex.example                       | 4 records (A, MX, TXT)    |
| CDN Site     | Acme    | www.acme.example, cdn.acme.example   | Cached, origin: Nginx      |
| CDN Site     | Globex  | api.globex.example                   | Pass-through, origin: Nginx|
| Route        | Acme    | ssh-to-hq                            | TCP/2222 via gw-hq         |
| Route        | Globex  | db-access                            | TCP/5432 via gw-branch     |
| BGP Session  | —       | node-1 ↔ AS64512                    | 203.0.113.0/24 (noop)      |
| BGP Session  | —       | node-2 ↔ AS64513                    | 198.51.100.0/24 (noop)     |
| IP Alloc     | Acme    | 203.0.113.0/24                       | Anycast                    |
| IP Alloc     | Globex  | 198.51.100.0/24                      | Anycast                    |

## Service Modes

Not all services can run fully in a Docker environment. Here's what's real vs simulated:

| Service | Mode       | Status | Why                                           |
|---------|------------|--------|-----------------------------------------------|
| CDN     | `proxy`    | Real   | HTTP reverse proxy works without WireGuard    |
| DNS     | `miekg`    | Real   | Authoritative DNS works without WireGuard     |
| BGP     | `noop`     | Sim    | GoBGP needs kernel routing tables             |
| Route   | `noop`     | Sim    | Forwarder needs WireGuard overlay             |
| Gateway | `noop`     | Sim    | Gateway reconciliation requires WireGuard     |

## Demo Walkthrough

### 1. CDN Caching

The CDN reverse proxy serves content from the origin server with caching:

```bash
# First request — cache MISS (proxied to origin)
curl -v -H "Host: www.acme.example" http://localhost:8081

# Second request — cache HIT (served from node's LRU cache)
curl -v -H "Host: www.acme.example" http://localhost:8081

# Globex tenant CDN (pass-through, no caching)
curl -v -H "Host: api.globex.example" http://localhost:8082
```

Look for `X-Cache: HIT` / `X-Cache: MISS` headers in the response.

### 2. DNS Queries

Query the authoritative DNS servers running on the edge nodes:

```bash
# Acme Corp zones on node-1
dig @localhost -p 5354 www.acme.example A
dig @localhost -p 5354 acme.example MX
dig @localhost -p 5354 acme.example TXT

# Globex zones on node-2
dig @localhost -p 5355 www.globex.example A
dig @localhost -p 5355 api.globex.example A
```

### 3. Origin Server

Access the demo page directly (bypassing CDN):

```bash
curl http://localhost:8888
```

Open `http://localhost:8888` in a browser to see the demo page with request info.

### 4. VPS Enrollment Demo

SSH into the VPS container and perform manual node enrollment:

```bash
ssh root@localhost -p 2222
# Password: demo

# Inside the VPS:
cat /shared/vps.token
edgefabric node --config /etc/edgefabric/edgefabric.yaml
# (Set EF_NODE_ENROLLMENT_TOKEN env var first)
```

### 5. Web Console

Open `http://localhost:8443` in your browser and log in with the superuser credentials from the setup summary. Switch between superuser and tenant views to see different dashboards.

### 6. API Exploration

```bash
# Get the admin password from docker logs
docker compose logs demo-setup | grep "Superuser"

# Login
TOKEN=$(curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq -r '.data.token')

# Browse resources
curl -s http://localhost:8443/api/v1/nodes -H "Authorization: Bearer $TOKEN" | jq
curl -s http://localhost:8443/api/v1/status -H "Authorization: Bearer $TOKEN" | jq

# OpenAPI spec
curl -s http://localhost:8443/api/v1/openapi.yaml
```

## Troubleshooting

### Nodes not enrolling

Check that the demo-setup container completed successfully:

```bash
docker compose logs demo-setup
```

The enrollment tokens are written to the shared volume. Nodes wait for their token file before starting.

### CDN returns 502

The CDN needs ~30 seconds after node startup for the first reconciliation loop to fetch config from the controller. Wait and retry.

### DNS returns SERVFAIL

Same as CDN — the DNS server needs a reconciliation cycle to load zones from the controller. Wait 30 seconds.

### Port conflicts

If any ports are already in use, edit the port mappings in `docker-compose.yml`.

### Full reset

```bash
docker compose down -v
docker compose up --build
```

The `-v` flag removes all volumes including the SQLite database, ensuring a clean start.
