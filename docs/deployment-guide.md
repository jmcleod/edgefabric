# EdgeFabric Deployment Guide

Production deployment reference for operators. Covers controller, node, and gateway installation, configuration, and monitoring.

## Architecture Overview

```
┌─────────────────┐
│   Controller     │  Central management plane
│  :8443 (API)     │  Stores state in SQLite/Postgres
│  :51820/udp (WG) │  WireGuard hub for overlay mesh
└────────┬────────┘
         │ WireGuard overlay (10.100.0.0/16)
    ┌────┴────┐
    │         │
┌───┴───┐ ┌──┴────┐
│ Node  │ │Gateway│   Edge agents
│:9090  │ │:9090  │   Poll controller for config
│:5353  │ └───────┘   Report heartbeats
│:8080  │
└───────┘
```

**Controller**: Single process, runs the API server and WireGuard hub. All state in one database.

**Nodes**: Edge compute agents. Run DNS, CDN, BGP, and route forwarding services. Connect to controller over WireGuard.

**Gateways**: Traffic entry points. Forward inbound TCP/UDP to backend services through the mesh.

## Controller Deployment

### 1. Install Binary

```bash
# From source
git clone https://github.com/jmcleod/edgefabric.git
cd edgefabric
CGO_ENABLED=1 go build -ldflags "-s -w" -o /usr/local/bin/edgefabric ./cmd/edgefabric

# Or use Docker (see Docker Deployment below)
```

### 2. Create System User

```bash
sudo useradd -r -s /usr/sbin/nologin -m -d /var/lib/edgefabric edgefabric
sudo mkdir -p /etc/edgefabric /var/lib/edgefabric
sudo chown edgefabric:edgefabric /var/lib/edgefabric
```

### 3. Generate Keys

```bash
# Encryption key (AES-256, for secrets at rest)
ENCRYPTION_KEY=$(openssl rand -base64 32)

# Token signing key (JWT HMAC, separate from encryption)
SIGNING_KEY=$(openssl rand -base64 32)

echo "encryption_key: $ENCRYPTION_KEY"
echo "signing_key:    $SIGNING_KEY"
```

**Store these keys securely.** Loss of the encryption key means encrypted data (WireGuard keys, TOTP secrets) is unrecoverable.

### 4. Create Configuration

```bash
sudo cp examples/controller.yaml /etc/edgefabric/edgefabric.yaml
sudo chmod 600 /etc/edgefabric/edgefabric.yaml
sudo chown edgefabric:edgefabric /etc/edgefabric/edgefabric.yaml
```

Edit `/etc/edgefabric/edgefabric.yaml`:

```yaml
role: controller
log_level: info

controller:
  listen_addr: ":8443"
  external_url: "https://your-controller.example.com:8443"

  storage:
    driver: sqlite
    dsn: "/var/lib/edgefabric/edgefabric.db"

  tls:
    enabled: true
    cert_file: "/etc/edgefabric/tls/cert.pem"
    key_file: "/etc/edgefabric/tls/key.pem"

  wireguard:
    listen_port: 51820
    subnet: "10.100.0.0/16"
    address: "10.100.0.1/16"

  secrets:
    encryption_key: "<your-encryption-key>"
    token_signing_key: "<your-signing-key>"
```

See [examples/controller.yaml](../examples/controller.yaml) for all fields with documentation.

### 5. Install systemd Service

```bash
sudo cp deploy/systemd/edgefabric-controller.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable edgefabric-controller
```

### 6. First Boot

```bash
sudo systemctl start edgefabric-controller
sudo journalctl -u edgefabric-controller -f
```

On first boot, a superuser is created and the password is logged **once**:

```
level=WARN msg="seed superuser created — change this password immediately" email=admin@edgefabric.local password=XxXxXxXxXxXxXxXx
```

**Copy this password immediately.** It is not stored anywhere in plaintext. Change it via the API:

```bash
# Login with the seed password
TOKEN=$(curl -s -X POST http://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<seed-password>"}' | jq -r '.data.token')

# Update password
curl -s -X PUT http://localhost:8443/api/v1/users/<admin-user-id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password":"<new-secure-password>"}'
```

## Node Deployment

### 1. Install and Configure

```bash
# Install binary (same as controller)
sudo cp edgefabric /usr/local/bin/
sudo cp examples/node.yaml /etc/edgefabric/edgefabric.yaml
```

Edit the config:
```yaml
role: node
node:
  controller_addr: "10.100.0.1:8443"
  data_dir: /var/lib/edgefabric
  health_addr: ":9090"

  dns:
    enabled: true
    mode: "miekg"
    listen_addr: ":5353"

  cdn:
    enabled: true
    mode: "proxy"
    listen_addr: ":8080"
```

### 2. Provision via Controller

Create the node in the controller first, then trigger enrollment:

```bash
# On controller: create node
NODE=$(curl -s -X POST https://controller:8443/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"edge-1","hostname":"edge1.example.com","public_ip":"203.0.113.10"}' | jq -r '.data.id')

# Trigger enrollment (generates enrollment token, provisions via SSH)
curl -s -X POST "https://controller:8443/api/v1/nodes/$NODE/enroll" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 3. Install systemd Service

```bash
sudo cp deploy/systemd/edgefabric-node.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now edgefabric-node
```

## Gateway Deployment

### 1. Install and Configure

```bash
sudo cp edgefabric /usr/local/bin/
sudo cp examples/gateway.yaml /etc/edgefabric/edgefabric.yaml
```

Edit:
```yaml
role: gateway
gateway:
  controller_addr: "10.100.0.1:8443"
  data_dir: /var/lib/edgefabric
  health_addr: ":9090"
  route_mode: "forwarder"
```

### 2. Install systemd Service

```bash
sudo cp deploy/systemd/edgefabric-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now edgefabric-gateway
```

## Docker Deployment

### Build Image

```bash
docker build -f deploy/docker/Dockerfile -t edgefabric:latest .
```

### Run Controller

```bash
docker run -d \
  --name edgefabric-controller \
  -p 8443:8443 \
  -p 51820:51820/udp \
  -v edgefabric-data:/var/lib/edgefabric \
  -v /path/to/edgefabric.yaml:/etc/edgefabric/edgefabric.yaml:ro \
  edgefabric:latest
```

Or use the [Docker Compose demo](../demo/README.md) for quick evaluation.

### Environment Variables

All config fields have `EF_`-prefixed env var overrides (useful for container deployments):

| Variable | Config Field |
|----------|-------------|
| `EF_ROLE` | `role` |
| `EF_LOG_LEVEL` | `log_level` |
| `EF_CONTROLLER_LISTEN_ADDR` | `controller.listen_addr` |
| `EF_CONTROLLER_EXTERNAL_URL` | `controller.external_url` |
| `EF_CONTROLLER_STORAGE_DRIVER` | `controller.storage.driver` |
| `EF_CONTROLLER_STORAGE_DSN` | `controller.storage.dsn` |
| `EF_CONTROLLER_SECRETS_ENCRYPTION_KEY` | `controller.secrets.encryption_key` |
| `EF_CONTROLLER_SECRETS_TOKEN_SIGNING_KEY` | `controller.secrets.token_signing_key` |
| `EF_NODE_CONTROLLER_ADDR` | `node.controller_addr` |
| `EF_NODE_ENROLLMENT_TOKEN` | `node.enrollment_token` |
| `EF_NODE_DATA_DIR` | `node.data_dir` |
| `EF_GATEWAY_CONTROLLER_ADDR` | `gateway.controller_addr` |
| `EF_GATEWAY_ENROLLMENT_TOKEN` | `gateway.enrollment_token` |
| `EF_GATEWAY_DATA_DIR` | `gateway.data_dir` |

## Configuration Reference

See the fully commented example configs:

- [examples/controller.yaml](../examples/controller.yaml) — All controller fields
- [examples/node.yaml](../examples/node.yaml) — All node fields including service modes
- [examples/gateway.yaml](../examples/gateway.yaml) — All gateway fields

## Security Checklist

### TLS

- [ ] Enable TLS on the controller API (`controller.tls.enabled: true`)
- [ ] Use valid certificates (Let's Encrypt or internal CA)
- [ ] Ensure `external_url` uses `https://`

### Keys

- [ ] Generate unique `encryption_key` and `token_signing_key`
- [ ] Store keys in a secrets manager (Vault, AWS SSM, etc.)
- [ ] Never commit keys to version control
- [ ] Rotate signing key periodically (tokens expire in 24h)

### File Permissions

```bash
# Config file (contains keys)
chmod 600 /etc/edgefabric/edgefabric.yaml
chown edgefabric:edgefabric /etc/edgefabric/edgefabric.yaml

# Data directory
chmod 700 /var/lib/edgefabric
chown edgefabric:edgefabric /var/lib/edgefabric

# TLS certificates
chmod 600 /etc/edgefabric/tls/*
chown edgefabric:edgefabric /etc/edgefabric/tls/*
```

### Firewall

| Port | Protocol | Direction | Purpose |
|------|----------|-----------|---------|
| 8443 | TCP | Inbound | API / Web UI |
| 51820 | UDP | Inbound | WireGuard overlay |
| 5353 | TCP+UDP | Inbound (nodes) | DNS authoritative server |
| 8080 | TCP | Inbound (nodes) | CDN reverse proxy |
| 9090 | TCP | Inbound (nodes/gateways) | Health/metrics |

Restrict WireGuard port to known node/gateway IPs where possible.

### RBAC

- [ ] Change the seed admin password immediately after first boot
- [ ] Create tenant-scoped admin users for each team
- [ ] Use `readonly` role for monitoring/dashboard users
- [ ] Use API keys (not user tokens) for agent heartbeats and CI/CD

See [Security Model](security-model.md) and [Tenancy & RBAC](tenancy-and-rbac.md) for details.

## Monitoring

### Health Endpoints

| Endpoint | Auth | Purpose |
|----------|------|---------|
| `/healthz` | No | Dependency health (storage ping) — use for load balancer checks |
| `/readyz` | No | Readiness probe — returns 200 when all checks pass |
| `/livez` | No | Liveness probe — always returns 200 |
| `/metrics` | No | Prometheus metrics |

### Key Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `edgefabric_http_requests_total` | Counter | Total API requests by method, path, status |
| `edgefabric_http_duration_seconds` | Histogram | Request latency |
| `edgefabric_auth_failures_total` | Counter | Authentication failures by type |
| `edgefabric_active_nodes` | Gauge | Current node count |
| `edgefabric_active_gateways` | Gauge | Current gateway count |
| `edgefabric_active_tenants` | Gauge | Current tenant count |

### Alerting Suggestions

| Alert | Condition | Severity |
|-------|-----------|----------|
| Controller down | `/healthz` returns non-200 for > 30s | Critical |
| High error rate | `rate(edgefabric_http_requests_total{status=~"5.."}[5m]) > 0.05` | Warning |
| Auth failures | `rate(edgefabric_auth_failures_total[5m]) > 10` | Warning |
| Node stale config | `stale_node_count > 0` in `/api/v1/status` | Info |

### Status Dashboard

The `/api/v1/status` endpoint (authenticated) returns a fleet overview:

```bash
curl -s http://localhost:8443/api/v1/status \
  -H "Authorization: Bearer $TOKEN" | jq
```

Includes: node/gateway counts, status breakdowns, stale config counts, route/DNS/CDN totals.

## Backup and Restore

See [Operations Guide](operations.md) for backup procedures, online backup with SQLite, encryption key management, and upgrade procedures.

## Further Reading

- [Architecture](../ARCHITECTURE.md) — System design
- [Security Model](security-model.md) — Auth, RBAC, encryption
- [Networking Model](networking-model.md) — WireGuard, BGP, IP allocations
- [Operations](operations.md) — Backup, restore, upgrades
- [API Guide](api-guide.md) — REST API reference
