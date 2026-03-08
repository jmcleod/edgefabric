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

func (s *SQLiteStore) CreateTenant(ctx context.Context, t *domain.Tenant) error {
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
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
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

func (s *SQLiteStore) GetTenant(ctx context.Context, id domain.ID) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM tenants WHERE id = ?`, id.String(),
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

func (s *SQLiteStore) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM tenants WHERE slug = ?`, slug,
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

func (s *SQLiteStore) ListTenants(ctx context.Context, params storage.ListParams) ([]*domain.Tenant, int, error) {
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
		 FROM tenants ORDER BY created_at DESC LIMIT ? OFFSET ?`,
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

func (s *SQLiteStore) UpdateTenant(ctx context.Context, t *domain.Tenant) error {
	t.UpdatedAt = time.Now().UTC()

	settings := sql.NullString{}
	if t.Settings != nil {
		settings = sql.NullString{String: string(t.Settings), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET name = ?, slug = ?, status = ?, settings = ?, updated_at = ?
		 WHERE id = ?`,
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

func (s *SQLiteStore) DeleteTenant(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
