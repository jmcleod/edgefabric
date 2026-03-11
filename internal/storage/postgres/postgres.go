// Package postgres implements the storage.Store interface using PostgreSQL.
//
// This is the HA storage driver for multi-instance EdgeFabric controller
// deployments. It uses pgx via the database/sql compatibility layer.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx as "pgx" driver.

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Ensure PostgresStore implements storage.Store at compile time.
var _ storage.Store = (*PostgresStore)(nil)

// Ensure PostgresStore implements storage.SchemaVersioner at compile time.
var _ storage.SchemaVersioner = (*PostgresStore)(nil)

// PostgresStore implements storage.Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// New creates a new PostgreSQL store with connection pooling.
func New(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Connection pool tuning.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Verify connectivity.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// DB returns the underlying *sql.DB for use by the leader elector.
// Advisory locks require a dedicated connection from the same pool.
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Ping checks database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Migrate runs database migrations to ensure the schema is current.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	// Bootstrap the version tracking table.
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_versions (
		version INTEGER PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("create schema_versions table: %w", err)
	}

	for i, m := range migrations {
		version := i + 1

		var exists int
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM schema_versions WHERE version = $1", version,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check schema version %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}

		if _, err := s.db.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("migration %d (%s): %w", version, m.Description, err)
		}

		if _, err := s.db.ExecContext(ctx,
			"INSERT INTO schema_versions (version, description) VALUES ($1, $2)",
			version, m.Description,
		); err != nil {
			return fmt.Errorf("record schema version %d: %w", version, err)
		}

		slog.Info("applied migration",
			slog.Int("version", version),
			slog.String("description", m.Description),
		)
	}
	return nil
}

// SchemaVersion returns the latest applied schema version number.
func (s *PostgresStore) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_versions",
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("query schema version: %w", err)
	}
	return version, nil
}

// isUniqueViolation checks if an error is a PostgreSQL unique constraint violation (23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx wraps errors; check the error string for the PG unique violation code.
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}

// nullString converts a *string to sql.NullString.
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullIDString converts a *domain.ID to sql.NullString for SQL.
func nullIDString(id *domain.ID) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: id.String(), Valid: true}
}

// nullStringEmpty converts an empty string to NULL, non-empty to a valid NullString.
func nullStringEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullIntPtr converts a *int to sql.NullInt64.
func nullIntPtr(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

// Migration describes a single schema migration step.
type Migration struct {
	SQL         string
	Description string
}

// migrations contains the PostgreSQL DDL for the schema.
// Each migration is applied at most once, tracked by schema_versions.
var migrations = []Migration{
	{
		Description: "create tenants table",
		SQL: `CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		slug TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL DEFAULT 'active',
		settings JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create users table",
		SQL: `CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT REFERENCES tenants(id),
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		totp_secret TEXT,
		totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		role TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		last_login_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create api_keys table",
		SQL: `CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		user_id TEXT NOT NULL REFERENCES users(id),
		name TEXT NOT NULL,
		key_hash TEXT NOT NULL,
		key_prefix TEXT NOT NULL,
		role TEXT NOT NULL,
		expires_at TIMESTAMPTZ,
		last_used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create ssh_keys table",
		SQL: `CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_rotated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create nodes table",
		SQL: `CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		tenant_id TEXT REFERENCES tenants(id),
		name TEXT NOT NULL,
		hostname TEXT NOT NULL,
		public_ip TEXT NOT NULL,
		wireguard_ip TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		region TEXT,
		provider TEXT,
		ssh_port INTEGER NOT NULL DEFAULT 22,
		ssh_user TEXT NOT NULL DEFAULT 'root',
		ssh_key_id TEXT REFERENCES ssh_keys(id),
		binary_version TEXT,
		last_heartbeat TIMESTAMPTZ,
		last_config_sync TIMESTAMPTZ,
		metadata JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create node_capabilities table",
		SQL: `CREATE TABLE IF NOT EXISTS node_capabilities (
		node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		capability TEXT NOT NULL,
		PRIMARY KEY (node_id, capability)
	)`,
	},

	{
		Description: "create node_groups table",
		SQL: `CREATE TABLE IF NOT EXISTS node_groups (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create node_group_memberships table",
		SQL: `CREATE TABLE IF NOT EXISTS node_group_memberships (
		node_group_id TEXT NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
		node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		PRIMARY KEY (node_group_id, node_id)
	)`,
	},

	{
		Description: "create gateways table",
		SQL: `CREATE TABLE IF NOT EXISTS gateways (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		public_ip TEXT,
		wireguard_ip TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		enrollment_token TEXT,
		last_heartbeat TIMESTAMPTZ,
		last_config_sync TIMESTAMPTZ,
		metadata JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create wireguard_peers table",
		SQL: `CREATE TABLE IF NOT EXISTS wireguard_peers (
		id TEXT PRIMARY KEY,
		owner_type TEXT NOT NULL,
		owner_id TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT NOT NULL,
		preshared_key TEXT,
		endpoint TEXT,
		last_rotated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create wireguard_peer_allowed_ips table",
		SQL: `CREATE TABLE IF NOT EXISTS wireguard_peer_allowed_ips (
		peer_id TEXT NOT NULL REFERENCES wireguard_peers(id) ON DELETE CASCADE,
		allowed_ip TEXT NOT NULL,
		PRIMARY KEY (peer_id, allowed_ip)
	)`,
	},

	{
		Description: "create ip_allocations table",
		SQL: `CREATE TABLE IF NOT EXISTS ip_allocations (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		prefix TEXT NOT NULL,
		type TEXT NOT NULL,
		purpose TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create bgp_sessions table",
		SQL: `CREATE TABLE IF NOT EXISTS bgp_sessions (
		id TEXT PRIMARY KEY,
		node_id TEXT NOT NULL REFERENCES nodes(id),
		peer_asn INTEGER NOT NULL,
		peer_address TEXT NOT NULL,
		local_asn INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'configured',
		import_policy TEXT,
		export_policy TEXT,
		metadata JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create bgp_announced_prefixes table",
		SQL: `CREATE TABLE IF NOT EXISTS bgp_announced_prefixes (
		session_id TEXT NOT NULL REFERENCES bgp_sessions(id) ON DELETE CASCADE,
		prefix TEXT NOT NULL,
		PRIMARY KEY (session_id, prefix)
	)`,
	},

	{
		Description: "create dns_zones table",
		SQL: `CREATE TABLE IF NOT EXISTS dns_zones (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		serial INTEGER NOT NULL DEFAULT 1,
		ttl INTEGER NOT NULL DEFAULT 3600,
		node_group_id TEXT REFERENCES node_groups(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create dns_records table",
		SQL: `CREATE TABLE IF NOT EXISTS dns_records (
		id TEXT PRIMARY KEY,
		zone_id TEXT NOT NULL REFERENCES dns_zones(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		ttl INTEGER,
		priority INTEGER,
		weight INTEGER,
		port INTEGER,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create cdn_sites table",
		SQL: `CREATE TABLE IF NOT EXISTS cdn_sites (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		tls_mode TEXT NOT NULL DEFAULT 'auto',
		tls_cert_id TEXT,
		cache_enabled BOOLEAN NOT NULL DEFAULT TRUE,
		cache_ttl INTEGER NOT NULL DEFAULT 3600,
		compression_enabled BOOLEAN NOT NULL DEFAULT TRUE,
		rate_limit_rps INTEGER,
		node_group_id TEXT REFERENCES node_groups(id),
		header_rules JSONB,
		status TEXT NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create cdn_site_domains table",
		SQL: `CREATE TABLE IF NOT EXISTS cdn_site_domains (
		site_id TEXT NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
		domain TEXT NOT NULL,
		PRIMARY KEY (site_id, domain)
	)`,
	},

	{
		Description: "create cdn_origins table",
		SQL: `CREATE TABLE IF NOT EXISTS cdn_origins (
		id TEXT PRIMARY KEY,
		site_id TEXT NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
		address TEXT NOT NULL,
		scheme TEXT NOT NULL DEFAULT 'https',
		weight INTEGER NOT NULL DEFAULT 1,
		health_check_path TEXT,
		health_check_interval INTEGER,
		status TEXT NOT NULL DEFAULT 'unknown',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create routes table",
		SQL: `CREATE TABLE IF NOT EXISTS routes (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		protocol TEXT NOT NULL,
		entry_ip TEXT NOT NULL,
		entry_port INTEGER,
		gateway_id TEXT NOT NULL REFERENCES gateways(id),
		destination_ip TEXT NOT NULL,
		destination_port INTEGER,
		node_group_id TEXT REFERENCES node_groups(id),
		status TEXT NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create tls_certificates table",
		SQL: `CREATE TABLE IF NOT EXISTS tls_certificates (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		cert_pem TEXT NOT NULL,
		key_pem TEXT NOT NULL,
		issuer TEXT,
		expires_at TIMESTAMPTZ NOT NULL,
		auto_renew BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create tls_certificate_domains table",
		SQL: `CREATE TABLE IF NOT EXISTS tls_certificate_domains (
		cert_id TEXT NOT NULL REFERENCES tls_certificates(id) ON DELETE CASCADE,
		domain TEXT NOT NULL,
		PRIMARY KEY (cert_id, domain)
	)`,
	},

	{
		Description: "create audit_events table",
		SQL: `CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		tenant_id TEXT,
		user_id TEXT,
		api_key_id TEXT,
		action TEXT NOT NULL,
		resource TEXT NOT NULL,
		details JSONB,
		source_ip TEXT NOT NULL,
		timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create audit_events tenant index",
		SQL:         `CREATE INDEX IF NOT EXISTS idx_audit_events_tenant ON audit_events(tenant_id)`,
	},

	{
		Description: "create audit_events timestamp index",
		SQL:         `CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp)`,
	},

	{
		Description: "create enrollment_tokens table",
		SQL: `CREATE TABLE IF NOT EXISTS enrollment_tokens (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at TIMESTAMPTZ NOT NULL,
		used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create provisioning_jobs table",
		SQL: `CREATE TABLE IF NOT EXISTS provisioning_jobs (
		id TEXT PRIMARY KEY,
		node_id TEXT NOT NULL REFERENCES nodes(id),
		tenant_id TEXT REFERENCES tenants(id),
		action TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		current_step TEXT NOT NULL DEFAULT '',
		steps JSONB,
		error TEXT,
		initiated_by TEXT NOT NULL,
		started_at TIMESTAMPTZ,
		completed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	},

	{
		Description: "create provisioning_jobs node index",
		SQL:         `CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_node ON provisioning_jobs(node_id)`,
	},

	{
		Description: "create provisioning_jobs status index",
		SQL:         `CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_status ON provisioning_jobs(status)`,
	},

	{
		Description: "create provisioning_jobs node+status index",
		SQL:         `CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_node_status ON provisioning_jobs(node_id, status)`,
	},

	{
		Description: "add waf_enabled to cdn_sites",
		SQL:         `ALTER TABLE cdn_sites ADD COLUMN IF NOT EXISTS waf_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
	},

	{
		Description: "add waf_mode to cdn_sites",
		SQL:         `ALTER TABLE cdn_sites ADD COLUMN IF NOT EXISTS waf_mode TEXT NOT NULL DEFAULT 'detect'`,
	},

	{
		Description: "add transfer_allowed_ips to dns_zones",
		SQL:         `ALTER TABLE dns_zones ADD COLUMN IF NOT EXISTS transfer_allowed_ips TEXT NOT NULL DEFAULT '[]'`,
	},
}
