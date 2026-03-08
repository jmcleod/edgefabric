package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateUser(ctx context.Context, u *domain.User) error {
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.Status == "" {
		u.Status = domain.UserStatusActive
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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

func (s *SQLiteStore) GetUser(ctx context.Context, id domain.ID) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
		 FROM users WHERE id = ?`, id.String(),
	))
}

func (s *SQLiteStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	))
}

func (s *SQLiteStore) scanUser(row *sql.Row) (*domain.User, error) {
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

func (s *SQLiteStore) ListUsers(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.User, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if tenantID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM users WHERE tenant_id = ?`, tenantID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM users`,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count users: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if tenantID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
			 FROM users WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			tenantID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, email, name, password_hash, totp_secret, totp_enabled, role, status, last_login_at, created_at, updated_at
			 FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`,
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

func (s *SQLiteStore) UpdateUser(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET tenant_id = ?, email = ?, name = ?, password_hash = ?,
		 totp_secret = ?, totp_enabled = ?, role = ?, status = ?, last_login_at = ?, updated_at = ?
		 WHERE id = ?`,
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

func (s *SQLiteStore) DeleteUser(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
