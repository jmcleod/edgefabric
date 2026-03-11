package provisioning

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"

	"golang.org/x/crypto/curve25519"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// WireGuardKeyPair holds a generated Curve25519 key pair for WireGuard.
type WireGuardKeyPair struct {
	PrivateKey string // Base64-encoded 32-byte private key.
	PublicKey  string // Base64-encoded 32-byte public key.
}

// GenerateWireGuardKeyPair generates a new Curve25519 key pair for WireGuard.
// The private key is clamped per the WireGuard specification.
func GenerateWireGuardKeyPair() (*WireGuardKeyPair, error) {
	// Generate 32 random bytes for the private key.
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	// Clamp private key per Curve25519/WireGuard spec.
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key.
	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}

	return &WireGuardKeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
	}, nil
}

// GeneratePresharedKey generates a random 32-byte preshared key for WireGuard.
func GeneratePresharedKey() (string, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", fmt.Errorf("generate preshared key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key[:]), nil
}

// AllocateOverlayIP finds the next available IP in the overlay subnet.
// It queries existing WireGuard peers and nodes for used IPs and finds the first gap.
// The controller address (e.g., 10.100.0.1) is always reserved.
func AllocateOverlayIP(subnet string, controllerAddr string, peers []*domain.WireGuardPeer, nodes []*domain.Node) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("parse subnet %q: %w", subnet, err)
	}

	// Dispatch to IPv6 allocator if subnet is IPv6.
	networkIP := ipNet.IP.To4()
	if networkIP == nil {
		return allocateOverlayIPv6(ipNet, controllerAddr, peers, nodes)
	}

	// Build set of used IPs.
	used := make(map[string]bool)

	// Reserve network and broadcast addresses.
	used[networkIP.String()] = true

	// Reserve broadcast.
	broadcast := broadcastAddr(ipNet)
	used[broadcast.String()] = true

	// Reserve controller address.
	if controllerAddr != "" {
		ctrlIP, _, err := net.ParseCIDR(controllerAddr)
		if err != nil {
			// Try parsing as plain IP.
			ctrlIP = net.ParseIP(controllerAddr)
		}
		if ctrlIP != nil {
			used[ctrlIP.To4().String()] = true
		}
	}

	// Collect IPs from existing WireGuard peers.
	for _, p := range peers {
		for _, cidr := range p.AllowedIPs {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				ip = net.ParseIP(cidr)
			}
			if ip != nil {
				used[ip.To4().String()] = true
			}
		}
	}

	// Collect WireGuard IPs from existing nodes.
	for _, n := range nodes {
		if n.WireGuardIP != "" {
			ip := net.ParseIP(n.WireGuardIP)
			if ip != nil {
				used[ip.To4().String()] = true
			}
		}
	}

	// Iterate through subnet to find first available IP.
	// Start from network+1 (skip network address).
	ip := make(net.IP, 4)
	copy(ip, networkIP)
	incrementIP(ip)

	for ipNet.Contains(ip) {
		if !used[ip.String()] {
			return ip.String(), nil
		}
		incrementIP(ip)
	}

	return "", fmt.Errorf("%w: overlay subnet %s exhausted", storage.ErrConflict, subnet)
}

// incrementIP increments an IPv4 address by 1.
func incrementIP(ip net.IP) {
	v := binary.BigEndian.Uint32(ip.To4())
	v++
	binary.BigEndian.PutUint32(ip, v)
}

// broadcastAddr returns the broadcast address for a network.
func broadcastAddr(n *net.IPNet) net.IP {
	ip := make(net.IP, len(n.IP.To4()))
	copy(ip, n.IP.To4())
	for i := range ip {
		ip[i] |= ^n.Mask[i]
	}
	return ip
}

// allocateOverlayIPv6 finds the next available IPv6 address in the overlay subnet.
// It uses math/big for 128-bit address arithmetic.
func allocateOverlayIPv6(ipNet *net.IPNet, controllerAddr string, peers []*domain.WireGuardPeer, nodes []*domain.Node) (string, error) {
	// Build set of used IPv6 addresses.
	used := make(map[string]bool)

	// Reserve network address.
	networkIP := ipNet.IP.To16()
	used[networkIP.String()] = true

	// Reserve controller address.
	if controllerAddr != "" {
		ctrlIP, _, err := net.ParseCIDR(controllerAddr)
		if err != nil {
			ctrlIP = net.ParseIP(controllerAddr)
		}
		if ctrlIP != nil {
			used[ctrlIP.To16().String()] = true
		}
	}

	// Collect IPs from existing WireGuard peers.
	for _, p := range peers {
		for _, cidr := range p.AllowedIPs {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				ip = net.ParseIP(cidr)
			}
			if ip != nil && ip.To4() == nil {
				used[ip.To16().String()] = true
			}
		}
	}

	// Collect WireGuard IPv6 IPs from existing nodes.
	for _, n := range nodes {
		if n.WireGuardIPv6 != "" {
			ip := net.ParseIP(n.WireGuardIPv6)
			if ip != nil {
				used[ip.To16().String()] = true
			}
		}
	}

	// Compute last address in subnet.
	lastIP := lastIPv6Addr(ipNet)

	// Iterate from network+1 to find first available.
	current := new(big.Int).SetBytes(networkIP)
	one := big.NewInt(1)
	last := new(big.Int).SetBytes(lastIP)

	current.Add(current, one) // Skip network address.
	for current.Cmp(last) <= 0 {
		b := current.Bytes()
		// Pad to 16 bytes.
		ip := make(net.IP, 16)
		copy(ip[16-len(b):], b)

		if !used[ip.String()] {
			return ip.String(), nil
		}
		current.Add(current, one)
	}

	return "", fmt.Errorf("%w: overlay IPv6 subnet %s exhausted", storage.ErrConflict, ipNet.String())
}

// lastIPv6Addr returns the last address in an IPv6 subnet.
func lastIPv6Addr(n *net.IPNet) net.IP {
	ip := make(net.IP, 16)
	copy(ip, n.IP.To16())
	for i := range ip {
		ip[i] |= ^n.Mask[i]
	}
	return ip
}
