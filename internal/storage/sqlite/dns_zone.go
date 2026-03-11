package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateDNSZone(ctx context.Context, z *domain.DNSZone) error {
	now := time.Now().UTC()
	z.CreatedAt = now
	z.UpdatedAt = now
	if z.Serial == 0 {
		z.Serial = 1
	}
	if z.TTL == 0 {
		z.TTL = 3600
	}
	if z.Status == "" {
		z.Status = domain.DNSZoneActive
	}

	var nodeGroupID *string
	if z.NodeGroupID != nil {
		s := z.NodeGroupID.String()
		nodeGroupID = &s
	}

	transferIPs := "[]"
	if len(z.TransferAllowedIPs) > 0 {
		b, _ := json.Marshal(z.TransferAllowedIPs)
		transferIPs = string(b)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dns_zones (id, tenant_id, name, status, serial, ttl, node_group_id, transfer_allowed_ips, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		z.ID.String(), z.TenantID.String(), z.Name,
		string(z.Status), z.Serial, z.TTL, nodeGroupID, transferIPs,
		z.CreatedAt, z.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: dns zone already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert dns zone: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetDNSZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error) {
	z := &domain.DNSZone{}
	var transferIPs string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, status, serial, ttl, node_group_id, transfer_allowed_ips, created_at, updated_at
		 FROM dns_zones WHERE id = ?`, id.String(),
	).Scan(&z.ID, &z.TenantID, &z.Name, &z.Status, &z.Serial,
		&z.TTL, &z.NodeGroupID, &transferIPs, &z.CreatedAt, &z.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get dns zone: %w", err)
	}

	if transferIPs != "" {
		_ = json.Unmarshal([]byte(transferIPs), &z.TransferAllowedIPs)
	}

	return z, nil
}

func (s *SQLiteStore) ListDNSZones(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.DNSZone, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dns_zones WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count dns zones: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, status, serial, ttl, node_group_id, transfer_allowed_ips, created_at, updated_at
		 FROM dns_zones WHERE tenant_id = ? ORDER BY name ASC LIMIT ? OFFSET ?`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list dns zones: %w", err)
	}
	defer rows.Close()

	var zones []*domain.DNSZone
	for rows.Next() {
		z := &domain.DNSZone{}
		var transferIPs string
		if err := rows.Scan(&z.ID, &z.TenantID, &z.Name, &z.Status, &z.Serial,
			&z.TTL, &z.NodeGroupID, &transferIPs, &z.CreatedAt, &z.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan dns zone: %w", err)
		}
		if transferIPs != "" {
			_ = json.Unmarshal([]byte(transferIPs), &z.TransferAllowedIPs)
		}
		zones = append(zones, z)
	}
	return zones, total, rows.Err()
}

func (s *SQLiteStore) UpdateDNSZone(ctx context.Context, z *domain.DNSZone) error {
	z.UpdatedAt = time.Now().UTC()

	var nodeGroupID *string
	if z.NodeGroupID != nil {
		s := z.NodeGroupID.String()
		nodeGroupID = &s
	}

	transferIPs := "[]"
	if len(z.TransferAllowedIPs) > 0 {
		b, _ := json.Marshal(z.TransferAllowedIPs)
		transferIPs = string(b)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_zones SET name = ?, status = ?, ttl = ?, node_group_id = ?, transfer_allowed_ips = ?, updated_at = ?
		 WHERE id = ?`,
		z.Name, string(z.Status), z.TTL, nodeGroupID, transferIPs,
		z.UpdatedAt, z.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update dns zone: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteDNSZone(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM dns_zones WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete dns zone: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) IncrementDNSZoneSerial(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_zones SET serial = serial + 1, updated_at = ? WHERE id = ?`,
		now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("increment dns zone serial: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
