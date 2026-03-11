package networking

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// generateMeshPeers builds WireGuard peer configs for all nodes that share
// a node group with the given node. Each mesh peer gets a /32 (and /128 for
// IPv6) AllowedIPs entry, which takes priority over the hub's /16 subnet
// route via WireGuard's longest-prefix-match routing.
//
// Returns the list of mesh peer configs and the listen port to set on the
// node's WireGuard interface (mesh nodes must accept incoming connections).
func (s *DefaultService) generateMeshPeers(ctx context.Context, node *domain.Node) ([]WireGuardPeerConfig, int, error) {
	// Find all node groups this node belongs to.
	groups, err := s.nodeGroups.ListNodeGroups_ByNode(ctx, node.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("list node groups for node %s: %w", node.ID, err)
	}

	if len(groups) == 0 {
		return nil, 0, nil // No groups → no mesh peers, hub-spoke only.
	}

	// Collect unique peer node IDs from all groups.
	seen := make(map[domain.ID]bool)
	seen[node.ID] = true // Exclude self.

	var peerNodes []*domain.Node
	for _, g := range groups {
		members, err := s.nodeGroups.ListGroupNodes(ctx, g.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("list group nodes for %s: %w", g.ID, err)
		}
		for _, m := range members {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			peerNodes = append(peerNodes, m)
		}
	}

	var meshPeers []WireGuardPeerConfig
	for _, peerNode := range peerNodes {
		if peerNode.WireGuardIP == "" {
			continue // Skip nodes without WireGuard IP.
		}

		// Get the peer's WireGuard peer record for its public key.
		wgPeer, err := s.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, peerNode.ID)
		if err != nil {
			continue // Skip if no peer record (not yet provisioned).
		}

		// Build AllowedIPs: /32 for IPv4, /128 for IPv6 (if assigned).
		allowedIPs := []string{peerNode.WireGuardIP + "/32"}
		if peerNode.WireGuardIPv6 != "" {
			allowedIPs = append(allowedIPs, peerNode.WireGuardIPv6+"/128")
		}

		// Mesh peers connect via their public IP on the WireGuard listen port.
		endpoint := ""
		if peerNode.PublicIP != "" && s.wgConfig.ListenPort > 0 {
			endpoint = peerNode.PublicIP + ":" + strconv.Itoa(s.wgConfig.ListenPort)
		}

		pc := WireGuardPeerConfig{
			PublicKey:           wgPeer.PublicKey,
			AllowedIPs:          allowedIPs,
			Endpoint:            endpoint,
			PersistentKeepalive: 25,
		}

		meshPeers = append(meshPeers, pc)
	}

	// Mesh nodes need to listen for incoming connections from other mesh peers.
	listenPort := s.wgConfig.ListenPort

	return meshPeers, listenPort, nil
}
