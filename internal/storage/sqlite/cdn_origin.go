package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	now := time.Now().UTC()
	o.CreatedAt = now
	o.UpdatedAt = now
	if o.Status == "" {
		o.Status = domain.CDNOriginUnknown
	}
	if o.Weight == 0 {
		o.Weight = 1
	}
	if o.Scheme == "" {
		o.Scheme = domain.CDNOriginHTTPS
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cdn_origins (id, site_id, address, scheme, weight, health_check_path, health_check_interval, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID.String(), o.SiteID.String(), o.Address,
		string(o.Scheme), o.Weight, o.HealthCheckPath, o.HealthCheckInterval,
		string(o.Status), o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: cdn origin already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert cdn origin: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetCDNOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error) {
	o := &domain.CDNOrigin{}

	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, address, scheme, weight, health_check_path, health_check_interval,
		        status, created_at, updated_at
		 FROM cdn_origins WHERE id = ?`, id.String(),
	).Scan(&o.ID, &o.SiteID, &o.Address,
		&o.Scheme, &o.Weight, &o.HealthCheckPath, &o.HealthCheckInterval,
		&o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cdn origin: %w", err)
	}

	return o, nil
}

func (s *SQLiteStore) ListCDNOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdn_origins WHERE site_id = ?`, siteID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cdn origins: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, address, scheme, weight, health_check_path, health_check_interval,
		        status, created_at, updated_at
		 FROM cdn_origins WHERE site_id = ? ORDER BY address ASC LIMIT ? OFFSET ?`,
		siteID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list cdn origins: %w", err)
	}
	defer rows.Close()

	var origins []*domain.CDNOrigin
	for rows.Next() {
		o := &domain.CDNOrigin{}
		if err := rows.Scan(&o.ID, &o.SiteID, &o.Address,
			&o.Scheme, &o.Weight, &o.HealthCheckPath, &o.HealthCheckInterval,
			&o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan cdn origin: %w", err)
		}
		origins = append(origins, o)
	}
	return origins, total, rows.Err()
}

func (s *SQLiteStore) UpdateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	o.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE cdn_origins SET address = ?, scheme = ?, weight = ?, health_check_path = ?,
		 health_check_interval = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		o.Address, string(o.Scheme), o.Weight, o.HealthCheckPath,
		o.HealthCheckInterval, string(o.Status), o.UpdatedAt, o.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update cdn origin: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteCDNOrigin(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cdn_origins WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete cdn origin: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
