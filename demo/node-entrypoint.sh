#!/bin/sh
# Node entrypoint for Docker demo.
#
# Waits for the demo-setup container to write the enrollment token file
# to the shared volume, then starts the node agent with that token.
set -e

TOKEN_FILE="/shared/${NODE_NAME}.token"

printf '\033[1;34m▸ [%s] Waiting for enrollment token at %s...\033[0m\n' "$NODE_NAME" "$TOKEN_FILE"

attempts=0
while [ ! -f "$TOKEN_FILE" ]; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 60 ]; then
    printf '\033[1;31m✘ [%s] Timed out waiting for enrollment token\033[0m\n' "$NODE_NAME"
    exit 1
  fi
  sleep 2
done

export EF_NODE_ENROLLMENT_TOKEN=$(cat "$TOKEN_FILE")
printf '\033[1;32m✔ [%s] Enrollment token loaded\033[0m\n' "$NODE_NAME"

exec edgefabric node --config /etc/edgefabric/edgefabric.yaml
