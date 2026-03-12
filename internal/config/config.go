// Package config handles loading and validating EdgeFabric configuration.
package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"

	"github.com/jmcleod/edgefabric/internal/plugin"

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
	ListenAddr     string                `yaml:"listen_addr"`
	ExternalURL    string                `yaml:"external_url"`
	Storage        StorageConfig         `yaml:"storage"`
	TLS            TLSConfig             `yaml:"tls,omitempty"`
	CORS           CORSConfig            `yaml:"cors,omitempty"`
	Notifications  NotificationsConfig   `yaml:"notifications,omitempty"`
	LeaderElection LeaderElectionConfig  `yaml:"leader_election,omitempty"`
	WireGuard      WireGuardHub          `yaml:"wireguard"`
	Secrets        SecretsConfig         `yaml:"secrets"`
}

// LeaderElectionConfig holds HA leader election settings.
type LeaderElectionConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`  // Enable leader election (auto-enabled for postgres).
	Interval string `yaml:"interval,omitempty"` // Lock check interval (default "5s").
}

// CORSConfig holds Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins,omitempty"` // e.g., ["https://console.example.com"]
}

// NotificationsConfig holds webhook, Slack, and email notification settings.
type NotificationsConfig struct {
	Webhooks []WebhookEndpoint `yaml:"webhooks,omitempty"`
	Slack    SlackNotification `yaml:"slack,omitempty"`
	Email    EmailNotification `yaml:"email,omitempty"`
}

// EmailNotification defines SMTP email notification settings.
type EmailNotification struct {
	SMTPHost   string   `yaml:"smtp_host,omitempty"`
	SMTPPort   int      `yaml:"smtp_port,omitempty"`    // Default 587.
	Username   string   `yaml:"username,omitempty"`
	Password   string   `yaml:"password,omitempty"`
	FromAddr   string   `yaml:"from_addr,omitempty"`
	Recipients []string `yaml:"recipients,omitempty"`
	UseTLS     bool     `yaml:"use_tls,omitempty"`
}

// WebhookEndpoint defines a single webhook notification target.
type WebhookEndpoint struct {
	URL    string `yaml:"url"`
	Secret string `yaml:"secret,omitempty"` // HMAC-SHA256 signing secret.
}

// SlackNotification defines Slack webhook notification settings.
type SlackNotification struct {
	WebhookURL string `yaml:"webhook_url,omitempty"`
	Channel    string `yaml:"channel,omitempty"`
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
	ListenPort  int    `yaml:"listen_port"`
	Subnet      string `yaml:"subnet"`       // e.g., "10.100.0.0/16"
	Address     string `yaml:"address"`      // controller's overlay IP, e.g., "10.100.0.1/16"
	Topology    string `yaml:"topology"`     // "hub-spoke" (default) or "mesh"
	IPv6Subnet  string `yaml:"ipv6_subnet"`  // e.g., "fd00:ef::/48"
	IPv6Address string `yaml:"ipv6_address"` // controller's IPv6 overlay, e.g., "fd00:ef::1/48"
}

// SecretsConfig defines how secrets are encrypted at rest.
type SecretsConfig struct {
	EncryptionKey   string `yaml:"encryption_key"`              // base64-encoded AES-256 key
	TokenSigningKey string `yaml:"token_signing_key,omitempty"` // Separate HMAC key for JWT signing. Falls back to EncryptionKey if empty.
}

// NodeConfig holds node-specific settings.
type NodeConfig struct {
	ControllerAddr  string           `yaml:"controller_addr"`
	EnrollmentToken string           `yaml:"enrollment_token,omitempty"`
	DataDir         string           `yaml:"data_dir"`
	HealthAddr      string           `yaml:"health_addr,omitempty"` // Health/metrics server address. Default ":9090"
	BGP             BGPConfig        `yaml:"bgp,omitempty"`
	DNS             DNSConfig        `yaml:"dns,omitempty"`
	CDN             CDNConfig        `yaml:"cdn,omitempty"`
	Route           RouteConfig      `yaml:"route,omitempty"`
	Monitoring      MonitoringConfig `yaml:"monitoring,omitempty"`
}

// MonitoringConfig holds node-side monitoring settings.
type MonitoringConfig struct {
	OverlayHealthInterval string `yaml:"overlay_health_interval,omitempty"` // Duration string. Default "30s"
	BGPPollInterval       string `yaml:"bgp_poll_interval,omitempty"`       // Duration string. Default "15s"
	RouteHealthInterval   string `yaml:"route_health_interval,omitempty"`   // Duration string. Default "30s"
	ControllerOverlayIP   string `yaml:"controller_overlay_ip,omitempty"`   // WireGuard overlay IP to probe. Default: empty (disabled)
}

// BGPConfig holds BGP runtime settings for a node.
type BGPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	RouterID string `yaml:"router_id,omitempty"` // If empty, uses node's WireGuard IP.
	LocalASN uint32 `yaml:"local_asn,omitempty"`
	Mode     string `yaml:"mode,omitempty"` // "gobgp" or "noop", defaults to "noop"
}

// DNSConfig holds DNS runtime settings for a node.
type DNSConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ListenAddr  string `yaml:"listen_addr,omitempty"` // Default ":5353"
	Mode        string `yaml:"mode,omitempty"`        // "miekg" or "noop", defaults to "noop"
	AXFREnabled bool   `yaml:"axfr_enabled,omitempty"`
}

// CDNConfig holds CDN runtime settings for a node.
type CDNConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ListenAddr    string `yaml:"listen_addr,omitempty"`       // Default ":8080"
	Mode          string `yaml:"mode,omitempty"`              // "proxy" or "noop", defaults to "noop"
	CacheDir      string `yaml:"cache_dir,omitempty"`         // Directory for disk cache. Empty = memory only.
	CacheMaxBytes int64  `yaml:"cache_max_bytes,omitempty"`   // Max disk cache bytes per site. Default 512MB.
}

// RouteConfig holds route forwarding runtime settings for a node.
type RouteConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode,omitempty"` // "forwarder" or "noop", defaults to "noop"
}

// GatewayConfig holds gateway-specific settings.
type GatewayConfig struct {
	ControllerAddr  string `yaml:"controller_addr"`
	EnrollmentToken string `yaml:"enrollment_token,omitempty"`
	DataDir         string `yaml:"data_dir"`
	HealthAddr      string `yaml:"health_addr,omitempty"`  // Health/metrics server address. Default ":9090"
	WireGuardIP     string `yaml:"wireguard_ip,omitempty"` // Gateway's overlay IP for route listeners.
	RouteMode       string `yaml:"route_mode,omitempty"`   // "forwarder" or "noop", defaults to "noop"
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
//	EF_CONTROLLER_SECRETS_ENCRYPTION_KEY   → controller.secrets.encryption_key
//	EF_CONTROLLER_SECRETS_TOKEN_SIGNING_KEY → controller.secrets.token_signing_key
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
	envStr("EF_CONTROLLER_SECRETS_TOKEN_SIGNING_KEY", &c.Controller.Secrets.TokenSigningKey)

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
	// Validate encryption key: required, must be valid base64 decoding to 32 bytes (AES-256).
	if c.Controller.Secrets.EncryptionKey == "" {
		return fmt.Errorf("controller.secrets.encryption_key is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(c.Controller.Secrets.EncryptionKey)
	if err != nil {
		return fmt.Errorf("controller.secrets.encryption_key: invalid base64: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("controller.secrets.encryption_key: must decode to 32 bytes (AES-256), got %d", len(decoded))
	}

	// Validate token signing key if set: must be valid base64 decoding to ≥32 bytes.
	if c.Controller.Secrets.TokenSigningKey != "" {
		sigDecoded, err := base64.StdEncoding.DecodeString(c.Controller.Secrets.TokenSigningKey)
		if err != nil {
			return fmt.Errorf("controller.secrets.token_signing_key: invalid base64: %w", err)
		}
		if len(sigDecoded) < 32 {
			return fmt.Errorf("controller.secrets.token_signing_key: must decode to at least 32 bytes, got %d", len(sigDecoded))
		}
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
	// Validate service mode settings.
	if err := validateMode("node.bgp.mode", c.Node.BGP.Mode, plugin.RegisteredNames(plugin.PluginTypeBGP)); err != nil {
		return err
	}
	if err := validateMode("node.dns.mode", c.Node.DNS.Mode, plugin.RegisteredNames(plugin.PluginTypeDNS)); err != nil {
		return err
	}
	if err := validateMode("node.cdn.mode", c.Node.CDN.Mode, plugin.RegisteredNames(plugin.PluginTypeCDN)); err != nil {
		return err
	}
	if err := validateMode("node.route.mode", c.Node.Route.Mode, plugin.RegisteredNames(plugin.PluginTypeRoute)); err != nil {
		return err
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
	// Validate WireGuard IP if set.
	if c.Gateway.WireGuardIP != "" {
		if net.ParseIP(c.Gateway.WireGuardIP) == nil {
			return fmt.Errorf("gateway.wireguard_ip: invalid IP address: %q", c.Gateway.WireGuardIP)
		}
	}
	if err := validateMode("gateway.route_mode", c.Gateway.RouteMode, plugin.RegisteredNames(plugin.PluginTypeRoute)); err != nil {
		return err
	}
	return nil
}

// validateMode checks that a mode string is one of the allowed values, or empty.
func validateMode(field, value string, allowed []string) error {
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s: unknown mode %q (allowed: %v)", field, value, allowed)
}

// DefaultLogLevel returns the log level, defaulting to "info".
func (c *Config) DefaultLogLevel() string {
	if c.LogLevel == "" {
		return "info"
	}
	return c.LogLevel
}
