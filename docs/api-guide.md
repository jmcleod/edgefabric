# EdgeFabric API Guide

Practical guide to the EdgeFabric REST API with curl examples. For the complete specification, see the [OpenAPI 3.0 spec](/api/v1/openapi.yaml).

## Setup

All examples use these variables — set them once:

```bash
BASE_URL="http://localhost:8443"        # Controller address
TOKEN=""                                # Set after login
TENANT_ID=""                            # Set after creating a tenant
```

## Response Format

**Single object:**
```json
{ "data": { "id": "...", "name": "..." } }
```

**List:**
```json
{ "data": [...], "total": 42, "offset": 0, "limit": 50 }
```

**Error:**
```json
{ "error": { "code": "not_found", "message": "node not found" } }
```

---

## Authentication

### Login

```bash
curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq
```

Response includes a JWT token:
```json
{ "data": { "token": "eyJhbG...", "totp_required": false } }
```

Save it:
```bash
TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@edgefabric.local","password":"<password>"}' | jq -r '.data.token')
```

All subsequent requests use the `Authorization: Bearer <token>` header.

### TOTP (Two-Factor)

```bash
# Enroll in TOTP (returns QR code / secret)
curl -s -X POST "$BASE_URL/api/v1/auth/totp/enroll" \
  -H "Authorization: Bearer $TOKEN" | jq

# Confirm enrollment with a code from your authenticator app
curl -s -X POST "$BASE_URL/api/v1/auth/totp/confirm" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"123456"}' | jq

# Verify TOTP on login (when totp_required=true)
curl -s -X POST "$BASE_URL/api/v1/auth/totp/verify" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"123456"}' | jq
```

### API Keys

For programmatic access (agent heartbeats, CI/CD):

```bash
# Create API key
curl -s -X POST "$BASE_URL/api/v1/api-keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"ci-deploy","expires_in_days":90}' | jq

# List API keys
curl -s "$BASE_URL/api/v1/api-keys" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete API key
curl -s -X DELETE "$BASE_URL/api/v1/api-keys/$KEY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Use an API key as a Bearer token:
```bash
curl -s "$BASE_URL/api/v1/nodes" \
  -H "Authorization: Bearer $API_KEY" | jq
```

---

## Tenants

Multi-tenant isolation boundary.

```bash
# Create tenant
curl -s -X POST "$BASE_URL/api/v1/tenants" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Corp","slug":"acme"}' | jq

# List tenants
curl -s "$BASE_URL/api/v1/tenants" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get tenant
curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Update tenant
curl -s -X PUT "$BASE_URL/api/v1/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Corporation"}' | jq

# Delete tenant
curl -s -X DELETE "$BASE_URL/api/v1/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Users

```bash
# Create user (scoped to tenant)
curl -s -X POST "$BASE_URL/api/v1/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "'$TENANT_ID'",
    "email": "ops@acme.example",
    "name": "Ops User",
    "password": "secure-password",
    "role": "admin"
  }' | jq

# Roles: "superuser" (no tenant), "admin" (tenant-scoped), "readonly"

# List users
curl -s "$BASE_URL/api/v1/users" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get user
curl -s "$BASE_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Update user
curl -s -X PUT "$BASE_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Senior Ops"}' | jq

# Delete user
curl -s -X DELETE "$BASE_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Fleet Management

### Nodes

```bash
# Create node
curl -s -X POST "$BASE_URL/api/v1/nodes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "edge-us-east-1",
    "hostname": "edge1.us-east.example.com",
    "public_ip": "203.0.113.10",
    "region": "us-east-1",
    "provider": "aws",
    "ssh_port": 22,
    "ssh_user": "root"
  }' | jq

# List nodes (with pagination)
curl -s "$BASE_URL/api/v1/nodes?limit=20&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get node
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Update node (assign to tenant)
curl -s -X PUT "$BASE_URL/api/v1/nodes/$NODE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "'$TENANT_ID'"}' | jq

# Send heartbeat
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/heartbeat" \
  -H "Authorization: Bearer $TOKEN"

# Delete node
curl -s -X DELETE "$BASE_URL/api/v1/nodes/$NODE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Gateways

```bash
# Create gateway
curl -s -X POST "$BASE_URL/api/v1/gateways" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "'$TENANT_ID'",
    "name": "gw-hq",
    "public_ip": "192.0.2.50"
  }' | jq

# List gateways
curl -s "$BASE_URL/api/v1/gateways" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get gateway
curl -s "$BASE_URL/api/v1/gateways/$GW_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Send heartbeat
curl -s -X POST "$BASE_URL/api/v1/gateways/$GW_ID/heartbeat" \
  -H "Authorization: Bearer $TOKEN"

# Get route config (polled by gateway agents)
curl -s "$BASE_URL/api/v1/gateways/$GW_ID/config/routes" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete gateway
curl -s -X DELETE "$BASE_URL/api/v1/gateways/$GW_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Node Groups

```bash
# Create group
curl -s -X POST "$BASE_URL/api/v1/node-groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "'$TENANT_ID'",
    "name": "global-edge",
    "description": "Edge nodes across all regions"
  }' | jq

# List groups
curl -s "$BASE_URL/api/v1/node-groups" \
  -H "Authorization: Bearer $TOKEN" | jq

# Add node to group
curl -s -X POST "$BASE_URL/api/v1/node-groups/$GROUP_ID/nodes/$NODE_ID" \
  -H "Authorization: Bearer $TOKEN"

# Remove node from group
curl -s -X DELETE "$BASE_URL/api/v1/node-groups/$GROUP_ID/nodes/$NODE_ID" \
  -H "Authorization: Bearer $TOKEN"

# Delete group
curl -s -X DELETE "$BASE_URL/api/v1/node-groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### SSH Keys

```bash
# Create SSH key (auto-generates key pair)
curl -s -X POST "$BASE_URL/api/v1/ssh-keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"deploy-key"}' | jq

# List SSH keys
curl -s "$BASE_URL/api/v1/ssh-keys" \
  -H "Authorization: Bearer $TOKEN" | jq

# Rotate key
curl -s -X POST "$BASE_URL/api/v1/ssh-keys/$KEY_ID/rotate" \
  -H "Authorization: Bearer $TOKEN" | jq

# Deploy key to a node
curl -s -X POST "$BASE_URL/api/v1/ssh-keys/$KEY_ID/deploy" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"node_id":"'$NODE_ID'"}' | jq

# Delete SSH key
curl -s -X DELETE "$BASE_URL/api/v1/ssh-keys/$KEY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Provisioning

Node lifecycle management: enroll, start, stop, restart, upgrade, decommission.

```bash
# Trigger enrollment
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/enroll" \
  -H "Authorization: Bearer $TOKEN" | jq

# Start node services
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/start" \
  -H "Authorization: Bearer $TOKEN" | jq

# Stop node services
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/stop" \
  -H "Authorization: Bearer $TOKEN" | jq

# Restart node services
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/restart" \
  -H "Authorization: Bearer $TOKEN" | jq

# Upgrade node binary
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/upgrade" \
  -H "Authorization: Bearer $TOKEN" | jq

# Reprovision node
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/reprovision" \
  -H "Authorization: Bearer $TOKEN" | jq

# Decommission node
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/decommission" \
  -H "Authorization: Bearer $TOKEN" | jq

# List provisioning jobs for a node
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/jobs" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get specific job
curl -s "$BASE_URL/api/v1/provisioning/jobs/$JOB_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

---

## DNS

### Zones

```bash
# Create zone
curl -s -X POST "$BASE_URL/api/v1/tenants/$TENANT_ID/dns/zones" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "example.com",
    "ttl": 300,
    "node_group_id": "'$GROUP_ID'"
  }' | jq

# List zones
curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID/dns/zones" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get zone
curl -s "$BASE_URL/api/v1/dns/zones/$ZONE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Update zone
curl -s -X PUT "$BASE_URL/api/v1/dns/zones/$ZONE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ttl": 600}' | jq

# Delete zone
curl -s -X DELETE "$BASE_URL/api/v1/dns/zones/$ZONE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Records

Supported types: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`, `SRV`, `CAA`, `PTR`

```bash
# Create A record
curl -s -X POST "$BASE_URL/api/v1/dns/zones/$ZONE_ID/records" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "www",
    "type": "A",
    "value": "203.0.113.10",
    "ttl": 300
  }' | jq

# Create MX record
curl -s -X POST "$BASE_URL/api/v1/dns/zones/$ZONE_ID/records" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "@",
    "type": "MX",
    "value": "mail.example.com",
    "priority": 10
  }' | jq

# Create SRV record
curl -s -X POST "$BASE_URL/api/v1/dns/zones/$ZONE_ID/records" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "_sip._tcp",
    "type": "SRV",
    "value": "sip.example.com",
    "priority": 10,
    "weight": 60,
    "port": 5060
  }' | jq

# List records for a zone
curl -s "$BASE_URL/api/v1/dns/zones/$ZONE_ID/records" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get record
curl -s "$BASE_URL/api/v1/dns/records/$RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete record
curl -s -X DELETE "$BASE_URL/api/v1/dns/records/$RECORD_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## CDN

### Sites

```bash
# Create CDN site
curl -s -X POST "$BASE_URL/api/v1/tenants/$TENANT_ID/cdn/sites" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "www-cdn",
    "domains": ["www.example.com", "cdn.example.com"],
    "tls_mode": "auto",
    "cache_enabled": true,
    "cache_ttl": 3600,
    "compression_enabled": true,
    "rate_limit_rps": 1000,
    "node_group_id": "'$GROUP_ID'"
  }' | jq

# TLS modes: "auto", "manual", "disabled"

# List CDN sites
curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID/cdn/sites" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get site
curl -s "$BASE_URL/api/v1/cdn/sites/$SITE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Purge cache
curl -s -X POST "$BASE_URL/api/v1/cdn/sites/$SITE_ID/purge" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete site
curl -s -X DELETE "$BASE_URL/api/v1/cdn/sites/$SITE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Origins

```bash
# Create origin
curl -s -X POST "$BASE_URL/api/v1/cdn/sites/$SITE_ID/origins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "origin.example.com:443",
    "scheme": "https",
    "weight": 100,
    "health_check_path": "/healthz",
    "health_check_interval": 30
  }' | jq

# Schemes: "http", "https"

# List origins for a site
curl -s "$BASE_URL/api/v1/cdn/sites/$SITE_ID/origins" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get origin
curl -s "$BASE_URL/api/v1/cdn/origins/$ORIGIN_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete origin
curl -s -X DELETE "$BASE_URL/api/v1/cdn/origins/$ORIGIN_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Routes

TCP/UDP traffic forwarding through gateways.

```bash
# Create route
curl -s -X POST "$BASE_URL/api/v1/tenants/$TENANT_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ssh-to-hq",
    "protocol": "tcp",
    "entry_ip": "0.0.0.0",
    "entry_port": 2222,
    "gateway_id": "'$GW_ID'",
    "destination_ip": "10.0.1.100",
    "destination_port": 22,
    "node_group_id": "'$GROUP_ID'"
  }' | jq

# Protocols: "tcp", "udp", "icmp", "all"

# List routes
curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID/routes" \
  -H "Authorization: Bearer $TOKEN" | jq

# Get route
curl -s "$BASE_URL/api/v1/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq

# Update route
curl -s -X PUT "$BASE_URL/api/v1/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"ssh-to-hq-v2","destination_port":2222}' | jq

# Delete route
curl -s -X DELETE "$BASE_URL/api/v1/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Networking

### BGP Sessions

```bash
# Create BGP session
curl -s -X POST "$BASE_URL/api/v1/nodes/$NODE_ID/bgp-sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "neighbor_ip": "10.0.0.1",
    "neighbor_asn": 64512,
    "local_asn": 65001
  }' | jq

# List BGP sessions for a node
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/bgp-sessions" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete BGP session
curl -s -X DELETE "$BASE_URL/api/v1/bgp-sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### IP Allocations

```bash
# Create IP allocation
curl -s -X POST "$BASE_URL/api/v1/tenants/$TENANT_ID/ip-allocations" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cidr": "10.10.0.0/24",
    "description": "Office network"
  }' | jq

# List IP allocations
curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID/ip-allocations" \
  -H "Authorization: Bearer $TOKEN" | jq

# Delete IP allocation
curl -s -X DELETE "$BASE_URL/api/v1/ip-allocations/$ALLOC_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### WireGuard Peers

```bash
# List WireGuard peers
curl -s "$BASE_URL/api/v1/wireguard/peers" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Node Networking State

```bash
# Get node networking state (WG, BGP, IPs combined)
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/networking" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Node Config Endpoints

Polled by node agents to get their desired configuration:

```bash
# WireGuard config
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/config/wireguard" \
  -H "Authorization: Bearer $TOKEN" | jq

# BGP config
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/config/bgp" \
  -H "Authorization: Bearer $TOKEN" | jq

# DNS config
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/config/dns" \
  -H "Authorization: Bearer $TOKEN" | jq

# CDN config
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/config/cdn" \
  -H "Authorization: Bearer $TOKEN" | jq

# Route config
curl -s "$BASE_URL/api/v1/nodes/$NODE_ID/config/routes" \
  -H "Authorization: Bearer $TOKEN" | jq
```

---

## Status & Observability

### Dashboard Status

```bash
curl -s "$BASE_URL/api/v1/status" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Returns node/gateway/route/DNS/CDN counts, status breakdowns, and stale config detection.

### Audit Log

```bash
# List recent audit events
curl -s "$BASE_URL/api/v1/audit-events?limit=20" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Health & Metrics (Unauthenticated)

```bash
# Health check (used by load balancers)
curl -s "$BASE_URL/healthz"

# Readiness check
curl -s "$BASE_URL/readyz"

# Liveness check
curl -s "$BASE_URL/livez"

# Prometheus metrics
curl -s "$BASE_URL/metrics"
```

### OpenAPI Spec

```bash
curl -s "$BASE_URL/api/v1/openapi.yaml"
```

---

## Enrollment (Agent ↔ Controller)

Token-based enrollment for new nodes (unauthenticated):

```bash
curl -s -X POST "$BASE_URL/api/v1/enroll" \
  -H "Content-Type: application/json" \
  -d '{"token":"ef-enroll-xxxxxxxxxxxx"}' | jq
```

This endpoint is used by node/gateway agents during initial provisioning. The token is generated by the controller when a provisioning job is triggered.

---

## Pagination

List endpoints accept `limit` and `offset` query parameters:

```bash
# First page (20 items)
curl -s "$BASE_URL/api/v1/nodes?limit=20&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq

# Second page
curl -s "$BASE_URL/api/v1/nodes?limit=20&offset=20" \
  -H "Authorization: Bearer $TOKEN" | jq
```

The response includes `total` for client-side pagination:
```json
{ "data": [...], "total": 42, "offset": 20, "limit": 20 }
```

---

## Error Codes

| HTTP Status | Code | Meaning |
|-------------|------|---------|
| 400 | `bad_request` | Invalid input or missing required field |
| 401 | `unauthorized` | Missing or invalid token |
| 403 | `forbidden` | Insufficient permissions (RBAC) |
| 404 | `not_found` | Resource does not exist |
| 409 | `already_exists` | Duplicate (e.g., tenant slug) |
| 500 | `internal_error` | Server error |

---

## Further Reading

- [OpenAPI Spec](../openapi/v1.yaml) — Full API specification
- [Security Model](security-model.md) — Authentication, RBAC, secret management
- [Tenancy & RBAC](tenancy-and-rbac.md) — Multi-tenant isolation and role model
- [Domain Model](DOMAIN_MODEL.md) — Entity relationships
