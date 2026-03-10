package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateUser(ctx context.Context, u *domain.User) error {
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.Status == "" {
		u.Status = domain.UserStatusActive
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		u.ID.String(), nullIDString(u.TenantID), u.Email, u.Name,
		u.PasswordHash, nullString(&u.TOTPSecret),
		u.TOTPEnabled, string(u.Role), string(u.Status),
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: email already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetUser(ctx context.Context, id domain.ID) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
		 FROM users WHERE id = $1`, id.String(),
	))
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	))
}

func (s *PostgresStore) scanUser(row *sql.Row) (*domain.User, error) {
	u := &domain.User{}
	var tenantID sql.NullString
	var totpSecret sql.NullString
	var lastLoginAt sql.NullTime

	err := row.Scan(
		&u.ID, &tenantID, &u.Email, &u.Name,
		&u.PasswordHash, &totpSecret, &u.TOTPEnabled,
		&u.Role, &u.Status, &lastLoginAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}

	if tenantID.Valid {
		id, err := uuid.Parse(tenantID.String)
		if err == nil {
			u.TenantID = &id
		}
	}
	if totpSecret.Valid {
		u.TOTPSecret = totpSecret.String
	}
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	return u, nil
}

func (s *PostgresStore) ListUsers(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.User, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if tenantID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM users WHERE tenant_id = $1`, tenantID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count users: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if tenantID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
			 FROM users WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			tenantID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
			 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			params.Limit, params.Offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		var tid sql.NullString
		var totpSecret sql.NullString
		var lastLoginAt sql.NullTime

		if err := rows.Scan(
			&u.ID, &tid, &u.Email, &u.Name,
			&u.PasswordHash, &totpSecret, &u.TOTPEnabled,
			&u.Role, &u.Status, &lastLoginAt,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user row: %w", err)
		}

		if tid.Valid {
			id, err := uuid.Parse(tid.String)
			if err == nil {
				u.TenantID = &id
			}
		}
		if totpSecret.Valid {
			u.TOTPSecret = totpSecret.String
		}
		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

func (s *PostgresStore) UpdateUser(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET tenant_id = $1, email = $2, name = $3, password_hash = $4,
		 totp_secret = $5, totp_enabled = $6, role = $7, status = $8, last_login_at = $9, updated_at = $10
		 WHERE id = $11`,
		nullIDString(u.TenantID), u.Email, u.Name, u.PasswordHash,
		nullString(&u.TOTPSecret), u.TOTPEnabled,
		string(u.Role), string(u.Status),
		u.LastLoginAt, u.UpdatedAt, u.ID.String(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: email already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("update user: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
