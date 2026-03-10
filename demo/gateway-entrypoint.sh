#!/bin/sh
# Gateway entrypoint for Docker demo.
#
# Starts the gateway agent directly — no enrollment required in demo mode
# since the gateway reconciliation loop is a TODO stub.
set -e

printf '\033[1;34m▸ Starting gateway agent...\033[0m\n'
exec edgefabric gateway --config /etc/edgefabric/edgefabric.yaml
