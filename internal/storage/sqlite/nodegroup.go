package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateNodeGroup(ctx context.Context, g *domain.NodeGroup) error {
	now := time.Now().UTC()
	g.CreatedAt = now
	g.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_groups (id, tenant_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		g.ID.String(), g.TenantID.String(), g.Name,
		nullStringEmpty(g.Description), g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: node group already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert node group: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetNodeGroup(ctx context.Context, id domain.ID) (*domain.NodeGroup, error) {
	g := &domain.NodeGroup{}
	var desc sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, description, created_at, updated_at
		 FROM node_groups WHERE id = ?`, id.String(),
	).Scan(&g.ID, &g.TenantID, &g.Name, &desc, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get node group: %w", err)
	}
	if desc.Valid {
		g.Description = desc.String
	}
	return g, nil
}

func (s *SQLiteStore) ListNodeGroups(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.NodeGroup, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM node_groups WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count node groups: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, description, created_at, updated_at
		 FROM node_groups WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list node groups: %w", err)
	}
	defer rows.Close()

	var groups []*domain.NodeGroup
	for rows.Next() {
		g := &domain.NodeGroup{}
		var desc sql.NullString
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &desc, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan node group: %w", err)
		}
		if desc.Valid {
			g.Description = desc.String
		}
		groups = append(groups, g)
	}
	return groups, total, rows.Err()
}

func (s *SQLiteStore) UpdateNodeGroup(ctx context.Context, g *domain.NodeGroup) error {
	g.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE node_groups SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		g.Name, nullStringEmpty(g.Description), g.UpdatedAt, g.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update node group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteNodeGroup(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM node_groups WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete node group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) AddNodeToGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_group_memberships (node_group_id, node_id) VALUES (?, ?)`,
		groupID.String(), nodeID.String(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: node already in group", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("add node to group: %w", err)
	}
	return nil
}

func (s *SQLiteStore) RemoveNodeFromGroup(ctx context.Context, groupID, nodeID domain.ID) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM node_group_memberships WHERE node_group_id = ? AND node_id = ?`,
		groupID.String(), nodeID.String(),
	)
	if err != nil {
		return fmt.Errorf("remove node from group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListGroupNodes(ctx context.Context, groupID domain.ID) ([]*domain.Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT n.id, n.tenant_id, n.name, n.hostname, n.public_ip, n.wireguard_ip, n.status,
		        n.region, n.provider, n.ssh_port, n.ssh_user, n.ssh_key_id, n.binary_version,
		        n.last_heartbeat, n.metadata, n.created_at, n.updated_at
		 FROM nodes n
		 JOIN node_group_memberships m ON m.node_id = n.id
		 WHERE m.node_group_id = ?
		 ORDER BY n.name`, groupID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list group nodes: %w", err)
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
			return nil, fmt.Errorf("scan group node: %w", err)
		}
		applyNullableNodeFields(n, tid, wgIP, region, provider, sshKeyID, binaryVersion, metadata, lastHeartbeat)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *SQLiteStore) ListNodeGroups_ByNode(ctx context.Context, nodeID domain.ID) ([]*domain.NodeGroup, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.tenant_id, g.name, g.description, g.created_at, g.updated_at
		 FROM node_groups g
		 JOIN node_group_memberships m ON m.node_group_id = g.id
		 WHERE m.node_id = ?
		 ORDER BY g.name`, nodeID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list node groups by node: %w", err)
	}
	defer rows.Close()

	var groups []*domain.NodeGroup
	for rows.Next() {
		g := &domain.NodeGroup{}
		var desc sql.NullString
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &desc, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node group: %w", err)
		}
		if desc.Valid {
			g.Description = desc.String
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
