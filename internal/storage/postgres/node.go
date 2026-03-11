package postgres

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

func (s *PostgresStore) CreateNode(ctx context.Context, n *domain.Node) error {
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
		`INSERT INTO nodes (id, tenant_id, name, hostname, public_ip, wireguard_ip, wireguard_ipv6, status, region, provider, ssh_port, ssh_user, ssh_key_id, binary_version, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		n.ID.String(), nullIDString(n.TenantID), n.Name, n.Hostname, n.PublicIP,
		nullString(&n.WireGuardIP), n.WireGuardIPv6, string(n.Status), nullString(&n.Region),
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
			`INSERT INTO node_capabilities (node_id, capability) VALUES ($1, $2)`,
			n.ID.String(), string(cap),
		)
		if err != nil {
			return fmt.Errorf("insert capability: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetNode(ctx context.Context, id domain.ID) (*domain.Node, error) {
	n := &domain.Node{}
	var tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata sql.NullString
	var lastHeartbeat, lastConfigSync sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, hostname, public_ip, wireguard_ip, wireguard_ipv6, status, region, provider,
		        ssh_port, ssh_user, ssh_key_id, binary_version, last_heartbeat, last_config_sync, metadata, created_at, updated_at
		 FROM nodes WHERE id = $1`, id.String(),
	).Scan(&n.ID, &tenantID, &n.Name, &n.Hostname, &n.PublicIP, &wgIP, &n.WireGuardIPv6, &n.Status,
		&region, &provider, &n.SSHPort, &n.SSHUser, &sshKeyID, &binaryVersion,
		&lastHeartbeat, &lastConfigSync, &metadata, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	applyNullableNodeFields(n, tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata, lastHeartbeat, lastConfigSync)

	// Load capabilities.
	caps, err := s.loadNodeCapabilities(ctx, id)
	if err != nil {
		return nil, err
	}
	n.Capabilities = caps

	return n, nil
}

func (s *PostgresStore) ListNodes(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.Node, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if tenantID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes WHERE tenant_id = $1`, tenantID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes`,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count nodes: %w", countErr)
	}

	const selectCols = `id, tenant_id, name, hostname, public_ip, wireguard_ip, wireguard_ipv6, status, region, provider,
			ssh_port, ssh_user, ssh_key_id, binary_version, last_heartbeat, last_config_sync, metadata, created_at, updated_at`

	var rows *sql.Rows
	var err error

	// Cursor-based pagination: use keyset WHERE (created_at, id) < ($N, $N) for DESC ordering.
	if cursor, ok := storage.DecodeCursor(params.Cursor); ok {
		if tenantID != nil {
			rows, err = s.db.QueryContext(ctx,
				`SELECT `+selectCols+`
				 FROM nodes WHERE tenant_id = $1 AND (created_at < $2 OR (created_at = $3 AND id < $4))
				 ORDER BY created_at DESC, id DESC LIMIT $5`,
				tenantID.String(), cursor.CreatedAt, cursor.CreatedAt, cursor.ID, params.Limit,
			)
		} else {
			rows, err = s.db.QueryContext(ctx,
				`SELECT `+selectCols+`
				 FROM nodes WHERE (created_at < $1 OR (created_at = $2 AND id < $3))
				 ORDER BY created_at DESC, id DESC LIMIT $4`,
				cursor.CreatedAt, cursor.CreatedAt, cursor.ID, params.Limit,
			)
		}
	} else if tenantID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+selectCols+`
			 FROM nodes WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			tenantID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+selectCols+`
			 FROM nodes ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
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
		var lastHeartbeat, lastConfigSync sql.NullTime

		if err := rows.Scan(&n.ID, &tid, &n.Name, &n.Hostname, &n.PublicIP, &wgIP, &n.WireGuardIPv6, &n.Status,
			&region, &provider, &n.SSHPort, &n.SSHUser, &sshKeyID, &binaryVersion,
			&lastHeartbeat, &lastConfigSync, &metadata, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan node row: %w", err)
		}

		applyNullableNodeFields(n, tid, wgIP, region, provider, sshKeyID, binaryVersion, metadata, lastHeartbeat, lastConfigSync)
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

func (s *PostgresStore) UpdateNode(ctx context.Context, n *domain.Node) error {
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
		`UPDATE nodes SET tenant_id = $1, name = $2, hostname = $3, public_ip = $4, wireguard_ip = $5,
		 wireguard_ipv6 = $6, status = $7, region = $8, provider = $9, ssh_port = $10, ssh_user = $11, ssh_key_id = $12,
		 binary_version = $13, metadata = $14, updated_at = $15
		 WHERE id = $16`,
		nullIDString(n.TenantID), n.Name, n.Hostname, n.PublicIP,
		nullString(&n.WireGuardIP), n.WireGuardIPv6, string(n.Status), nullString(&n.Region),
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
	_, err = tx.ExecContext(ctx, `DELETE FROM node_capabilities WHERE node_id = $1`, n.ID.String())
	if err != nil {
		return fmt.Errorf("delete old capabilities: %w", err)
	}
	for _, cap := range n.Capabilities {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO node_capabilities (node_id, capability) VALUES ($1, $2)`,
			n.ID.String(), string(cap),
		)
		if err != nil {
			return fmt.Errorf("insert capability: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) DeleteNode(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateNodeHeartbeat(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET last_heartbeat = $1, status = $2, updated_at = $3 WHERE id = $4`,
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

func (s *PostgresStore) UpdateNodeConfigSync(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET last_config_sync = $1, updated_at = $2 WHERE id = $3`,
		now, now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("update node config sync: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadNodeCapabilities returns capabilities for a given node.
func (s *PostgresStore) loadNodeCapabilities(ctx context.Context, nodeID domain.ID) ([]domain.NodeCapability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT capability FROM node_capabilities WHERE node_id = $1`, nodeID.String(),
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
func applyNullableNodeFields(n *domain.Node, tenantID, wgIP, region, provider, sshKeyID, binaryVersion, metadata sql.NullString, lastHeartbeat, lastConfigSync sql.NullTime) {
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
	if lastConfigSync.Valid {
		n.LastConfigSync = &lastConfigSync.Time
	}
}
