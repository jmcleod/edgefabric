package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateTenant(ctx context.Context, t *domain.Tenant) error {
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = domain.TenantStatusActive
	}

	settings := sql.NullString{}
	if t.Settings != nil {
		settings = sql.NullString{String: string(t.Settings), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, slug, status, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.ID.String(), t.Name, t.Slug, string(t.Status), settings,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: tenant name or slug already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert tenant: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetTenant(ctx context.Context, id domain.ID) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM tenants WHERE id = $1`, id.String(),
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &settings, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	if settings.Valid {
		t.Settings = json.RawMessage(settings.String)
	}
	return t, nil
}

func (s *PostgresStore) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM tenants WHERE slug = $1`, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &settings, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}
	if settings.Valid {
		t.Settings = json.RawMessage(settings.String)
	}
	return t, nil
}

func (s *PostgresStore) ListTenants(ctx context.Context, params storage.ListParams) ([]*domain.Tenant, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tenants: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*domain.Tenant
	for rows.Next() {
		t := &domain.Tenant{}
		var settings sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &settings, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan tenant: %w", err)
		}
		if settings.Valid {
			t.Settings = json.RawMessage(settings.String)
		}
		tenants = append(tenants, t)
	}
	return tenants, total, rows.Err()
}

func (s *PostgresStore) UpdateTenant(ctx context.Context, t *domain.Tenant) error {
	t.UpdatedAt = time.Now().UTC()

	settings := sql.NullString{}
	if t.Settings != nil {
		settings = sql.NullString{String: string(t.Settings), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET name = $1, slug = $2, status = $3, settings = $4, updated_at = $5
		 WHERE id = $6`,
		t.Name, t.Slug, string(t.Status), settings, t.UpdatedAt, t.ID.String(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: tenant name or slug already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("update tenant: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteTenant(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
