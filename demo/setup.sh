#!/bin/sh
# EdgeFabric Demo Setup Script
#
# Seeds the controller with sample data via the REST API.
# Requires: curl, jq (both available in the demo-setup container).
#
# This script is also useful as a reference for API usage patterns.
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

# ---------------------------------------------------------------------------
# 4. Create tenant
# ---------------------------------------------------------------------------

info "Creating tenant: Acme Corp..."
TENANT_RESP=$(api POST /api/v1/tenants '{"name":"Acme Corp","slug":"acme"}')
TENANT_ID=$(echo "$TENANT_RESP" | jq_data id)
ok "Tenant created: $TENANT_ID"

# ---------------------------------------------------------------------------
# 5. Create tenant user
# ---------------------------------------------------------------------------

info "Creating tenant user: ops@acme.example..."
USER_RESP=$(api POST /api/v1/users "{
  \"tenant_id\": \"$TENANT_ID\",
  \"email\": \"ops@acme.example\",
  \"name\": \"Ops User\",
  \"password\": \"demo-password-123\",
  \"role\": \"admin\"
}")
USER_ID=$(echo "$USER_RESP" | jq_data id)
ok "User created: $USER_ID"

# ---------------------------------------------------------------------------
# 6. Create nodes
# ---------------------------------------------------------------------------

info "Creating node: edge-us-east-1..."
NODE1_RESP=$(api POST /api/v1/nodes '{
  "name": "edge-us-east-1",
  "hostname": "edge1.us-east-1.acme.example",
  "public_ip": "203.0.113.10",
  "region": "us-east-1",
  "provider": "aws"
}')
NODE1_ID=$(echo "$NODE1_RESP" | jq_data id)
ok "Node created: $NODE1_ID"

info "Creating node: edge-eu-west-1..."
NODE2_RESP=$(api POST /api/v1/nodes '{
  "name": "edge-eu-west-1",
  "hostname": "edge1.eu-west-1.acme.example",
  "public_ip": "198.51.100.20",
  "region": "eu-west-1",
  "provider": "aws"
}')
NODE2_ID=$(echo "$NODE2_RESP" | jq_data id)
ok "Node created: $NODE2_ID"

# Assign nodes to tenant.
info "Assigning nodes to tenant..."
api PUT "/api/v1/nodes/${NODE1_ID}" "{\"tenant_id\": \"$TENANT_ID\"}" > /dev/null
api PUT "/api/v1/nodes/${NODE2_ID}" "{\"tenant_id\": \"$TENANT_ID\"}" > /dev/null
ok "Nodes assigned to Acme Corp"

# ---------------------------------------------------------------------------
# 7. Create node group and add nodes
# ---------------------------------------------------------------------------

info "Creating node group: global-edge..."
GROUP_RESP=$(api POST /api/v1/node-groups "{
  \"tenant_id\": \"$TENANT_ID\",
  \"name\": \"global-edge\",
  \"description\": \"Edge nodes across all regions\"
}")
GROUP_ID=$(echo "$GROUP_RESP" | jq_data id)
ok "Node group created: $GROUP_ID"

info "Adding nodes to group..."
api POST "/api/v1/node-groups/${GROUP_ID}/nodes/${NODE1_ID}" > /dev/null
api POST "/api/v1/node-groups/${GROUP_ID}/nodes/${NODE2_ID}" > /dev/null
ok "Nodes added to global-edge group"

# ---------------------------------------------------------------------------
# 8. Create gateway
# ---------------------------------------------------------------------------

info "Creating gateway: gw-hq..."
GW_RESP=$(api POST /api/v1/gateways "{
  \"tenant_id\": \"$TENANT_ID\",
  \"name\": \"gw-hq\",
  \"public_ip\": \"192.0.2.50\"
}")
GW_ID=$(echo "$GW_RESP" | jq_data id)
ok "Gateway created: $GW_ID"

# ---------------------------------------------------------------------------
# 9. Create DNS zone + records
# ---------------------------------------------------------------------------

info "Creating DNS zone: example.com..."
ZONE_RESP=$(api POST "/api/v1/tenants/${TENANT_ID}/dns/zones" "{
  \"name\": \"example.com\",
  \"ttl\": 300,
  \"node_group_id\": \"$GROUP_ID\"
}")
ZONE_ID=$(echo "$ZONE_RESP" | jq_data id)
ok "DNS zone created: $ZONE_ID"

info "Creating DNS records..."
api POST "/api/v1/dns/zones/${ZONE_ID}/records" '{
  "name": "www",
  "type": "A",
  "value": "203.0.113.10",
  "ttl": 300
}' > /dev/null
ok "  A record: www.example.com → 203.0.113.10"

api POST "/api/v1/dns/zones/${ZONE_ID}/records" '{
  "name": "@",
  "type": "MX",
  "value": "mail.example.com",
  "priority": 10
}' > /dev/null
ok "  MX record: example.com → mail.example.com (pri 10)"

api POST "/api/v1/dns/zones/${ZONE_ID}/records" '{
  "name": "@",
  "type": "TXT",
  "value": "v=spf1 include:_spf.example.com ~all"
}' > /dev/null
ok "  TXT record: SPF policy"

# ---------------------------------------------------------------------------
# 10. Create CDN site + origin
# ---------------------------------------------------------------------------

info "Creating CDN site: www-cdn..."
CDN_RESP=$(api POST "/api/v1/tenants/${TENANT_ID}/cdn/sites" "{
  \"name\": \"www-cdn\",
  \"domains\": [\"www.acme.example\", \"cdn.acme.example\"],
  \"tls_mode\": \"auto\",
  \"cache_enabled\": true,
  \"cache_ttl\": 3600,
  \"compression_enabled\": true,
  \"node_group_id\": \"$GROUP_ID\"
}")
CDN_ID=$(echo "$CDN_RESP" | jq_data id)
ok "CDN site created: $CDN_ID"

info "Adding CDN origin..."
api POST "/api/v1/cdn/sites/${CDN_ID}/origins" '{
  "address": "origin.acme.example:443",
  "scheme": "https",
  "weight": 100,
  "health_check_path": "/healthz",
  "health_check_interval": 30
}' > /dev/null
ok "CDN origin added: origin.acme.example:443"

# ---------------------------------------------------------------------------
# 11. Create route
# ---------------------------------------------------------------------------

info "Creating route: ssh-to-hq..."
ROUTE_RESP=$(api POST "/api/v1/tenants/${TENANT_ID}/routes" "{
  \"name\": \"ssh-to-hq\",
  \"protocol\": \"tcp\",
  \"entry_ip\": \"0.0.0.0\",
  \"entry_port\": 2222,
  \"gateway_id\": \"$GW_ID\",
  \"destination_ip\": \"10.0.1.100\",
  \"destination_port\": 22,
  \"node_group_id\": \"$GROUP_ID\"
}")
ROUTE_ID=$(echo "$ROUTE_RESP" | jq_data id)
ok "Route created: $ROUTE_ID"

# ---------------------------------------------------------------------------
# 12. Fetch status to verify
# ---------------------------------------------------------------------------

info "Fetching status overview..."
STATUS=$(api GET /api/v1/status)

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n'
printf '\033[1;36m╔══════════════════════════════════════════════════════════╗\033[0m\n'
printf '\033[1;36m║            EdgeFabric Demo Environment Ready            ║\033[0m\n'
printf '\033[1;36m╚══════════════════════════════════════════════════════════╝\033[0m\n'
printf '\n'
printf '\033[1mController:\033[0m  %s\n' "$CONTROLLER_URL"
printf '\033[1mAdmin email:\033[0m admin@edgefabric.local\n'
printf '\033[1mAdmin pass:\033[0m  %s\n' "$ADMIN_PASSWORD"
printf '\n'
printf '\033[1mResources created:\033[0m\n'
printf '  Tenant:     Acme Corp (%s)\n' "$TENANT_ID"
printf '  User:       ops@acme.example (%s)\n' "$USER_ID"
printf '  Nodes:      edge-us-east-1, edge-eu-west-1\n'
printf '  Group:      global-edge (2 nodes)\n'
printf '  Gateway:    gw-hq (%s)\n' "$GW_ID"
printf '  DNS Zone:   example.com (3 records)\n'
printf '  CDN Site:   www-cdn (1 origin)\n'
printf '  Route:      ssh-to-hq (tcp/2222 → 10.0.1.100:22)\n'
printf '\n'
printf '\033[1mStatus:\033[0m\n'
printf '  Nodes:    %s\n' "$(echo "$STATUS" | jq -r '.data.node_count')"
printf '  Gateways: %s\n' "$(echo "$STATUS" | jq -r '.data.gateway_count')"
printf '  Routes:   %s\n' "$(echo "$STATUS" | jq -r '.data.route_count')"
printf '  DNS:      %s zones\n' "$(echo "$STATUS" | jq -r '.data.dns_zone_count')"
printf '  CDN:      %s sites\n' "$(echo "$STATUS" | jq -r '.data.cdn_site_count')"
printf '\n'
printf '\033[1mTry it:\033[0m\n'
printf '  # Health check\n'
printf '  curl http://localhost:8443/healthz\n'
printf '\n'
printf '  # Login\n'
printf '  curl -s -X POST http://localhost:8443/api/v1/auth/login \\\n'
printf '    -H "Content-Type: application/json" \\\n'
printf '    -d '\''{"email":"admin@edgefabric.local","password":"%s"}'\'' | jq\n' "$ADMIN_PASSWORD"
printf '\n'
printf '  # List nodes (use token from login response)\n'
printf '  curl -s http://localhost:8443/api/v1/nodes \\\n'
printf '    -H "Authorization: Bearer <token>" | jq\n'
printf '\n'
