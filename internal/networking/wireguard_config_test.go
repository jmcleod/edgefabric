package networking

import (
	"strings"
	"testing"
)

func TestWireGuardConfigRenderHub(t *testing.T) {
	cfg := &WireGuardConfig{
		PrivateKey: "hub-private-key-base64",
		Address:    "10.100.0.1/16",
		ListenPort: 51820,
		Peers: []WireGuardPeerConfig{
			{
				PublicKey:   "node1-pub-key",
				PresharedKey: "psk-1",
				AllowedIPs:  []string{"10.100.0.2/32"},
				Endpoint:    "203.0.113.1:51820",
			},
			{
				PublicKey:   "node2-pub-key",
				PresharedKey: "psk-2",
				AllowedIPs:  []string{"10.100.0.3/32"},
				Endpoint:    "203.0.113.2:51820",
			},
		},
	}

	rendered := cfg.Render()

	// Verify [Interface] section.
	if !strings.Contains(rendered, "[Interface]") {
		t.Error("expected [Interface] section")
	}
	if !strings.Contains(rendered, "PrivateKey = hub-private-key-base64") {
		t.Error("expected hub private key")
	}
	if !strings.Contains(rendered, "Address = 10.100.0.1/16") {
		t.Error("expected hub address")
	}
	if !strings.Contains(rendered, "ListenPort = 51820") {
		t.Error("expected listen port")
	}

	// Verify [Peer] sections.
	peerCount := strings.Count(rendered, "[Peer]")
	if peerCount != 2 {
		t.Errorf("expected 2 peer sections, got %d", peerCount)
	}
	if !strings.Contains(rendered, "PublicKey = node1-pub-key") {
		t.Error("expected node1 public key")
	}
	if !strings.Contains(rendered, "PresharedKey = psk-1") {
		t.Error("expected psk-1")
	}
	if !strings.Contains(rendered, "AllowedIPs = 10.100.0.2/32") {
		t.Error("expected node1 allowed IPs")
	}
	if !strings.Contains(rendered, "Endpoint = 203.0.113.1:51820") {
		t.Error("expected node1 endpoint")
	}

	// Hub config should NOT have PersistentKeepalive.
	if strings.Contains(rendered, "PersistentKeepalive") {
		t.Error("hub config should not have PersistentKeepalive")
	}
}

func TestWireGuardConfigRenderNode(t *testing.T) {
	cfg := &WireGuardConfig{
		PrivateKey: "node-private-key-base64",
		Address:    "10.100.0.5/32",
		// No ListenPort for spoke node.
		Peers: []WireGuardPeerConfig{
			{
				PublicKey:           "hub-pub-key",
				PresharedKey:        "psk-hub",
				AllowedIPs:          []string{"10.100.0.0/16"},
				Endpoint:            "controller.example.com:51820",
				PersistentKeepalive: 25,
			},
		},
	}

	rendered := cfg.Render()

	// Verify node config specifics.
	if !strings.Contains(rendered, "Address = 10.100.0.5/32") {
		t.Error("expected node address /32")
	}
	if strings.Contains(rendered, "ListenPort") {
		t.Error("node config should not have ListenPort")
	}

	// Verify single hub peer.
	peerCount := strings.Count(rendered, "[Peer]")
	if peerCount != 1 {
		t.Errorf("expected 1 peer section, got %d", peerCount)
	}
	if !strings.Contains(rendered, "PublicKey = hub-pub-key") {
		t.Error("expected hub public key")
	}
	if !strings.Contains(rendered, "AllowedIPs = 10.100.0.0/16") {
		t.Error("expected hub AllowedIPs (entire overlay subnet)")
	}
	if !strings.Contains(rendered, "Endpoint = controller.example.com:51820") {
		t.Error("expected hub endpoint")
	}
	if !strings.Contains(rendered, "PersistentKeepalive = 25") {
		t.Error("expected PersistentKeepalive = 25 for spoke node")
	}
}

func TestWireGuardConfigRenderEmpty(t *testing.T) {
	cfg := &WireGuardConfig{
		PrivateKey: "key",
		Address:    "10.0.0.1/32",
	}

	rendered := cfg.Render()
	if !strings.Contains(rendered, "[Interface]") {
		t.Error("expected [Interface] section")
	}
	if strings.Contains(rendered, "[Peer]") {
		t.Error("expected no peer sections")
	}
}

func TestWireGuardConfigRenderDNS(t *testing.T) {
	cfg := &WireGuardConfig{
		PrivateKey: "key",
		Address:    "10.0.0.1/32",
		DNS:        "10.100.0.1",
	}

	rendered := cfg.Render()
	if !strings.Contains(rendered, "DNS = 10.100.0.1") {
		t.Error("expected DNS setting")
	}
}
