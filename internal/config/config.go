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

// Load reads and parses a YAML config file, then applies environment variable
// overrides. Environment variables take precedence over YAML values, enabling
// twelve-factor-style configuration in container deployments.
//
// Supported environment variables:
//
//	EF_ROLE                           → role
//	EF_LOG_LEVEL                      → log_level
//	EF_CONTROLLER_LISTEN_ADDR         → controller.listen_addr
//	EF_CONTROLLER_EXTERNAL_URL        → controller.external_url
//	EF_CONTROLLER_STORAGE_DRIVER      → controller.storage.driver
//	EF_CONTROLLER_STORAGE_DSN         → controller.storage.dsn
//	EF_CONTROLLER_SECRETS_ENCRYPTION_KEY → controller.secrets.encryption_key
//	EF_NODE_CONTROLLER_ADDR           → node.controller_addr
//	EF_NODE_ENROLLMENT_TOKEN          → node.enrollment_token
//	EF_NODE_DATA_DIR                  → node.data_dir
//	EF_GATEWAY_CONTROLLER_ADDR        → gateway.controller_addr
//	EF_GATEWAY_ENROLLMENT_TOKEN       → gateway.enrollment_token
//	EF_GATEWAY_DATA_DIR               → gateway.data_dir
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Each EF_-prefixed variable overrides the corresponding YAML field.
// Only non-empty environment values are applied.
func (c *Config) applyEnvOverrides() {
	envStr := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			*dst = v
		}
	}

	// Top-level.
	if v, ok := os.LookupEnv("EF_ROLE"); ok && v != "" {
		c.Role = RoleType(v)
	}
	envStr("EF_LOG_LEVEL", &c.LogLevel)

	// Controller.
	envStr("EF_CONTROLLER_LISTEN_ADDR", &c.Controller.ListenAddr)
	envStr("EF_CONTROLLER_EXTERNAL_URL", &c.Controller.ExternalURL)
	envStr("EF_CONTROLLER_STORAGE_DRIVER", &c.Controller.Storage.Driver)
	envStr("EF_CONTROLLER_STORAGE_DSN", &c.Controller.Storage.DSN)
	envStr("EF_CONTROLLER_SECRETS_ENCRYPTION_KEY", &c.Controller.Secrets.EncryptionKey)

	// Node.
	envStr("EF_NODE_CONTROLLER_ADDR", &c.Node.ControllerAddr)
	envStr("EF_NODE_ENROLLMENT_TOKEN", &c.Node.EnrollmentToken)
	envStr("EF_NODE_DATA_DIR", &c.Node.DataDir)

	// Gateway.
	envStr("EF_GATEWAY_CONTROLLER_ADDR", &c.Gateway.ControllerAddr)
	envStr("EF_GATEWAY_ENROLLMENT_TOKEN", &c.Gateway.EnrollmentToken)
	envStr("EF_GATEWAY_DATA_DIR", &c.Gateway.DataDir)
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
