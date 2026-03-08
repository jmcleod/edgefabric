package provisioning_test

import (
	"encoding/base64"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
)

func TestGenerateWireGuardKeyPair(t *testing.T) {
	kp, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	// Verify base64 encoding.
	privBytes, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("expected 32-byte private key, got %d", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("expected 32-byte public key, got %d", len(pubBytes))
	}

	// Verify clamping of private key.
	if privBytes[0]&7 != 0 {
		t.Error("private key not clamped: low 3 bits of first byte should be 0")
	}
	if privBytes[31]&128 != 0 {
		t.Error("private key not clamped: high bit of last byte should be 0")
	}
	if privBytes[31]&64 == 0 {
		t.Error("private key not clamped: second-highest bit of last byte should be 1")
	}

	// Generate a second key pair — should be different.
	kp2, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("generate second key pair: %v", err)
	}
	if kp.PrivateKey == kp2.PrivateKey {
		t.Error("two generated key pairs should have different private keys")
	}
	if kp.PublicKey == kp2.PublicKey {
		t.Error("two generated key pairs should have different public keys")
	}
}

func TestGeneratePresharedKey(t *testing.T) {
	psk, err := provisioning.GeneratePresharedKey()
	if err != nil {
		t.Fatalf("generate preshared key: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		t.Fatalf("decode preshared key: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32-byte preshared key, got %d", len(decoded))
	}
}

func TestAllocateOverlayIP(t *testing.T) {
	subnet := "10.100.0.0/24"
	controllerAddr := "10.100.0.1/24"

	// First allocation should get .2 (controller has .1).
	ip, err := provisioning.AllocateOverlayIP(subnet, controllerAddr, nil, nil)
	if err != nil {
		t.Fatalf("allocate IP: %v", err)
	}
	if ip != "10.100.0.2" {
		t.Errorf("expected 10.100.0.2, got %s", ip)
	}
}

func TestAllocateOverlayIPSequential(t *testing.T) {
	subnet := "10.100.0.0/24"
	controllerAddr := "10.100.0.1/24"

	// Simulate existing peers at .2 and .3.
	peers := []*domain.WireGuardPeer{
		{AllowedIPs: []string{"10.100.0.2/32"}},
		{AllowedIPs: []string{"10.100.0.3/32"}},
	}

	ip, err := provisioning.AllocateOverlayIP(subnet, controllerAddr, peers, nil)
	if err != nil {
		t.Fatalf("allocate IP: %v", err)
	}
	if ip != "10.100.0.4" {
		t.Errorf("expected 10.100.0.4, got %s", ip)
	}
}

func TestAllocateOverlayIPGapFilling(t *testing.T) {
	subnet := "10.100.0.0/24"
	controllerAddr := "10.100.0.1/24"

	// Simulate a gap: .2 is used, .3 is free, .4 is used.
	peers := []*domain.WireGuardPeer{
		{AllowedIPs: []string{"10.100.0.2/32"}},
		{AllowedIPs: []string{"10.100.0.4/32"}},
	}

	ip, err := provisioning.AllocateOverlayIP(subnet, controllerAddr, peers, nil)
	if err != nil {
		t.Fatalf("allocate IP: %v", err)
	}
	if ip != "10.100.0.3" {
		t.Errorf("expected 10.100.0.3 (gap fill), got %s", ip)
	}
}

func TestAllocateOverlayIPFromNodeWireGuardIP(t *testing.T) {
	subnet := "10.100.0.0/24"
	controllerAddr := "10.100.0.1/24"

	// Node already has a WireGuard IP assigned.
	nodes := []*domain.Node{
		{WireGuardIP: "10.100.0.2"},
	}

	ip, err := provisioning.AllocateOverlayIP(subnet, controllerAddr, nil, nodes)
	if err != nil {
		t.Fatalf("allocate IP: %v", err)
	}
	if ip != "10.100.0.3" {
		t.Errorf("expected 10.100.0.3, got %s", ip)
	}
}

func TestAllocateOverlayIPExhaustion(t *testing.T) {
	// Tiny /30 subnet: .0 (network), .1 (controller), .2, .3 (broadcast).
	// Only .2 is available, fill it.
	subnet := "10.100.0.0/30"
	controllerAddr := "10.100.0.1/30"

	peers := []*domain.WireGuardPeer{
		{AllowedIPs: []string{"10.100.0.2/32"}},
	}

	_, err := provisioning.AllocateOverlayIP(subnet, controllerAddr, peers, nil)
	if err == nil {
		t.Fatal("expected exhaustion error")
	}
}
