// Package sqlite implements the storage.Store interface using SQLite.
//
// This is the default storage driver for single-instance EdgeFabric deployments.
// It uses modernc.org/sqlite (pure Go, no CGO required for basic operation)
// or mattn/go-sqlite3 (CGO, better performance).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Ensure SQLiteStore implements storage.Store.
var _ storage.Store = (*SQLiteStore)(nil)

// SQLiteStore implements storage.Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// New creates a new SQLite store.
func New(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite performance tuning.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA cache_size=-64000", // 64MB
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("execute pragma %q: %w", p, err)
		}
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Ping checks database connectivity.
func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Migrate runs database migrations.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	for _, stmt := range migrations {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration error: %w\nStatement: %s", err, stmt)
		}
	}
	return nil
}

// migrations contains the SQL DDL for the SQLite schema.
// In production, this would use a proper migration framework.
// For now, these are idempotent CREATE IF NOT EXISTS statements.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		slug TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL DEFAULT 'active',
		settings TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT REFERENCES tenants(id),
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		totp_secret TEXT,
		totp_enabled INTEGER NOT NULL DEFAULT 0,
		role TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		last_login_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		user_id TEXT NOT NULL REFERENCES users(id),
		name TEXT NOT NULL,
		key_hash TEXT NOT NULL,
		key_prefix TEXT NOT NULL,
		role TEXT NOT NULL,
		expires_at DATETIME,
		last_used_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		last_rotated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS nodes (
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
		last_heartbeat DATETIME,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS node_capabilities (
		node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		capability TEXT NOT NULL,
		PRIMARY KEY (node_id, capability)
	)`,

	`CREATE TABLE IF NOT EXISTS node_groups (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS node_group_memberships (
		node_group_id TEXT NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
		node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		PRIMARY KEY (node_group_id, node_id)
	)`,

	`CREATE TABLE IF NOT EXISTS gateways (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		public_ip TEXT,
		wireguard_ip TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		enrollment_token TEXT,
		last_heartbeat DATETIME,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS wireguard_peers (
		id TEXT PRIMARY KEY,
		owner_type TEXT NOT NULL,
		owner_id TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT NOT NULL,
		preshared_key TEXT,
		endpoint TEXT,
		last_rotated_at DATETIME NOT NULL DEFAULT (datetime('now')),
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS wireguard_peer_allowed_ips (
		peer_id TEXT NOT NULL REFERENCES wireguard_peers(id) ON DELETE CASCADE,
		allowed_ip TEXT NOT NULL,
		PRIMARY KEY (peer_id, allowed_ip)
	)`,

	`CREATE TABLE IF NOT EXISTS ip_allocations (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		prefix TEXT NOT NULL,
		type TEXT NOT NULL,
		purpose TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS bgp_sessions (
		id TEXT PRIMARY KEY,
		node_id TEXT NOT NULL REFERENCES nodes(id),
		peer_asn INTEGER NOT NULL,
		peer_address TEXT NOT NULL,
		local_asn INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'configured',
		import_policy TEXT,
		export_policy TEXT,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS bgp_announced_prefixes (
		session_id TEXT NOT NULL REFERENCES bgp_sessions(id) ON DELETE CASCADE,
		prefix TEXT NOT NULL,
		PRIMARY KEY (session_id, prefix)
	)`,

	`CREATE TABLE IF NOT EXISTS dns_zones (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		serial INTEGER NOT NULL DEFAULT 1,
		ttl INTEGER NOT NULL DEFAULT 3600,
		node_group_id TEXT REFERENCES node_groups(id),
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS dns_records (
		id TEXT PRIMARY KEY,
		zone_id TEXT NOT NULL REFERENCES dns_zones(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		ttl INTEGER,
		priority INTEGER,
		weight INTEGER,
		port INTEGER,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS cdn_sites (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		name TEXT NOT NULL,
		tls_mode TEXT NOT NULL DEFAULT 'auto',
		tls_cert_id TEXT,
		cache_enabled INTEGER NOT NULL DEFAULT 1,
		cache_ttl INTEGER NOT NULL DEFAULT 3600,
		compression_enabled INTEGER NOT NULL DEFAULT 1,
		rate_limit_rps INTEGER,
		node_group_id TEXT REFERENCES node_groups(id),
		header_rules TEXT,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS cdn_site_domains (
		site_id TEXT NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
		domain TEXT NOT NULL,
		PRIMARY KEY (site_id, domain)
	)`,

	`CREATE TABLE IF NOT EXISTS cdn_origins (
		id TEXT PRIMARY KEY,
		site_id TEXT NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
		address TEXT NOT NULL,
		scheme TEXT NOT NULL DEFAULT 'https',
		weight INTEGER NOT NULL DEFAULT 1,
		health_check_path TEXT,
		health_check_interval INTEGER,
		status TEXT NOT NULL DEFAULT 'unknown',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS routes (
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
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS tls_certificates (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		cert_pem TEXT NOT NULL,
		key_pem TEXT NOT NULL,
		issuer TEXT,
		expires_at DATETIME NOT NULL,
		auto_renew INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS tls_certificate_domains (
		cert_id TEXT NOT NULL REFERENCES tls_certificates(id) ON DELETE CASCADE,
		domain TEXT NOT NULL,
		PRIMARY KEY (cert_id, domain)
	)`,

	`CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		tenant_id TEXT,
		user_id TEXT,
		api_key_id TEXT,
		action TEXT NOT NULL,
		resource TEXT NOT NULL,
		details TEXT,
		source_ip TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_tenant ON audit_events(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp)`,

	`CREATE TABLE IF NOT EXISTS enrollment_tokens (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id),
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		used_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`,
}

// --- Stub implementations for all Store interfaces ---
// These will be implemented in subsequent milestones.
// For now they satisfy the interface contract so the project compiles.

func (s *SQLiteStore) CreateTenant(ctx context.Context, t *domain.Tenant) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetTenant(ctx context.Context, id domain.ID) (*domain.Tenant, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListTenants(ctx context.Context, params storage.ListParams) ([]*domain.Tenant, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateTenant(ctx context.Context, t *domain.Tenant) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteTenant(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateUser(ctx context.Context, u *domain.User) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetUser(ctx context.Context, id domain.ID) (*domain.User, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListUsers(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.User, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateUser(ctx context.Context, u *domain.User) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteUser(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateAPIKey(ctx context.Context, k *domain.APIKey) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetAPIKey(ctx context.Context, id domain.ID) (*domain.APIKey, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListAPIKeys(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.APIKey, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteAPIKey(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateAPIKeyLastUsed(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateNode(ctx context.Context, n *domain.Node) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetNode(ctx context.Context, id domain.ID) (*domain.Node, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListNodes(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Node, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateNode(ctx context.Context, n *domain.Node) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteNode(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateNodeHeartbeat(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateNodeGroup(ctx context.Context, g *domain.NodeGroup) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetNodeGroup(ctx context.Context, id domain.ID) (*domain.NodeGroup, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListNodeGroups(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.NodeGroup, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateNodeGroup(ctx context.Context, g *domain.NodeGroup) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteNodeGroup(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) AddNodeToGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) RemoveNodeFromGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListGroupNodes(ctx context.Context, groupID domain.ID) ([]*domain.Node, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListNodeGroups_ByNode(ctx context.Context, nodeID domain.ID) ([]*domain.NodeGroup, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateGateway(ctx context.Context, g *domain.Gateway) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListGateways(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Gateway, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateGateway(ctx context.Context, g *domain.Gateway) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteGateway(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateGatewayHeartbeat(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetWireGuardPeer(ctx context.Context, id domain.ID) (*domain.WireGuardPeer, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetWireGuardPeerByOwner(ctx context.Context, ownerType domain.PeerOwnerType, ownerID domain.ID) (*domain.WireGuardPeer, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListWireGuardPeers(ctx context.Context, params storage.ListParams) ([]*domain.WireGuardPeer, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteWireGuardPeer(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetIPAllocation(ctx context.Context, id domain.ID) (*domain.IPAllocation, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListIPAllocations(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.IPAllocation, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteIPAllocation(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateBGPSession(ctx context.Context, sess *domain.BGPSession) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetBGPSession(ctx context.Context, id domain.ID) (*domain.BGPSession, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListBGPSessions(ctx context.Context, nodeID domain.ID, params storage.ListParams) ([]*domain.BGPSession, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateBGPSession(ctx context.Context, sess *domain.BGPSession) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteBGPSession(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateDNSZone(ctx context.Context, z *domain.DNSZone) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetDNSZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListDNSZones(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.DNSZone, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateDNSZone(ctx context.Context, z *domain.DNSZone) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteDNSZone(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) IncrementDNSZoneSerial(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetDNSRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListDNSRecords(ctx context.Context, zoneID domain.ID, params storage.ListParams) ([]*domain.DNSRecord, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteDNSRecord(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetCDNSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListCDNSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteCDNSite(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetCDNOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListCDNOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteCDNOrigin(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateRoute(ctx context.Context, r *domain.Route) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) UpdateRoute(ctx context.Context, r *domain.Route) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteRoute(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateSSHKey(ctx context.Context, k *domain.SSHKey) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetSSHKey(ctx context.Context, id domain.ID) (*domain.SSHKey, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListSSHKeys(ctx context.Context, params storage.ListParams) ([]*domain.SSHKey, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteSSHKey(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateTLSCertificate(ctx context.Context, c *domain.TLSCertificate) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetTLSCertificate(ctx context.Context, id domain.ID) (*domain.TLSCertificate, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListTLSCertificates(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.TLSCertificate, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) DeleteTLSCertificate(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateAuditEvent(ctx context.Context, e *domain.AuditEvent) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) ListAuditEvents(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.AuditEvent, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) CreateEnrollmentToken(ctx context.Context, t *domain.EnrollmentToken) error {
	return fmt.Errorf("not implemented")
}
func (s *SQLiteStore) GetEnrollmentToken(ctx context.Context, token string) (*domain.EnrollmentToken, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *SQLiteStore) MarkEnrollmentTokenUsed(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("not implemented")
}
