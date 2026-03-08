# EdgeFabric Demo

A self-contained Docker Compose environment that starts the EdgeFabric controller and seeds it with sample data. Use this to evaluate the control-plane API without setting up real infrastructure.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) 20.10+
- [Docker Compose](https://docs.docker.com/compose/) v2+

## Quick Start

From the project root:

```bash
docker compose up --build
```

This will:

1. Build the EdgeFabric controller image
2. Start the controller with a demo configuration
3. Wait for the controller to become healthy
4. Seed the database with sample resources via the API

When complete, you'll see a summary of everything that was created.

## What Gets Created

| Resource      | Name                              | Details                                  |
|---------------|-----------------------------------|------------------------------------------|
| Tenant        | Acme Corp                         | slug: `acme`                             |
| User          | ops@acme.example                  | role: `admin`, password: `demo-password-123` |
| Node          | edge-us-east-1                    | 203.0.113.10, us-east-1, AWS             |
| Node          | edge-eu-west-1                    | 198.51.100.20, eu-west-1, AWS            |
| Node Group    | global-edge                       | Contains both edge nodes                 |
| Gateway       | gw-hq                             | 192.0.2.50                               |
| DNS Zone      | example.com                       | TTL 300, 3 records (A, MX, TXT)          |
| CDN Site      | www-cdn                           | Domains: www.acme.example, cdn.acme.example |
| CDN Origin    | origin.acme.example:443           | HTTPS, weight 100                        |
| Route         | ssh-to-hq                         | TCP 2222 → 10.0.1.100:22 via gw-hq      |

## Interacting with the API

After the demo-setup container finishes, the controller API is available at `http://localhost:8443`.

### Health Check

```bash
curl http://localhost:8443/healthz
```

### Login

Use the admin credentials printed in the setup summary (the password is randomly generated on first boot):

```bash
curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq
```

Save the token from the response:

```bash
TOKEN=$(curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq -r '.data.token')
```

### Browse Resources

```bash
# List nodes
curl -s http://localhost:8443/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN" | jq

# View status overview
curl -s http://localhost:8443/api/v1/status \
  -H "Authorization: Bearer $TOKEN" | jq

# OpenAPI spec
curl -s http://localhost:8443/api/v1/openapi.yaml
```

### Tenant User Login

You can also log in as the demo tenant user:

```bash
curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ops@acme.example","password":"demo-password-123"}' | jq
```

## Cleanup

```bash
docker compose down -v
```

This removes all containers and volumes, including the SQLite database.

## What This Demo Does NOT Cover

The demo exercises the full control-plane API (resource CRUD, auth, RBAC, status). It does **not** start actual node or gateway agents because those require:

- WireGuard overlay networking between controller and agents
- SSH access for node enrollment/provisioning
- Real or simulated network infrastructure

For a full multi-node deployment, see the [Deployment Guide](../docs/deployment-guide.md).
