package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
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
		`INSERT INTO cdn_origins (id, site_id, address, scheme, weight, health_check_path,
		 health_check_interval, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		o.ID.String(), o.SiteID.String(), o.Address, string(o.Scheme), o.Weight,
		nullStringEmpty(o.HealthCheckPath), nullIntPtr(o.HealthCheckInterval),
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

func (s *PostgresStore) GetCDNOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error) {
	o := &domain.CDNOrigin{}
	var healthCheckPath sql.NullString
	var healthCheckInterval sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, address, scheme, weight, health_check_path, health_check_interval,
		        status, created_at, updated_at
		 FROM cdn_origins WHERE id = $1`, id.String(),
	).Scan(&o.ID, &o.SiteID, &o.Address, &o.Scheme, &o.Weight,
		&healthCheckPath, &healthCheckInterval, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cdn origin: %w", err)
	}

	if healthCheckPath.Valid {
		o.HealthCheckPath = healthCheckPath.String
	}
	if healthCheckInterval.Valid {
		v := int(healthCheckInterval.Int64)
		o.HealthCheckInterval = &v
	}

	return o, nil
}

func (s *PostgresStore) ListCDNOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdn_origins WHERE site_id = $1`, siteID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cdn origins: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, address, scheme, weight, health_check_path, health_check_interval,
		        status, created_at, updated_at
		 FROM cdn_origins WHERE site_id = $1 ORDER BY address ASC LIMIT $2 OFFSET $3`,
		siteID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list cdn origins: %w", err)
	}
	defer rows.Close()

	var origins []*domain.CDNOrigin
	for rows.Next() {
		o := &domain.CDNOrigin{}
		var healthCheckPath sql.NullString
		var healthCheckInterval sql.NullInt64

		if err := rows.Scan(&o.ID, &o.SiteID, &o.Address, &o.Scheme, &o.Weight,
			&healthCheckPath, &healthCheckInterval, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan cdn origin: %w", err)
		}

		if healthCheckPath.Valid {
			o.HealthCheckPath = healthCheckPath.String
		}
		if healthCheckInterval.Valid {
			v := int(healthCheckInterval.Int64)
			o.HealthCheckInterval = &v
		}

		origins = append(origins, o)
	}
	return origins, total, rows.Err()
}

func (s *PostgresStore) UpdateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	o.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE cdn_origins SET address = $1, scheme = $2, weight = $3, health_check_path = $4,
		 health_check_interval = $5, status = $6, updated_at = $7
		 WHERE id = $8`,
		o.Address, string(o.Scheme), o.Weight,
		nullStringEmpty(o.HealthCheckPath), nullIntPtr(o.HealthCheckInterval),
		string(o.Status), o.UpdatedAt, o.ID.String(),
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

func (s *PostgresStore) DeleteCDNOrigin(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cdn_origins WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete cdn origin: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
