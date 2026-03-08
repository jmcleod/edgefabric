package networking

import (
	"fmt"
	"strings"
)

// WireGuardConfig holds the data needed to render a wg0.conf file.
type WireGuardConfig struct {
	PrivateKey string
	Address    string // e.g., "10.100.0.1/16" or "10.100.0.5/32"
	ListenPort int    // 0 = omit (for nodes that connect outward)
	DNS        string // optional
	Peers      []WireGuardPeerConfig
}

// WireGuardPeerConfig holds one [Peer] section in a WireGuard config.
type WireGuardPeerConfig struct {
	PublicKey           string
	PresharedKey        string   // empty = omit
	AllowedIPs          []string // CIDR blocks
	Endpoint            string   // empty = omit (hub config may omit for peers behind NAT)
	PersistentKeepalive int      // 0 = omit, 25 = recommended for spokes behind NAT
}

// Render produces the wg0.conf text in standard WireGuard format.
func (c *WireGuardConfig) Render() string {
	var b strings.Builder

	b.WriteString("[Interface]\n")
	b.WriteString(fmt.Sprintf("PrivateKey = %s\n", c.PrivateKey))
	b.WriteString(fmt.Sprintf("Address = %s\n", c.Address))
	if c.ListenPort > 0 {
		b.WriteString(fmt.Sprintf("ListenPort = %d\n", c.ListenPort))
	}
	if c.DNS != "" {
		b.WriteString(fmt.Sprintf("DNS = %s\n", c.DNS))
	}

	for _, peer := range c.Peers {
		b.WriteString("\n[Peer]\n")
		b.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		if peer.PresharedKey != "" {
			b.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
		}
		if len(peer.AllowedIPs) > 0 {
			b.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", ")))
		}
		if peer.Endpoint != "" {
			b.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		}
		if peer.PersistentKeepalive > 0 {
			b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
	}

	return b.String()
}
