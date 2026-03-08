// Package config handles loading and validating EdgeFabric configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RoleType identifies which mode the binary runs in.
type RoleType string

const (
	RoleController RoleType = "controller"
	RoleNode       RoleType = "node"
	RoleGateway    RoleType = "gateway"
)

// Config is the top-level configuration structure.
type Config struct {
	Role       RoleType          `yaml:"role"`
	LogLevel   string            `yaml:"log_level"`
	Controller ControllerConfig  `yaml:"controller,omitempty"`
	Node       NodeConfig        `yaml:"node,omitempty"`
	Gateway    GatewayConfig     `yaml:"gateway,omitempty"`
}

// ControllerConfig holds controller-specific settings.
type ControllerConfig struct {
	ListenAddr     string         `yaml:"listen_addr"`
	ExternalURL    string         `yaml:"external_url"`
	Storage        StorageConfig  `yaml:"storage"`
	TLS            TLSConfig      `yaml:"tls,omitempty"`
	WireGuard      WireGuardHub   `yaml:"wireguard"`
	Secrets        SecretsConfig  `yaml:"secrets"`
}

// StorageConfig defines the persistence backend.
type StorageConfig struct {
	Driver string `yaml:"driver"` // "sqlite" or "postgres"
	DSN    string `yaml:"dsn"`
}

// TLSConfig holds TLS settings for the API.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
	AutoCert bool   `yaml:"auto_cert,omitempty"`
}

// WireGuardHub holds settings for the controller's WireGuard hub.
type WireGuardHub struct {
	ListenPort int    `yaml:"listen_port"`
	Subnet     string `yaml:"subnet"` // e.g., "10.100.0.0/16"
	Address    string `yaml:"address"` // controller's overlay IP, e.g., "10.100.0.1/16"
}

// SecretsConfig defines how secrets are encrypted at rest.
type SecretsConfig struct {
	EncryptionKey string `yaml:"encryption_key"` // base64-encoded AES-256 key
}

// NodeConfig holds node-specific settings.
type NodeConfig struct {
	ControllerAddr string `yaml:"controller_addr"`
	EnrollmentToken string `yaml:"enrollment_token,omitempty"`
	DataDir        string `yaml:"data_dir"`
}

// GatewayConfig holds gateway-specific settings.
type GatewayConfig struct {
	ControllerAddr  string `yaml:"controller_addr"`
	EnrollmentToken string `yaml:"enrollment_token,omitempty"`
	DataDir         string `yaml:"data_dir"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks that the configuration is internally consistent.
func (c *Config) Validate() error {
	switch c.Role {
	case RoleController:
		return c.validateController()
	case RoleNode:
		return c.validateNode()
	case RoleGateway:
		return c.validateGateway()
	case "":
		return fmt.Errorf("role is required")
	default:
		return fmt.Errorf("unknown role: %q", c.Role)
	}
}

func (c *Config) validateController() error {
	if c.Controller.ListenAddr == "" {
		c.Controller.ListenAddr = ":8443"
	}
	if c.Controller.Storage.Driver == "" {
		return fmt.Errorf("controller.storage.driver is required")
	}
	if c.Controller.Storage.Driver != "sqlite" && c.Controller.Storage.Driver != "postgres" {
		return fmt.Errorf("unsupported storage driver: %q", c.Controller.Storage.Driver)
	}
	if c.Controller.Storage.DSN == "" {
		return fmt.Errorf("controller.storage.dsn is required")
	}
	return nil
}

func (c *Config) validateNode() error {
	if c.Node.ControllerAddr == "" {
		return fmt.Errorf("node.controller_addr is required")
	}
	if c.Node.DataDir == "" {
		c.Node.DataDir = "/var/lib/edgefabric"
	}
	return nil
}

func (c *Config) validateGateway() error {
	if c.Gateway.ControllerAddr == "" {
		return fmt.Errorf("gateway.controller_addr is required")
	}
	if c.Gateway.DataDir == "" {
		c.Gateway.DataDir = "/var/lib/edgefabric"
	}
	return nil
}

// DefaultLogLevel returns the log level, defaulting to "info".
func (c *Config) DefaultLogLevel() string {
	if c.LogLevel == "" {
		return "info"
	}
	return c.LogLevel
}
