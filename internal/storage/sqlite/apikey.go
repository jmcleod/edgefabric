package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateAPIKey(ctx context.Context, k *domain.APIKey) error {
	k.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, tenant_id, user_id, name, key_hash, key_prefix, role, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID.String(), k.TenantID.String(), k.UserID.String(),
		k.Name, k.KeyHash, k.KeyPrefix, string(k.Role),
		k.ExpiresAt, k.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetAPIKey(ctx context.Context, id domain.ID) (*domain.APIKey, error) {
	k := &domain.APIKey{}
	var expiresAt sql.NullTime
	var lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, user_id, name, key_hash, key_prefix, role, expires_at, last_used_at, created_at
		 FROM api_keys WHERE id = ?`, id.String(),
	).Scan(&k.ID, &k.TenantID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &expiresAt, &lastUsedAt, &k.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	return k, nil
}

func (s *SQLiteStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	k := &domain.APIKey{}
	var expiresAt sql.NullTime
	var lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, user_id, name, key_hash, key_prefix, role, expires_at, last_used_at, created_at
		 FROM api_keys WHERE key_prefix = ?`, prefix,
	).Scan(&k.ID, &k.TenantID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Role, &expiresAt, &lastUsedAt, &k.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by prefix: %w", err)
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	return k, nil
}

func (s *SQLiteStore) ListAPIKeys(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.APIKey, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count api keys: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, user_id, name, key_hash, key_prefix, role, expires_at, last_used_at, created_at
		 FROM api_keys WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		k := &domain.APIKey{}
		var expiresAt sql.NullTime
		var lastUsedAt sql.NullTime

		if err := rows.Scan(&k.ID, &k.TenantID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix,
			&k.Role, &expiresAt, &lastUsedAt, &k.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan api key: %w", err)
		}
		if expiresAt.Valid {
			k.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			k.LastUsedAt = &lastUsedAt.Time
		}
		keys = append(keys, k)
	}
	return keys, total, rows.Err()
}

func (s *SQLiteStore) DeleteAPIKey(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) UpdateAPIKeyLastUsed(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
