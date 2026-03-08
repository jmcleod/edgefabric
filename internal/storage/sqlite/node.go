package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateNode(ctx context.Context, n *domain.Node) error {
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now
	if n.Status == "" {
		n.Status = domain.NodeStatusPending
	}
	if n.SSHPort == 0 {
		n.SSHPort = 22
	}

	metadata := sql.NullString{}
	if n.Metadata != nil {
		metadata = sql.NullString{String: string(n.Metadata), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO nodes (id, tenant_id, name, hostname, public_ip, wireguard_ip, status, region, provider, ssh_port, ssh_user, ssh_key_id, binary_version, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID.String(), nullIDString(n.TenantID), n.Name, n.Hostname, n.PublicIP,
		nullString(&n.WireGuardIP), string(n.Status), nullString(&n.Region),
		nullString(&n.Provider), n.SSHPort, n.SSHUser, nullIDString(n.SSHKeyID),
		nullString(&n.BinaryVersion), metadata, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: node already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert node: %w", err)
	}

	// Insert capabilities.
	for _, cap := range n.Capabilities {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO node_capabilities (node_id, capability) VALUES (?, ?)`,
			n.ID.String(), string(cap),
		)
		if err != nil {
			return fmt.Errorf("insert capability: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetNode(ctx context.Context, id domain.ID) (*domain.Node, error) {
	n := &domain.Node{}
	var tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata sql.NullString
	var lastHeartbeat sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, hostname, public_ip, wireguard_ip, status, region, provider,
		        ssh_port, ssh_user, ssh_key_id, binary_version, last_heartbeat, metadata, created_at, updated_at
		 FROM nodes WHERE id = ?`, id.String(),
	).Scan(&n.ID, &tenantID, &n.Name, &n.Hostname, &n.PublicIP, &wgIP, &n.Status,
		&region, &provider, &n.SSHPort, &n.SSHUser, &sshKeyID, &binaryVersion,
		&lastHeartbeat, &metadata, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	applyNullableNodeFields(n, tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata, lastHeartbeat)

	// Load capabilities.
	caps, err := s.loadNodeCapabilities(ctx, id)
	if err != nil {
		return nil, err
	}
	n.Capabilities = caps

	return n, nil
}

func (s *SQLiteStore) ListNodes(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Node, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if tenantID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes WHERE tenant_id = ?`, tenantID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes`,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count nodes: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if tenantID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, name, hostname, public_ip, wireguard_ip, status, region, provider,
			        ssh_port, ssh_user, ssh_key_id, binary_version, last_heartbeat, metadata, created_at, updated_at
			 FROM nodes WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			tenantID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, name, hostname, public_ip, wireguard_ip, status, region, provider,
			        ssh_port, ssh_user, ssh_key_id, binary_version, last_heartbeat, metadata, created_at, updated_at
			 FROM nodes ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			params.Limit, params.Offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*domain.Node
	for rows.Next() {
		n := &domain.Node{}
		var tid, wgIP, region, provider, sshKeyID, binaryVersion, metadata sql.NullString
		var lastHeartbeat sql.NullTime

		if err := rows.Scan(&n.ID, &tid, &n.Name, &n.Hostname, &n.PublicIP, &wgIP, &n.Status,
			&region, &provider, &n.SSHPort, &n.SSHUser, &sshKeyID, &binaryVersion,
			&lastHeartbeat, &metadata, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan node row: %w", err)
		}

		applyNullableNodeFields(n, tid, wgIP, region, provider, sshKeyID, binaryVersion, metadata, lastHeartbeat)
		nodes = append(nodes, n)
	}

	// Batch-load capabilities for all returned nodes.
	for _, n := range nodes {
		caps, err := s.loadNodeCapabilities(ctx, n.ID)
		if err != nil {
			return nil, 0, err
		}
		n.Capabilities = caps
	}

	return nodes, total, rows.Err()
}

func (s *SQLiteStore) UpdateNode(ctx context.Context, n *domain.Node) error {
	n.UpdatedAt = time.Now().UTC()

	metadata := sql.NullString{}
	if n.Metadata != nil {
		metadata = sql.NullString{String: string(n.Metadata), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE nodes SET tenant_id = ?, name = ?, hostname = ?, public_ip = ?, wireguard_ip = ?,
		 status = ?, region = ?, provider = ?, ssh_port = ?, ssh_user = ?, ssh_key_id = ?,
		 binary_version = ?, metadata = ?, updated_at = ?
		 WHERE id = ?`,
		nullIDString(n.TenantID), n.Name, n.Hostname, n.PublicIP,
		nullString(&n.WireGuardIP), string(n.Status), nullString(&n.Region),
		nullString(&n.Provider), n.SSHPort, n.SSHUser, nullIDString(n.SSHKeyID),
		nullString(&n.BinaryVersion), metadata, n.UpdatedAt, n.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	// Replace capabilities.
	_, err = tx.ExecContext(ctx, `DELETE FROM node_capabilities WHERE node_id = ?`, n.ID.String())
	if err != nil {
		return fmt.Errorf("delete old capabilities: %w", err)
	}
	for _, cap := range n.Capabilities {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO node_capabilities (node_id, capability) VALUES (?, ?)`,
			n.ID.String(), string(cap),
		)
		if err != nil {
			return fmt.Errorf("insert capability: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) DeleteNode(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) UpdateNodeHeartbeat(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET last_heartbeat = ?, status = ?, updated_at = ? WHERE id = ?`,
		now, string(domain.NodeStatusOnline), now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadNodeCapabilities returns capabilities for a given node.
func (s *SQLiteStore) loadNodeCapabilities(ctx context.Context, nodeID domain.ID) ([]domain.NodeCapability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT capability FROM node_capabilities WHERE node_id = ?`, nodeID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("load capabilities: %w", err)
	}
	defer rows.Close()

	var caps []domain.NodeCapability
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan capability: %w", err)
		}
		caps = append(caps, domain.NodeCapability(c))
	}
	return caps, rows.Err()
}

// applyNullableNodeFields applies nullable SQL scan results to a Node.
func applyNullableNodeFields(n *domain.Node, tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata sql.NullString, lastHeartbeat sql.NullTime) {
	if tenantID.Valid {
		id, err := uuid.Parse(tenantID.String)
		if err == nil {
			n.TenantID = &id
		}
	}
	if wgIP.Valid {
		n.WireGuardIP = wgIP.String
	}
	if region.Valid {
		n.Region = region.String
	}
	if provider.Valid {
		n.Provider = provider.String
	}
	if sshKeyID.Valid {
		id, err := uuid.Parse(sshKeyID.String)
		if err == nil {
			n.SSHKeyID = &id
		}
	}
	if binaryVersion.Valid {
		n.BinaryVersion = binaryVersion.String
	}
	if metadata.Valid {
		n.Metadata = json.RawMessage(metadata.String)
	}
	if lastHeartbeat.Valid {
		n.LastHeartbeat = &lastHeartbeat.Time
	}
}

// nullString converts an empty string to NULL, non-empty to a valid NullString.
// This overloads the package-level helper when we need empty-string-as-null behavior.
func nullStringEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

