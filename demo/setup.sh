#!/bin/sh
# EdgeFabric Comprehensive Demo Setup Script
#
# Seeds the controller with multi-tenant sample data via the REST API.
# Creates 2 tenants, 3 nodes, 2 gateways, DNS zones, CDN sites, routes,
# BGP sessions, and generates enrollment tokens for the node agents.
#
# Requires: curl, jq (both available in the demo-setup container).
set -e

CONTROLLER_URL="${CONTROLLER_URL:-http://controller:8443}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { printf '\033[1;34m▸ %s\033[0m\n' "$1"; }
ok()    { printf '\033[1;32m✔ %s\033[0m\n' "$1"; }
fail()  { printf '\033[1;31m✘ %s\033[0m\n' "$1"; exit 1; }

api() {
  # api METHOD PATH [JSON_BODY]
  _method="$1"; _path="$2"; _body="${3:-}"
  if [ -n "$_body" ]; then
    curl -sf -X "$_method" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "$_body" \
      "${CONTROLLER_URL}${_path}"
  else
    curl -sf -X "$_method" \
      -H "Authorization: Bearer $TOKEN" \
      "${CONTROLLER_URL}${_path}"
  fi
}

# Extract a field from a JSON response's "data" object.
jq_data() { jq -r ".data.$1"; }

# ---------------------------------------------------------------------------
# 1. Wait for the controller to become healthy
# ---------------------------------------------------------------------------

info "Waiting for controller to become healthy..."
attempts=0
until curl -sf "${CONTROLLER_URL}/healthz" > /dev/null 2>&1; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 30 ]; then
    fail "Controller did not become healthy after 60 seconds"
  fi
  sleep 2
done
ok "Controller is healthy"

# ---------------------------------------------------------------------------
# 2. Extract admin password from startup log
# ---------------------------------------------------------------------------

info "Extracting admin password from startup log..."
attempts=0
ADMIN_PASSWORD=""
while [ -z "$ADMIN_PASSWORD" ]; do
  if [ -f /shared/startup.log ]; then
    ADMIN_PASSWORD=$(grep 'seed superuser created' /shared/startup.log | head -1 | jq -r '.password' 2>/dev/null || true)
  fi
  if [ -z "$ADMIN_PASSWORD" ]; then
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 15 ]; then
      fail "Could not extract admin password from startup log"
    fi
    sleep 2
  fi
done
ok "Admin password extracted"

# ---------------------------------------------------------------------------
# 3. Login and get token
# ---------------------------------------------------------------------------

info "Logging in as admin@edgefabric.local..."
LOGIN_RESP=$(curl -sf -X POST \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin@edgefabric.local\",\"password\":\"${ADMIN_PASSWORD}\"}" \
  "${CONTROLLER_URL}/api/v1/auth/login")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  fail "Login failed: $LOGIN_RESP"
fi
ok "Logged in (token acquired)"

# ===========================================================================
# TENANT 1 — Acme Corp
# ===========================================================================

info "Creating tenant: Acme Corp..."
ACME_RESP=$(api POST /api/v1/tenants '{"name":"Acme Corp","slug":"acme"}')
ACME_ID=$(echo "$ACME_RESP" | jq_data id)
ok "Tenant created: $ACME_ID"

info "Creating tenant user: ops@acme.example..."
ACME_USER_RESP=$(api POST /api/v1/users "{
  \"tenant_id\": \"$ACME_ID\",
  \"email\": \"ops@acme.example\",
  \"name\": \"Acme Ops\",
  \"password\": \"demo-password-123\",
  \"role\": \"admin\"
}")
ACME_USER_ID=$(echo "$ACME_USER_RESP" | jq_data id)
ok "User created: $ACME_USER_ID"

# ===========================================================================
# TENANT 2 — Globex Inc
# ===========================================================================

info "Creating tenant: Globex Inc..."
GLOBEX_RESP=$(api POST /api/v1/tenants '{"name":"Globex Inc","slug":"globex"}')
GLOBEX_ID=$(echo "$GLOBEX_RESP" | jq_data id)
ok "Tenant created: $GLOBEX_ID"

info "Creating tenant user: admin@globex.example..."
GLOBEX_USER_RESP=$(api POST /api/v1/users "{
  \"tenant_id\": \"$GLOBEX_ID\",
  \"email\": \"admin@globex.example\",
  \"name\": \"Globex Admin\",
  \"password\": \"demo-password-456\",
  \"role\": \"admin\"
}")
GLOBEX_USER_ID=$(echo "$GLOBEX_USER_RESP" | jq_data id)
ok "User created: $GLOBEX_USER_ID"

# ===========================================================================
# NODES
# ===========================================================================

info "Creating node: edge-us-east-1..."
NODE1_RESP=$(api POST /api/v1/nodes '{
  "name": "edge-us-east-1",
  "hostname": "edge1.us-east-1.edgefabric.local",
  "public_ip": "172.20.0.10",
  "region": "us-east-1",
  "provider": "docker"
}')
NODE1_ID=$(echo "$NODE1_RESP" | jq_data id)
ok "Node created: $NODE1_ID"

info "Creating node: edge-eu-west-1..."
NODE2_RESP=$(api POST /api/v1/nodes '{
  "name": "edge-eu-west-1",
  "hostname": "edge1.eu-west-1.edgefabric.local",
  "public_ip": "172.20.0.11",
  "region": "eu-west-1",
  "provider": "docker"
}')
NODE2_ID=$(echo "$NODE2_RESP" | jq_data id)
ok "Node created: $NODE2_ID"

info "Creating node: vps-staging (enrollment demo)..."
VPS_RESP=$(api POST /api/v1/nodes '{
  "name": "vps-staging",
  "hostname": "vps.us-west-2.edgefabric.local",
  "public_ip": "172.20.0.12",
  "region": "us-west-2",
  "provider": "docker"
}')
VPS_ID=$(echo "$VPS_RESP" | jq_data id)
ok "Node created: $VPS_ID"

# Assign nodes to tenants.
info "Assigning nodes to Acme Corp..."
api PUT "/api/v1/nodes/${NODE1_ID}" "{\"tenant_id\": \"$ACME_ID\"}" > /dev/null
api PUT "/api/v1/nodes/${NODE2_ID}" "{\"tenant_id\": \"$ACME_ID\"}" > /dev/null
ok "node-1 and node-2 assigned to Acme Corp"

info "Assigning VPS to Acme Corp..."
api PUT "/api/v1/nodes/${VPS_ID}" "{\"tenant_id\": \"$ACME_ID\"}" > /dev/null
ok "vps-staging assigned to Acme Corp"

# ===========================================================================
# NODE GROUPS
# ===========================================================================

info "Creating node group: acme-global-edge..."
ACME_GROUP_RESP=$(api POST /api/v1/node-groups "{
  \"tenant_id\": \"$ACME_ID\",
  \"name\": \"acme-global-edge\",
  \"description\": \"Acme Corp edge nodes across all regions\"
}")
ACME_GROUP_ID=$(echo "$ACME_GROUP_RESP" | jq_data id)
ok "Node group created: $ACME_GROUP_ID"

api POST "/api/v1/node-groups/${ACME_GROUP_ID}/nodes/${NODE1_ID}" > /dev/null
api POST "/api/v1/node-groups/${ACME_GROUP_ID}/nodes/${NODE2_ID}" > /dev/null
ok "Nodes added to acme-global-edge"

info "Creating node group: globex-edge..."
GLOBEX_GROUP_RESP=$(api POST /api/v1/node-groups "{
  \"tenant_id\": \"$GLOBEX_ID\",
  \"name\": \"globex-edge\",
  \"description\": \"Globex Inc shared edge infrastructure\"
}")
GLOBEX_GROUP_ID=$(echo "$GLOBEX_GROUP_RESP" | jq_data id)
ok "Node group created: $GLOBEX_GROUP_ID"

api POST "/api/v1/node-groups/${GLOBEX_GROUP_ID}/nodes/${NODE1_ID}" > /dev/null
api POST "/api/v1/node-groups/${GLOBEX_GROUP_ID}/nodes/${NODE2_ID}" > /dev/null
ok "Nodes added to globex-edge (shared infrastructure)"

# ===========================================================================
# GATEWAYS
# ===========================================================================

info "Creating gateway: gw-hq (Acme)..."
ACME_GW_RESP=$(api POST /api/v1/gateways "{
  \"tenant_id\": \"$ACME_ID\",
  \"name\": \"gw-hq\",
  \"public_ip\": \"172.20.0.50\"
}")
ACME_GW_ID=$(echo "$ACME_GW_RESP" | jq_data id)
ok "Gateway created: $ACME_GW_ID"

# Write gateway state file so the gateway container can poll route config.
info "Writing gateway state file..."
printf '{\n  "gateway_id": "%s",\n  "api_token": "%s"\n}\n' "$ACME_GW_ID" "$TOKEN" > /shared/gateway-state.json
ok "Gateway state written to /shared/gateway-state.json"

info "Creating gateway: gw-branch (Globex)..."
GLOBEX_GW_RESP=$(api POST /api/v1/gateways "{
  \"tenant_id\": \"$GLOBEX_ID\",
  \"name\": \"gw-branch\",
  \"public_ip\": \"192.0.2.60\"
}")
GLOBEX_GW_ID=$(echo "$GLOBEX_GW_RESP" | jq_data id)
ok "Gateway created: $GLOBEX_GW_ID"

# ===========================================================================
# ENROLLMENT TOKENS
# ===========================================================================

info "Generating enrollment token for node-1..."
TOKEN1_RESP=$(api POST "/api/v1/nodes/${NODE1_ID}/enrollment-token")
TOKEN1=$(echo "$TOKEN1_RESP" | jq -r '.data.token')
echo "$TOKEN1" > /shared/node-1.token
ok "Token written to /shared/node-1.token"

info "Generating enrollment token for node-2..."
TOKEN2_RESP=$(api POST "/api/v1/nodes/${NODE2_ID}/enrollment-token")
TOKEN2=$(echo "$TOKEN2_RESP" | jq -r '.data.token')
echo "$TOKEN2" > /shared/node-2.token
ok "Token written to /shared/node-2.token"

info "Generating enrollment token for VPS..."
VPS_TOKEN_RESP=$(api POST "/api/v1/nodes/${VPS_ID}/enrollment-token")
VPS_TOKEN=$(echo "$VPS_TOKEN_RESP" | jq -r '.data.token')
echo "$VPS_TOKEN" > /shared/vps.token
ok "Token written to /shared/vps.token"

# ===========================================================================
# DNS — Acme Corp (acme.example)
# ===========================================================================

info "Creating DNS zone: acme.example..."
ACME_ZONE_RESP=$(api POST "/api/v1/tenants/${ACME_ID}/dns/zones" "{
  \"name\": \"acme.example\",
  \"ttl\": 300,
  \"node_group_id\": \"$ACME_GROUP_ID\"
}")
ACME_ZONE_ID=$(echo "$ACME_ZONE_RESP" | jq_data id)
ok "DNS zone created: $ACME_ZONE_ID"

info "Creating DNS records for acme.example..."
api POST "/api/v1/dns/zones/${ACME_ZONE_ID}/records" '{
  "name": "www", "type": "A", "value": "172.20.0.100", "ttl": 300
}' > /dev/null
ok "  A   www.acme.example -> 172.20.0.100"

api POST "/api/v1/dns/zones/${ACME_ZONE_ID}/records" '{
  "name": "@", "type": "A", "value": "172.20.0.10", "ttl": 300
}' > /dev/null
ok "  A   acme.example -> 172.20.0.10"

api POST "/api/v1/dns/zones/${ACME_ZONE_ID}/records" '{
  "name": "cdn", "type": "CNAME", "value": "www.acme.example", "ttl": 300
}' > /dev/null
ok "  CNAME cdn.acme.example -> www.acme.example"

api POST "/api/v1/dns/zones/${ACME_ZONE_ID}/records" '{
  "name": "@", "type": "MX", "value": "mail.acme.example", "priority": 10
}' > /dev/null
ok "  MX  acme.example -> mail.acme.example (pri 10)"

api POST "/api/v1/dns/zones/${ACME_ZONE_ID}/records" '{
  "name": "@", "type": "TXT", "value": "v=spf1 include:_spf.acme.example ~all"
}' > /dev/null
ok "  TXT SPF policy"

# ===========================================================================
# DNS — Globex Inc (globex.example)
# ===========================================================================

info "Creating DNS zone: globex.example..."
GLOBEX_ZONE_RESP=$(api POST "/api/v1/tenants/${GLOBEX_ID}/dns/zones" "{
  \"name\": \"globex.example\",
  \"ttl\": 600,
  \"node_group_id\": \"$GLOBEX_GROUP_ID\"
}")
GLOBEX_ZONE_ID=$(echo "$GLOBEX_ZONE_RESP" | jq_data id)
ok "DNS zone created: $GLOBEX_ZONE_ID"

info "Creating DNS records for globex.example..."
api POST "/api/v1/dns/zones/${GLOBEX_ZONE_ID}/records" '{
  "name": "www", "type": "A", "value": "172.20.0.100", "ttl": 600
}' > /dev/null
ok "  A   www.globex.example -> 172.20.0.100"

api POST "/api/v1/dns/zones/${GLOBEX_ZONE_ID}/records" '{
  "name": "api", "type": "A", "value": "172.20.0.100", "ttl": 600
}' > /dev/null
ok "  A   api.globex.example -> 172.20.0.100"

api POST "/api/v1/dns/zones/${GLOBEX_ZONE_ID}/records" '{
  "name": "@", "type": "MX", "value": "mail.globex.example", "priority": 10
}' > /dev/null
ok "  MX  globex.example -> mail.globex.example (pri 10)"

api POST "/api/v1/dns/zones/${GLOBEX_ZONE_ID}/records" '{
  "name": "@", "type": "TXT", "value": "v=spf1 include:_spf.globex.example ~all"
}' > /dev/null
ok "  TXT SPF policy"

# ===========================================================================
# CDN — Acme Corp (www-cdn)
# ===========================================================================

info "Creating CDN site: www-cdn (Acme)..."
ACME_CDN_RESP=$(api POST "/api/v1/tenants/${ACME_ID}/cdn/sites" "{
  \"name\": \"www-cdn\",
  \"domains\": [\"www.acme.example\", \"cdn.acme.example\"],
  \"tls_mode\": \"auto\",
  \"cache_enabled\": true,
  \"cache_ttl\": 3600,
  \"compression_enabled\": true,
  \"node_group_id\": \"$ACME_GROUP_ID\"
}")
ACME_CDN_ID=$(echo "$ACME_CDN_RESP" | jq_data id)
ok "CDN site created: $ACME_CDN_ID"

info "Adding CDN origin for Acme (origin container)..."
api POST "/api/v1/cdn/sites/${ACME_CDN_ID}/origins" '{
  "address": "172.20.0.100:80",
  "scheme": "http",
  "weight": 100,
  "health_check_path": "/healthz",
  "health_check_interval": 30
}' > /dev/null
ok "CDN origin added: 172.20.0.100:80 (origin container)"

# ===========================================================================
# CDN — Globex Inc (api-cdn)
# ===========================================================================

info "Creating CDN site: api-cdn (Globex)..."
GLOBEX_CDN_RESP=$(api POST "/api/v1/tenants/${GLOBEX_ID}/cdn/sites" "{
  \"name\": \"api-cdn\",
  \"domains\": [\"api.globex.example\"],
  \"tls_mode\": \"auto\",
  \"cache_enabled\": false,
  \"compression_enabled\": true,
  \"node_group_id\": \"$GLOBEX_GROUP_ID\"
}")
GLOBEX_CDN_ID=$(echo "$GLOBEX_CDN_RESP" | jq_data id)
ok "CDN site created: $GLOBEX_CDN_ID"

info "Adding CDN origin for Globex (origin container)..."
api POST "/api/v1/cdn/sites/${GLOBEX_CDN_ID}/origins" '{
  "address": "172.20.0.100:80",
  "scheme": "http",
  "weight": 100,
  "health_check_path": "/healthz",
  "health_check_interval": 30
}' > /dev/null
ok "CDN origin added: 172.20.0.100:80 (origin container)"

# ===========================================================================
# ROUTES
# ===========================================================================

# Route: http-to-origin — demonstrates traffic flowing:
#   Client → Node (port 9000) → Gateway (172.20.0.50:9000) → Origin (172.20.0.100:80)
# Exposed on host as: node-1 → localhost:9001, node-2 → localhost:9002
info "Creating route: http-to-origin (Acme)..."
ACME_HTTP_ROUTE_RESP=$(api POST "/api/v1/tenants/${ACME_ID}/routes" "{
  \"name\": \"http-to-origin\",
  \"protocol\": \"tcp\",
  \"entry_ip\": \"0.0.0.0\",
  \"entry_port\": 9000,
  \"gateway_id\": \"$ACME_GW_ID\",
  \"destination_ip\": \"172.20.0.100\",
  \"destination_port\": 80,
  \"node_group_id\": \"$ACME_GROUP_ID\"
}")
ACME_HTTP_ROUTE_ID=$(echo "$ACME_HTTP_ROUTE_RESP" | jq_data id)
ok "Route created: $ACME_HTTP_ROUTE_ID (node:9000 → gw → origin:80)"

# Route: ssh-to-vps — demonstrates SSH traffic via gateway:
#   Client → Node (port 2222) → Gateway (172.20.0.50:2222) → VPS (172.20.0.12:22)
info "Creating route: ssh-to-vps (Acme)..."
ACME_SSH_ROUTE_RESP=$(api POST "/api/v1/tenants/${ACME_ID}/routes" "{
  \"name\": \"ssh-to-vps\",
  \"protocol\": \"tcp\",
  \"entry_ip\": \"0.0.0.0\",
  \"entry_port\": 2222,
  \"gateway_id\": \"$ACME_GW_ID\",
  \"destination_ip\": \"172.20.0.12\",
  \"destination_port\": 22,
  \"node_group_id\": \"$ACME_GROUP_ID\"
}")
ACME_SSH_ROUTE_ID=$(echo "$ACME_SSH_ROUTE_RESP" | jq_data id)
ok "Route created: $ACME_SSH_ROUTE_ID (node:2222 → gw → vps:22)"

# Route: db-access (Globex) — uses the non-demo gateway, kept for SPA display.
info "Creating route: db-access (Globex)..."
GLOBEX_ROUTE_RESP=$(api POST "/api/v1/tenants/${GLOBEX_ID}/routes" "{
  \"name\": \"db-access\",
  \"protocol\": \"tcp\",
  \"entry_ip\": \"0.0.0.0\",
  \"entry_port\": 5432,
  \"gateway_id\": \"$GLOBEX_GW_ID\",
  \"destination_ip\": \"10.0.2.50\",
  \"destination_port\": 5432,
  \"node_group_id\": \"$GLOBEX_GROUP_ID\"
}")
GLOBEX_ROUTE_ID=$(echo "$GLOBEX_ROUTE_RESP" | jq_data id)
ok "Route created: $GLOBEX_ROUTE_ID"

# ===========================================================================
# BGP SESSIONS
# ===========================================================================

info "Creating BGP session for node-1..."
api POST "/api/v1/nodes/${NODE1_ID}/bgp-sessions" '{
  "peer_asn": 64512,
  "peer_address": "172.16.0.1",
  "local_asn": 65001,
  "announced_prefixes": ["203.0.113.0/24"],
  "import_policy": "accept-all",
  "export_policy": "announce-allocated"
}' > /dev/null
ok "BGP session: node-1 <-> AS64512 (172.16.0.1)"

info "Creating BGP session for node-2..."
api POST "/api/v1/nodes/${NODE2_ID}/bgp-sessions" '{
  "peer_asn": 64513,
  "peer_address": "172.16.0.2",
  "local_asn": 65001,
  "announced_prefixes": ["198.51.100.0/24"],
  "import_policy": "accept-all",
  "export_policy": "announce-allocated"
}' > /dev/null
ok "BGP session: node-2 <-> AS64513 (172.16.0.2)"

# ===========================================================================
# IP ALLOCATIONS
# ===========================================================================

info "Creating IP allocations..."
api POST "/api/v1/tenants/${ACME_ID}/ip-allocations" '{
  "prefix": "203.0.113.0/24",
  "type": "ipv4",
  "purpose": "anycast"
}' > /dev/null
ok "  Acme: 203.0.113.0/24 (anycast)"

api POST "/api/v1/tenants/${GLOBEX_ID}/ip-allocations" '{
  "prefix": "198.51.100.0/24",
  "type": "ipv4",
  "purpose": "anycast"
}' > /dev/null
ok "  Globex: 198.51.100.0/24 (anycast)"

# ===========================================================================
# VERIFY
# ===========================================================================

info "Fetching status overview..."
STATUS=$(api GET /api/v1/status)

# ===========================================================================
# SUMMARY
# ===========================================================================

printf '\n'
printf '\033[1;36m╔══════════════════════════════════════════════════════════════════╗\033[0m\n'
printf '\033[1;36m║         EdgeFabric Comprehensive Demo — Ready!                  ║\033[0m\n'
printf '\033[1;36m╚══════════════════════════════════════════════════════════════════╝\033[0m\n'
printf '\n'
printf '\033[1m── Credentials ─────────────────────────────────────────────────────\033[0m\n'
printf '  \033[1mSuperuser:\033[0m   admin@edgefabric.local / %s\n' "$ADMIN_PASSWORD"
printf '  \033[1mAcme user:\033[0m   ops@acme.example / demo-password-123\n'
printf '  \033[1mGlobex user:\033[0m admin@globex.example / demo-password-456\n'
printf '  \033[1mVPS SSH:\033[0m     ssh root@localhost -p 2222 (password: demo)\n'
printf '\n'
printf '\033[1m── Services ────────────────────────────────────────────────────────\033[0m\n'
printf '  Controller + SPA:  http://localhost:8443\n'
printf '  CDN (node-1):      curl -H "Host: www.acme.example" http://localhost:8081\n'
printf '  CDN (node-2):      curl -H "Host: www.acme.example" http://localhost:8082\n'
printf '  DNS (node-1):      dig @localhost -p 5354 www.acme.example A\n'
printf '  DNS (node-2):      dig @localhost -p 5355 www.acme.example A\n'
printf '  Route (node-1→gw): curl http://localhost:9001  (via gateway)\n'
printf '  Route (node-2→gw): curl http://localhost:9002  (via gateway)\n'
printf '  Origin (direct):   http://localhost:8888\n'
printf '  Gateway health:    http://localhost:9190/healthz\n'
printf '\n'
printf '\033[1m── Resources Created ───────────────────────────────────────────────\033[0m\n'
printf '  Tenants:    Acme Corp, Globex Inc\n'
printf '  Nodes:      edge-us-east-1, edge-eu-west-1, vps-staging\n'
printf '  Groups:     acme-global-edge (2 nodes), globex-edge (2 nodes)\n'
printf '  Gateways:   gw-hq (Acme), gw-branch (Globex)\n'
printf '  DNS Zones:  acme.example (5 records), globex.example (4 records)\n'
printf '  CDN Sites:  www-cdn (Acme, cached), api-cdn (Globex, pass-through)\n'
printf '  Routes:     http-to-origin (tcp/9000), ssh-to-vps (tcp/2222), db-access (tcp/5432)\n'
printf '  BGP:        node-1 <-> AS64512, node-2 <-> AS64513\n'
printf '  IPs:        203.0.113.0/24 (Acme), 198.51.100.0/24 (Globex)\n'
printf '\n'
printf '\033[1m── Status ──────────────────────────────────────────────────────────\033[0m\n'
printf '  Nodes:    %s\n' "$(echo "$STATUS" | jq -r '.data.node_count')"
printf '  Gateways: %s\n' "$(echo "$STATUS" | jq -r '.data.gateway_count')"
printf '  Routes:   %s\n' "$(echo "$STATUS" | jq -r '.data.route_count')"
printf '  DNS:      %s zones\n' "$(echo "$STATUS" | jq -r '.data.dns_zone_count')"
printf '  CDN:      %s sites\n' "$(echo "$STATUS" | jq -r '.data.cdn_site_count')"
printf '\n'
printf '\033[1m── Demo Commands ───────────────────────────────────────────────────\033[0m\n'
printf '  # CDN caching (first request = MISS, second = HIT)\n'
printf '  curl -v -H "Host: www.acme.example" http://localhost:8081\n'
printf '\n'
printf '  # DNS lookup\n'
printf '  dig @localhost -p 5354 www.acme.example A\n'
printf '\n'
printf '  # Route via gateway (node → gateway → origin, no WireGuard needed)\n'
printf '  curl http://localhost:9001    # node-1 → gateway → origin\n'
printf '  curl http://localhost:9002    # node-2 → gateway → origin\n'
printf '\n'
printf '  # VPS enrollment\n'
printf '  ssh root@localhost -p 2222\n'
printf '  cat /shared/vps.token\n'
printf '  edgefabric node enroll --controller http://172.20.0.2:8443 --token $(cat /shared/vps.token)\n'
printf '\n'
printf '  # Web console\n'
printf '  open http://localhost:8443\n'
printf '\n'
