# Networking Model

## Overview

EdgeFabric's networking is built on three layers: a WireGuard overlay for control and data plane connectivity, BGP for anycast IP announcement, and application-level proxying for DNS/CDN/Route services.

---

## WireGuard Overlay

### Topology: Hub-and-Spoke

```
                    ┌──────────────┐
                    │  Controller  │
                    │  (Hub)       │
                    │  10.100.0.1  │
                    └──────┬───────┘
              ┌────────────┼────────────┐
              │            │            │
     ┌────────▼───┐ ┌──────▼────┐ ┌────▼──────┐
     │  Node A    │ │  Node B   │ │ Gateway X │
     │ 10.100.1.1 │ │ 10.100.1.2│ │ 10.100.2.1│
     └────────────┘ └───────────┘ └───────────┘
```

- **Controller** is always the WireGuard hub (`10.100.0.1/16` by default).
- **Nodes** and **Gateways** are spokes connecting to the controller.
- Default overlay subnet: `10.100.0.0/16` (configurable).
- All management traffic (heartbeats, config sync, provisioning) flows over this overlay.
- Forwarded data traffic (Route service) also uses this overlay in v1.

### Security Assumptions

- All WireGuard key generation happens on the Controller. Private keys are encrypted at rest using AES-256-GCM.
- Key rotation is Controller-initiated and will be implemented in Milestone 3.
- Preshared keys are optional but recommended for post-quantum resistance.
- The overlay is trusted — once a peer authenticates via WireGuard, internal services accept connections without additional transport-level auth.

### Why Hub-and-Spoke (Not Full Mesh)

- Simpler key management: each spoke only needs the hub's public key.
- Easier NAT traversal: only the hub needs a stable public endpoint.
- Sufficient for v1 traffic patterns (control plane + node-to-gateway forwarding).
- Full mesh is out of scope for v1 but the `WireGuardPeer` model supports it.

---

## IP Allocation

Tenants are assigned IP prefixes (`IPAllocation` entity) that Nodes announce via BGP. Key rules:

- Each prefix belongs to exactly one tenant at a time (exclusivity invariant).
- Types: `anycast` (announced from multiple nodes) or `unicast` (single node).
- Purpose tags: `dns`, `cdn`, `route` — determines which service binds to the IP.
- Status: `active` (announced), `withdrawn` (removed from BGP), `pending` (not yet configured).

---

## BGP

### Design

Each Node runs GoBGP as a library (not a sidecar). BGP sessions are configured by the Controller and synced to nodes over the WireGuard overlay.

### Session Model

```
BGPSession {
    NodeID            → which node runs this session
    PeerASN/Address   → upstream router to peer with
    LocalASN          → our ASN on this node
    AnnouncedPrefixes → CIDRs from IPAllocation
    Status            → configured | established | idle | error
}
```

### Anycast Behavior

When multiple Nodes announce the same prefix, upstream routers see multiple paths. Traffic is attracted to the nearest Node based on BGP path selection (shortest AS path, lowest MED, etc.). This creates **anycast** — the same IP answers from the geographically closest Node.

### Assumptions Documented

- EdgeFabric assumes the operator provides appropriate ASNs and has peering arrangements with upstream routers.
- EdgeFabric does not participate in full-table BGP — it only announces tenant prefixes.
- BGP session health is reported to the Controller and visible via the API.
- If a Node goes offline, its BGP sessions drop and upstream routers reconverge to other Nodes announcing the same prefix.

---

## Traffic Flows

### DNS (Anycast)

```
Client → (anycast IP:53/UDP) → Node DNS Server → Authoritative response
```

- Zone data is synced from Controller to Nodes over WireGuard.
- Nodes serve authoritative DNS using `miekg/dns`.
- Multiple Nodes serving the same zone = anycast DNS.

### CDN (Reverse Proxy)

```
Client → (anycast IP:80/443) → Node CDN Proxy → (cache hit? serve : fetch from origin)
```

- TLS termination on the Node (auto-cert or manual certificates).
- Disk-based response cache with configurable TTL.
- Origin health checks and weighted load balancing across multiple origins.

### Route/Gateway (TCP/UDP/ICMP Forwarding)

```
Client → (anycast IP:port) → Node → (WireGuard tunnel) → Gateway → Private destination
```

- Node accepts traffic on the configured entry IP/port.
- Node forwards to the assigned Gateway over the WireGuard overlay.
- Gateway forwards to the private destination (RFC1918 address).
- Return traffic follows the reverse path with connection tracking/NAT.

---

## Port Allocation

| Service | Default Port | Protocol | Notes |
|---------|-------------|----------|-------|
| Controller API | 8443 | TCP (HTTP/HTTPS) | Configurable via `listen_addr` |
| WireGuard | 51820 | UDP | Configurable via `wireguard.listen_port` |
| DNS | 53 | UDP/TCP | Bound to anycast IPs |
| CDN | 80, 443 | TCP | Bound to anycast IPs |
| Route | Per-route config | TCP/UDP/ICMP | Bound to anycast IPs |
| Prometheus metrics | 8443/metrics | TCP | Same port as API |

---

## Future Considerations

- **Full mesh WireGuard**: enables direct node-to-node traffic without hub relay. Model supports it; implementation deferred.
- **IPv6**: overlay and anycast support for IPv6 prefixes.
- **ECMP**: multiple gateways per route for load distribution.
- **Direct return**: bypass the overlay for return traffic where possible.
