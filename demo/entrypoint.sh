#!/bin/sh
# Demo entrypoint: run the controller and tee stdout/stderr to a shared volume
# so the setup container can extract the seeded admin password.
exec edgefabric controller --config /etc/edgefabric/edgefabric.yaml 2>&1 | tee /shared/startup.log
