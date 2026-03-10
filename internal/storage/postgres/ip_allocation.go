package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error {
	now := time.Now().UTC()
	ip.CreatedAt = now
	ip.UpdatedAt = now
	if ip.Status == "" {
		ip.Status = domain.IPAllocationPending
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ip_allocations (id, tenant_id, prefix, type, purpose, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ip.ID.String(), ip.TenantID.String(), ip.Prefix,
		string(ip.Type), string(ip.Purpose), string(ip.Status),
		ip.CreatedAt, ip.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: ip allocation already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert ip allocation: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetIPAllocation(ctx context.Context, id domain.ID) (*domain.IPAllocation, error) {
	ip := &domain.IPAllocation{}

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, prefix, type, purpose, status, created_at, updated_at
		 FROM ip_allocations WHERE id = $1`, id.String(),
	).Scan(&ip.ID, &ip.TenantID, &ip.Prefix, &ip.Type, &ip.Purpose,
		&ip.Status, &ip.CreatedAt, &ip.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ip allocation: %w", err)
	}

	return ip, nil
}

func (s *PostgresStore) ListIPAllocations(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.IPAllocation, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ip_allocations WHERE tenant_id = $1`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count ip allocations: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, prefix, type, purpose, status, created_at, updated_at
		 FROM ip_allocations WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list ip allocations: %w", err)
	}
	defer rows.Close()

	var allocations []*domain.IPAllocation
	for rows.Next() {
		ip := &domain.IPAllocation{}
		if err := rows.Scan(&ip.ID, &ip.TenantID, &ip.Prefix, &ip.Type, &ip.Purpose,
			&ip.Status, &ip.CreatedAt, &ip.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ip allocation: %w", err)
		}
		allocations = append(allocations, ip)
	}
	return allocations, total, rows.Err()
}

func (s *PostgresStore) UpdateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error {
	ip.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE ip_allocations SET prefix = $1, type = $2, purpose = $3, status = $4, updated_at = $5
		 WHERE id = $6`,
		ip.Prefix, string(ip.Type), string(ip.Purpose), string(ip.Status),
		ip.UpdatedAt, ip.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update ip allocation: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteIPAllocation(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM ip_allocations WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete ip allocation: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
