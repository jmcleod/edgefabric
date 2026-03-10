#!/bin/sh
# Gateway entrypoint for Docker demo.
#
# Waits for the demo-setup container to write the gateway state file
# (containing gateway_id and api_token) to the shared volume, then
# copies it to the data dir and starts the gateway agent.
set -e

STATE_FILE="/shared/gateway-state.json"
DATA_DIR="/var/lib/edgefabric"

printf '\033[1;34m▸ [gateway] Waiting for state file at %s...\033[0m\n' "$STATE_FILE"

attempts=0
while [ ! -f "$STATE_FILE" ]; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 60 ]; then
    printf '\033[1;31m✘ [gateway] Timed out waiting for state file\033[0m\n'
    printf '\033[1;33m  Starting without controller client (route polling disabled)\033[0m\n'
    exec edgefabric gateway --config /etc/edgefabric/edgefabric.yaml
  fi
  sleep 2
done

# Copy state file to the gateway's data directory.
mkdir -p "$DATA_DIR"
cp "$STATE_FILE" "$DATA_DIR/gateway-state.json"
printf '\033[1;32m✔ [gateway] State file loaded\033[0m\n'

exec edgefabric gateway --config /etc/edgefabric/edgefabric.yaml
