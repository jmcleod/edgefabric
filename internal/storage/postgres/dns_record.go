package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dns_records (id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		r.ID.String(), r.ZoneID.String(), r.Name,
		string(r.Type), r.Value,
		nullIntPtr(r.TTL), nullIntPtr(r.Priority), nullIntPtr(r.Weight), nullIntPtr(r.Port),
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

func (s *PostgresStore) GetDNSRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error) {
	r := &domain.DNSRecord{}
	var ttl, priority, weight, port sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at
		 FROM dns_records WHERE id = $1`, id.String(),
	).Scan(&r.ID, &r.ZoneID, &r.Name, &r.Type, &r.Value,
		&ttl, &priority, &weight, &port,
		&r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get dns record: %w", err)
	}

	r.TTL = scanNullInt(ttl)
	r.Priority = scanNullInt(priority)
	r.Weight = scanNullInt(weight)
	r.Port = scanNullInt(port)

	return r, nil
}

func (s *PostgresStore) ListDNSRecords(ctx context.Context, zoneID domain.ID, params storage.ListParams) ([]*domain.DNSRecord, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dns_records WHERE zone_id = $1`, zoneID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count dns records: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, zone_id, name, type, value, ttl, priority, weight, port, created_at, updated_at
		 FROM dns_records WHERE zone_id = $1 ORDER BY name ASC, type ASC LIMIT $2 OFFSET $3`,
		zoneID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list dns records: %w", err)
	}
	defer rows.Close()

	var records []*domain.DNSRecord
	for rows.Next() {
		r := &domain.DNSRecord{}
		var ttl, priority, weight, port sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ZoneID, &r.Name, &r.Type, &r.Value,
			&ttl, &priority, &weight, &port,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan dns record: %w", err)
		}
		r.TTL = scanNullInt(ttl)
		r.Priority = scanNullInt(priority)
		r.Weight = scanNullInt(weight)
		r.Port = scanNullInt(port)
		records = append(records, r)
	}
	return records, total, rows.Err()
}

func (s *PostgresStore) UpdateDNSRecord(ctx context.Context, r *domain.DNSRecord) error {
	r.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE dns_records SET name = $1, type = $2, value = $3, ttl = $4, priority = $5, weight = $6, port = $7, updated_at = $8
		 WHERE id = $9`,
		r.Name, string(r.Type), r.Value,
		nullIntPtr(r.TTL), nullIntPtr(r.Priority), nullIntPtr(r.Weight), nullIntPtr(r.Port),
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

func (s *PostgresStore) DeleteDNSRecord(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete dns record: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// scanNullInt converts a sql.NullInt64 to *int.
func scanNullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}
