# EdgeFabric Operations Guide

This document covers backup/restore, upgrades, health monitoring, and key metrics for operating an EdgeFabric deployment.

## Backup and Restore

### Controller (SQLite)

EdgeFabric uses SQLite with WAL mode. To take a consistent backup:

1. **Stop the controller** (recommended for consistency):
   ```bash
   # Stop the controller process
   systemctl stop edgefabric-controller

   # Copy database files
   cp /path/to/edgefabric.db /backup/edgefabric.db
   cp /path/to/edgefabric.db-wal /backup/edgefabric.db-wal 2>/dev/null
   cp /path/to/edgefabric.db-shm /backup/edgefabric.db-shm 2>/dev/null

   # Restart
   systemctl start edgefabric-controller
   ```

2. **Online backup** (using SQLite `.backup` command):
   ```bash
   sqlite3 /path/to/edgefabric.db ".backup /backup/edgefabric.db"
   ```

3. **Back up the encryption key separately**. The key in `controller.secrets.encryption_key` is required to decrypt TOTP secrets and SSH key passphrases. Store it in a secure vault, not alongside the database backup.

### Restore

1. Stop the controller.
2. Replace the database file with the backup.
3. Ensure the encryption key matches the one used when the backup was created.
4. Start the controller. Schema migrations will run automatically (idempotent).

## Upgrades

### Schema Migrations

- Migrations are **forward-only** and **idempotent** (`CREATE TABLE IF NOT EXISTS`).
- Applied migrations are tracked in the `schema_versions` table.
- Running `Migrate()` on a current database is a no-op.
- **No rollback support in v1**. Always back up before upgrading.

### Version Compatibility

- Node and gateway binary versions should match the controller version.
- The controller will accept older node/gateway versions but new features may not work.
- Check `GET /api/v1/status` for the controller's version, commit, and build time.

### Upgrade Procedure

1. Back up the controller database and encryption key.
2. Stop the controller.
3. Replace the controller binary.
4. Start the controller (migrations run automatically).
5. Upgrade nodes and gateways (order does not matter).

## Health Endpoints

All roles expose health endpoints for use with load balancers, Kubernetes probes, and monitoring systems.

### Controller

Available on the API listen address (default `:8443`):

| Endpoint | Purpose | Probe Type |
|----------|---------|------------|
| `/livez` | Process is running | Liveness probe |
| `/healthz` | All dependency checks pass | Readiness probe |
| `/readyz` | Same as `/healthz` | Readiness probe |
| `/metrics` | Prometheus metrics | Monitoring |

### Node and Gateway

Available on a separate health server (default `:9090`):

| Endpoint | Purpose | Probe Type |
|----------|---------|------------|
| `/livez` | Process is running | Liveness probe |
| `/healthz` | Service health checks | Readiness probe |
| `/readyz` | Same as `/healthz` | Readiness probe |
| `/metrics` | Prometheus metrics | Monitoring |

Node health checks verify that enabled services (BGP, DNS, CDN, route forwarding) are operational. Gateway health checks verify the route forwarding service.

### Health Check Behavior

- `/livez` always returns HTTP 200 `{"status":"ok"}`. Use for liveness probes.
- `/healthz` and `/readyz` return HTTP 200 when all checks pass, HTTP 503 when any check fails.
- Each individual health check has a 5-second timeout.

## Prometheus Metrics

### Controller Metrics (`:8443/metrics`)

| Metric | Type | Description |
|--------|------|-------------|
| `edgefabric_active_nodes` | Gauge | Total registered nodes |
| `edgefabric_active_gateways` | Gauge | Total registered gateways |
| `edgefabric_active_tenants` | Gauge | Total tenants |
| `edgefabric_http_requests_total` | Counter | HTTP requests by method, path, status |
| `edgefabric_http_request_duration_seconds` | Histogram | Request latency |
| `go_*` | Various | Go runtime metrics (GC, goroutines, memory) |

### Node/Gateway Metrics (`:9090/metrics`)

| Metric | Type | Description |
|--------|------|-------------|
| `edgefabric_route_connections_active` | Gauge | Open forwarder connections |
| `edgefabric_route_bytes_forwarded_total` | Counter | Total bytes forwarded |
| `edgefabric_route_listeners_active` | Gauge | Active listeners by role and protocol |
| `edgefabric_cdn_cache_hits_total` | Counter | CDN cache hits |
| `edgefabric_cdn_cache_misses_total` | Counter | CDN cache misses |
| `edgefabric_cdn_requests_total` | Counter | Total CDN requests |
| `edgefabric_dns_queries_total` | Counter | Total DNS queries |
| `edgefabric_dns_zones_active` | Gauge | Active DNS zones |

### Key Alerts

Consider alerting on:

- `edgefabric_active_nodes` drops unexpectedly (fleet size change)
- `edgefabric_http_requests_total` with status `5xx` rate exceeds threshold
- `edgefabric_route_connections_active` near connection limits
- Node/gateway health endpoint returning 503

## Configuration

### Environment Variables

All config values can be overridden with environment variables (twelve-factor style):

| Variable | Config Path |
|----------|------------|
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
| `EF_GATEWAY_CONTROLLER_ADDR` | `gateway.controller_addr` |
| `EF_GATEWAY_ENROLLMENT_TOKEN` | `gateway.enrollment_token` |

### Config Validation

The controller validates at startup:
- Encryption key must be valid base64 decoding to exactly 32 bytes (if set)
- Service modes must be one of the allowed values
- WireGuard IPs must be valid IP addresses
- Required fields (listen address, storage driver/DSN) must be present

## Troubleshooting

### Controller won't start

- Check `controller.storage.dsn` points to a writable path
- Check `controller.secrets.encryption_key` is valid base64 (32 bytes decoded)
- Check port availability for `controller.listen_addr`

### Node services not starting

- Verify `node.controller_addr` is reachable
- Check service mode configuration (e.g., `node.bgp.mode: gobgp`)
- Check the health endpoint at `:9090/healthz` for individual service status

### Audit trail

Query audit events via `GET /api/v1/audit-events` (requires authentication). Events include source IP, user ID, action, and timestamp for forensic analysis.
