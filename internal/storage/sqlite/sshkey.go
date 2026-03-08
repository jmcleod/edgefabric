package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateSSHKey(ctx context.Context, k *domain.SSHKey) error {
	now := time.Now().UTC()
	k.CreatedAt = now
	k.LastRotatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ssh_keys (id, name, public_key, private_key, fingerprint, created_at, last_rotated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		k.ID.String(), k.Name, k.PublicKey, k.PrivateKey, k.Fingerprint,
		k.CreatedAt, k.LastRotatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: SSH key already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert ssh key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSSHKey(ctx context.Context, id domain.ID) (*domain.SSHKey, error) {
	k := &domain.SSHKey{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, public_key, private_key, fingerprint, created_at, last_rotated_at
		 FROM ssh_keys WHERE id = ?`, id.String(),
	).Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.CreatedAt, &k.LastRotatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ssh key: %w", err)
	}
	return k, nil
}

func (s *SQLiteStore) ListSSHKeys(ctx context.Context, params storage.ListParams) ([]*domain.SSHKey, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ssh_keys`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count ssh keys: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, public_key, private_key, fingerprint, created_at, last_rotated_at
		 FROM ssh_keys ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list ssh keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.SSHKey
	for rows.Next() {
		k := &domain.SSHKey{}
		if err := rows.Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.CreatedAt, &k.LastRotatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ssh key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, total, rows.Err()
}

func (s *SQLiteStore) UpdateSSHKey(ctx context.Context, k *domain.SSHKey) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE ssh_keys SET name = ?, public_key = ?, private_key = ?, fingerprint = ?, last_rotated_at = ?
		 WHERE id = ?`,
		k.Name, k.PublicKey, k.PrivateKey, k.Fingerprint, k.LastRotatedAt, k.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update ssh key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteSSHKey(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete ssh key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
