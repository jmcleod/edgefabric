# Provisioning Model

## Overview

EdgeFabric provisions Nodes and Gateways via SSH from the Controller. The goal is zero-touch deployment: the operator provides a bare Linux VPS with SSH access, and the Controller handles binary deployment, configuration, WireGuard setup, and service startup.

---

## Enrollment Flow

### Prerequisites

1. A Linux VPS with SSH access (key-based authentication).
2. The SSH key registered in EdgeFabric (`SSHKey` entity, private key encrypted at rest).
3. A Node or Gateway record created in the Controller with `status: pending`.

### Sequence

```
1. Operator creates Node record in Controller (via API or CLI)
   → Node.Status = "pending"
   → Node has: PublicIP, SSHPort, SSHUser, SSHKeyID

2. Operator (or automation) triggers enrollment
   → Controller generates an EnrollmentToken (signed, time-limited)
   → Controller begins SSH session to the target host

3. Controller provisions over SSH:
   a. Upload edgefabric binary to /usr/local/bin/edgefabric
   b. Generate and push config YAML to /etc/edgefabric/config.yaml
   c. Generate WireGuard keys (on controller), push WireGuard config
   d. Install systemd unit file
   e. Start the edgefabric service
   → Node.Status = "enrolling"

4. Node starts and connects to Controller over WireGuard
   → Node presents its enrollment token
   → Controller validates token, marks node as enrolled
   → Node.Status = "online"

5. Node begins heartbeat loop
   → Periodic heartbeats over WireGuard update Node.LastHeartbeat
   → If heartbeat stops → Controller marks Node.Status = "offline"
```

### Enrollment Tokens

```
EnrollmentToken {
    TenantID     → scoping
    TargetType   → "node" or "gateway"
    TargetID     → which node/gateway this token is for
    Token        → HMAC-signed string (same signing infrastructure as session tokens)
    ExpiresAt    → short-lived (default: 1 hour)
    UsedAt       → set on first use; token is single-use
}
```

- Tokens are single-use and time-limited to minimize the window of compromise.
- The token ties to a specific Node/Gateway ID, preventing token reuse across hosts.
- Tokens are signed server-side (HMAC-SHA256) — no need for a separate CA.

---

## Configuration Generation

The Controller generates per-node configuration at enrollment time:

```yaml
role: node

node:
  controller_addr: "10.100.0.1:8443"  # Controller's WireGuard overlay IP
  enrollment_token: "<signed-token>"
  data_dir: "/var/lib/edgefabric"
```

Additional WireGuard configuration is pushed separately:

```ini
[Interface]
PrivateKey = <generated-by-controller>
Address = 10.100.1.X/16

[Peer]
PublicKey = <controller-public-key>
Endpoint = <controller-public-ip>:51820
AllowedIPs = 10.100.0.0/16
PersistentKeepalive = 25
```

### Security Assumption

The Controller generates all WireGuard private keys and pushes them over the SSH session. This means the Controller must be trusted with all overlay keys. The rationale: centralized key management simplifies rotation and revocation. Keys are encrypted at rest on both Controller (AES-256-GCM in the database) and Node (filesystem permissions).

---

## Binary Management

### Upload

The Controller uploads the `edgefabric` binary via SCP/SFTP during enrollment. The binary is the same single static binary used by the Controller, invoked with the `node` or `gateway` subcommand.

### Version Tracking

Each Node/Gateway record tracks `BinaryVersion`. After upload, the Controller records the deployed version. This enables:

- Version inventory across the fleet.
- Rolling update detection (which nodes need updating).
- Rollback tracking.

### Rolling Updates

```
1. Operator triggers fleet update (via API or CLI)
2. Controller iterates nodes (respecting concurrency limits):
   a. SSH to node
   b. Upload new binary to /usr/local/bin/edgefabric.new
   c. Stop service
   d. Replace binary (atomic rename)
   e. Start service
   f. Verify heartbeat resumes
   g. Update Node.BinaryVersion
3. If verification fails → rollback (rename old binary back, restart)
```

- Updates proceed one node at a time (configurable parallelism).
- A failed update stops the rollout and alerts the operator.
- The operator can set a "drain" period before update to allow traffic to shift.

---

## SSH Security Model

### Key Management

- SSH keys are stored in the `SSHKey` entity. Private keys are encrypted at rest (AES-256-GCM).
- Each Node record references an SSH key by ID.
- Multiple SSH keys can exist (e.g., per-provider, per-region).

### Assumptions

- The Controller must have network access to each Node's public IP on the configured SSH port.
- SSH is used only for provisioning and updates — ongoing management uses the WireGuard overlay.
- After initial enrollment, SSH is a fallback recovery mechanism, not the primary communication channel.
- SSH host key verification is not enforced in v1 (TOFU model). Future versions may store and verify host keys.

---

## Gateway Provisioning

Gateway provisioning follows the same flow as Node provisioning with these differences:

1. Gateways are deployed inside tenant infrastructure (private networks, corporate data centers).
2. Gateways may be behind NAT — the Controller's WireGuard hub handles NAT traversal via persistent keepalive.
3. Gateways do not run BGP, DNS, or CDN services — they only forward traffic from the overlay to private destinations.

---

## Lifecycle States

### Node

```
pending → enrolling → online ↔ offline → decommissioned
                                           (terminal state)
```

### Gateway

```
pending → enrolling → online ↔ offline → error
```

### Transitions

| From | To | Trigger |
|------|----|---------|
| pending | enrolling | Controller begins SSH provisioning |
| enrolling | online | Node connects over WireGuard + presents valid token |
| online | offline | Heartbeat timeout (configurable, default 5 minutes) |
| offline | online | Heartbeat resumes |
| any | decommissioned | Operator explicitly decommissions (irreversible) |
| any | error | Provisioning failure or unrecoverable state |

---

## Future Considerations

- **Agent-based provisioning**: instead of SSH-push, nodes could pull their config from the Controller. This would better support environments where SSH access is restricted.
- **SSH host key verification**: store and verify host keys to prevent MITM during provisioning.
- **Canary deployments**: update a subset of nodes first, validate metrics, then proceed.
- **Configuration drift detection**: periodic reconciliation of actual vs. desired state.
