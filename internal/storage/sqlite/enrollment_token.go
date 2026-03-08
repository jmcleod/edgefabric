package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateEnrollmentToken(ctx context.Context, t *domain.EnrollmentToken) error {
	now := time.Now().UTC()
	t.CreatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO enrollment_tokens (id, tenant_id, target_type, target_id, token, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID.String(), t.TenantID.String(), string(t.TargetType), t.TargetID.String(),
		t.Token, t.ExpiresAt, t.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: enrollment token already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert enrollment token: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetEnrollmentToken(ctx context.Context, token string) (*domain.EnrollmentToken, error) {
	t := &domain.EnrollmentToken{}
	var usedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, target_type, target_id, token, expires_at, used_at, created_at
		 FROM enrollment_tokens WHERE token = ?`, token,
	).Scan(&t.ID, &t.TenantID, &t.TargetType, &t.TargetID, &t.Token,
		&t.ExpiresAt, &usedAt, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get enrollment token: %w", err)
	}

	if usedAt.Valid {
		t.UsedAt = &usedAt.Time
	}
	return t, nil
}

func (s *SQLiteStore) MarkEnrollmentTokenUsed(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE enrollment_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL`,
		now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("mark enrollment token used: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
