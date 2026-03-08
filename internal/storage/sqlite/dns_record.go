package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dns_records (id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID.String(), r.ZoneID.String(), r.Name,
		string(r.Type), r.Value, r.TTL, r.Priority, r.Weight, r.Port,
		r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: dns record already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert dns record: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetDNSRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error) {
	r := &domain.DNSRecord{}

	err := s.db.QueryRowContext(ctx,
		`SELECT id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at
		 FROM dns_records WHERE id = ?`, id.String(),
	).Scan(&r.ID, &r.ZoneID, &r.Name, &r.Type, &r.Value,
		&r.TTL, &r.Priority, &r.Weight, &r.Port,
		&r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get dns record: %w", err)
	}

	return r, nil
}

func (s *SQLiteStore) ListDNSRecords(ctx context.Context, zoneID domain.ID, params storage.ListParams) ([]*domain.DNSRecord, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dns_records WHERE zone_id = ?`, zoneID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count dns records: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at
		 FROM dns_records WHERE zone_id = ? ORDER BY name ASC, type ASC LIMIT ? OFFSET ?`,
		zoneID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list dns records: %w", err)
	}
	defer rows.Close()

	var records []*domain.DNSRecord
	for rows.Next() {
		r := &domain.DNSRecord{}
		if err := rows.Scan(&r.ID, &r.ZoneID, &r.Name, &r.Type, &r.Value,
			&r.TTL, &r.Priority, &r.Weight, &r.Port,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan dns record: %w", err)
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

func (s *SQLiteStore) UpdateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	r.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_records SET name = ?, type = ?, value = ?, ttl = ?, priority = ?, weight = ?, port = ?, updated_at = ?
		 WHERE id = ?`,
		r.Name, string(r.Type), r.Value, r.TTL, r.Priority, r.Weight, r.Port,
		r.UpdatedAt, r.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update dns record: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteDNSRecord(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete dns record: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
