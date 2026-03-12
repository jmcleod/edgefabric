package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateController_EncryptionKey_Valid(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets:    SecretsConfig{EncryptionKey: key},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateController_EncryptionKey_TooShort(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 16)) // 16 bytes, not 32
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets:    SecretsConfig{EncryptionKey: key},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for short encryption key")
	}
	if !strings.Contains(err.Error(), "32 bytes") {
		t.Errorf("expected error about 32 bytes, got: %v", err)
	}
}

func TestValidateController_EncryptionKey_InvalidBase64(t *testing.T) {
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets:    SecretsConfig{EncryptionKey: "not-valid-base64!!!"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !strings.Contains(err.Error(), "invalid base64") {
		t.Errorf("expected error about invalid base64, got: %v", err)
	}
}

func TestValidateController_EncryptionKey_Empty(t *testing.T) {
	// Empty key should now be rejected at validation time.
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets:    SecretsConfig{EncryptionKey: ""},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty encryption_key, got nil")
	}
}

func TestValidateController_TokenSigningKey_Valid(t *testing.T) {
	validKey32 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets: SecretsConfig{
				EncryptionKey:   validKey32,
				TokenSigningKey: validKey32,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateController_TokenSigningKey_TooShort(t *testing.T) {
	validKey32 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets: SecretsConfig{
				EncryptionKey:   validKey32,
				TokenSigningKey: shortKey,
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for short token_signing_key, got nil")
	}
}

func TestValidateController_TokenSigningKey_InvalidBase64(t *testing.T) {
	validKey32 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg := &Config{
		Role: RoleController,
		Controller: ControllerConfig{
			ListenAddr: ":8443",
			Storage:    StorageConfig{Driver: "sqlite", DSN: "test.db"},
			Secrets: SecretsConfig{
				EncryptionKey:   validKey32,
				TokenSigningKey: "not-valid-base64!!!",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid base64 token_signing_key, got nil")
	}
}

func TestValidateNode_Modes_Valid(t *testing.T) {
	modes := []struct {
		bgp, dns, cdn, route string
	}{
		{"gobgp", "miekg", "proxy", "forwarder"},
		{"noop", "noop", "noop", "noop"},
		{"", "", "", ""},
	}
	for _, m := range modes {
		cfg := &Config{
			Role: RoleNode,
			Node: NodeConfig{
				ControllerAddr: "http://localhost:8443",
				BGP:            BGPConfig{Mode: m.bgp},
				DNS:            DNSConfig{Mode: m.dns},
				CDN:            CDNConfig{Mode: m.cdn},
				Route:          RouteConfig{Mode: m.route},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config for modes %+v, got: %v", m, err)
		}
	}
}

func TestValidateNode_Modes_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		field string
		cfg   NodeConfig
	}{
		{"bad BGP mode", "node.bgp.mode", NodeConfig{ControllerAddr: "x", BGP: BGPConfig{Mode: "invalid"}}},
		{"bad DNS mode", "node.dns.mode", NodeConfig{ControllerAddr: "x", DNS: DNSConfig{Mode: "invalid"}}},
		{"bad CDN mode", "node.cdn.mode", NodeConfig{ControllerAddr: "x", CDN: CDNConfig{Mode: "invalid"}}},
		{"bad route mode", "node.route.mode", NodeConfig{ControllerAddr: "x", Route: RouteConfig{Mode: "invalid"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Role: RoleNode, Node: tt.cfg}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error for invalid mode")
			}
			if !strings.Contains(err.Error(), tt.field) {
				t.Errorf("expected error to mention %q, got: %v", tt.field, err)
			}
		})
	}
}

func TestValidateGateway_WireGuardIP_Valid(t *testing.T) {
	cfg := &Config{
		Role: RoleGateway,
		Gateway: GatewayConfig{
			ControllerAddr: "http://localhost:8443",
			WireGuardIP:    "10.100.0.5",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateGateway_WireGuardIP_Invalid(t *testing.T) {
	cfg := &Config{
		Role: RoleGateway,
		Gateway: GatewayConfig{
			ControllerAddr: "http://localhost:8443",
			WireGuardIP:    "not-an-ip",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid WireGuard IP")
	}
	if !strings.Contains(err.Error(), "invalid IP") {
		t.Errorf("expected error about invalid IP, got: %v", err)
	}
}

func TestValidateGateway_RouteMode_Invalid(t *testing.T) {
	cfg := &Config{
		Role: RoleGateway,
		Gateway: GatewayConfig{
			ControllerAddr: "http://localhost:8443",
			RouteMode:      "bogus",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid route mode")
	}
	if !strings.Contains(err.Error(), "gateway.route_mode") {
		t.Errorf("expected error about gateway.route_mode, got: %v", err)
	}
}
