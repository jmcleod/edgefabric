package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateDNSZone(ctx context.Context, z *domain.DNSZone) error {
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

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dns_zones (id, tenant_id, name, status, serial, ttl, node_group_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		z.ID.String(), z.TenantID.String(), z.Name,
		string(z.Status), z.Serial, z.TTL, nullIDString(z.NodeGroupID),
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

func (s *PostgresStore) GetDNSZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error) {
	z := &domain.DNSZone{}
	var nodeGroupID sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, status, serial, ttl, node_group_id, created_at, updated_at
		 FROM dns_zones WHERE id = $1`, id.String(),
	).Scan(&z.ID, &z.TenantID, &z.Name, &z.Status, &z.Serial,
		&z.TTL, &nodeGroupID, &z.CreatedAt, &z.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get dns zone: %w", err)
	}

	if nodeGroupID.Valid {
		parsed, err := domain.ParseID(nodeGroupID.String)
		if err != nil {
			return nil, fmt.Errorf("parse node_group_id: %w", err)
		}
		z.NodeGroupID = &parsed
	}

	return z, nil
}

func (s *PostgresStore) ListDNSZones(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.DNSZone, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dns_zones WHERE tenant_id = $1`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count dns zones: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, status, serial, ttl, node_group_id, created_at, updated_at
		 FROM dns_zones WHERE tenant_id = $1 ORDER BY name ASC LIMIT $2 OFFSET $3`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list dns zones: %w", err)
	}
	defer rows.Close()

	var zones []*domain.DNSZone
	for rows.Next() {
		z := &domain.DNSZone{}
		var nodeGroupID sql.NullString
		if err := rows.Scan(&z.ID, &z.TenantID, &z.Name, &z.Status, &z.Serial,
			&z.TTL, &nodeGroupID, &z.CreatedAt, &z.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan dns zone: %w", err)
		}
		if nodeGroupID.Valid {
			parsed, err := domain.ParseID(nodeGroupID.String)
			if err != nil {
				return nil, 0, fmt.Errorf("parse node_group_id: %w", err)
			}
			z.NodeGroupID = &parsed
		}
		zones = append(zones, z)
	}
	return zones, total, rows.Err()
}

func (s *PostgresStore) UpdateDNSZone(ctx context.Context, z *domain.DNSZone) error {
	z.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_zones SET name = $1, status = $2, ttl = $3, node_group_id = $4, updated_at = $5
		 WHERE id = $6`,
		z.Name, string(z.Status), z.TTL, nullIDString(z.NodeGroupID),
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

func (s *PostgresStore) DeleteDNSZone(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM dns_zones WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete dns zone: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) IncrementDNSZoneSerial(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_zones SET serial = serial + 1, updated_at = $1 WHERE id = $2`,
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
