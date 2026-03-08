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

func (s *SQLiteStore) CreateGateway(ctx context.Context, g *domain.Gateway) error {
	now := time.Now().UTC()
	g.CreatedAt = now
	g.UpdatedAt = now
	if g.Status == "" {
		g.Status = domain.GatewayStatusPending
	}

	metadata := sql.NullString{}
	if g.Metadata != nil {
		metadata = sql.NullString{String: string(g.Metadata), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO gateways (id, tenant_id, name, public_ip, wireguard_ip, status, enrollment_token, last_heartbeat, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID.String(), g.TenantID.String(), g.Name,
		nullStringEmpty(g.PublicIP), nullStringEmpty(g.WireGuardIP),
		string(g.Status), nullStringEmpty(g.EnrollmentToken),
		nil, metadata, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: gateway already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert gateway: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error) {
	g := &domain.Gateway{}
	var publicIP, wgIP, enrollToken, metadata sql.NullString
	var lastHeartbeat sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, public_ip, wireguard_ip, status, enrollment_token, last_heartbeat, metadata, created_at, updated_at
		 FROM gateways WHERE id = ?`, id.String(),
	).Scan(&g.ID, &g.TenantID, &g.Name, &publicIP, &wgIP, &g.Status,
		&enrollToken, &lastHeartbeat, &metadata, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway: %w", err)
	}

	applyNullableGatewayFields(g, publicIP, wgIP, enrollToken, metadata, lastHeartbeat)
	return g, nil
}

func (s *SQLiteStore) ListGateways(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Gateway, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM gateways WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count gateways: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, public_ip, wireguard_ip, status, enrollment_token, last_heartbeat, metadata, created_at, updated_at
		 FROM gateways WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list gateways: %w", err)
	}
	defer rows.Close()

	var gateways []*domain.Gateway
	for rows.Next() {
		g := &domain.Gateway{}
		var publicIP, wgIP, enrollToken, metadata sql.NullString
		var lastHeartbeat sql.NullTime

		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &publicIP, &wgIP, &g.Status,
			&enrollToken, &lastHeartbeat, &metadata, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan gateway: %w", err)
		}
		applyNullableGatewayFields(g, publicIP, wgIP, enrollToken, metadata, lastHeartbeat)
		gateways = append(gateways, g)
	}
	return gateways, total, rows.Err()
}

func (s *SQLiteStore) UpdateGateway(ctx context.Context, g *domain.Gateway) error {
	g.UpdatedAt = time.Now().UTC()

	metadata := sql.NullString{}
	if g.Metadata != nil {
		metadata = sql.NullString{String: string(g.Metadata), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE gateways SET name = ?, public_ip = ?, wireguard_ip = ?, status = ?,
		 enrollment_token = ?, metadata = ?, updated_at = ?
		 WHERE id = ?`,
		g.Name, nullStringEmpty(g.PublicIP), nullStringEmpty(g.WireGuardIP),
		string(g.Status), nullStringEmpty(g.EnrollmentToken),
		metadata, g.UpdatedAt, g.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update gateway: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteGateway(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM gateways WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete gateway: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) UpdateGatewayHeartbeat(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE gateways SET last_heartbeat = ?, status = ?, updated_at = ? WHERE id = ?`,
		now, string(domain.GatewayStatusOnline), now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("update gateway heartbeat: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// applyNullableGatewayFields applies nullable SQL scan results to a Gateway.
func applyNullableGatewayFields(g *domain.Gateway, publicIP, wgIP, enrollToken, metadata sql.NullString, lastHeartbeat sql.NullTime) {
	if publicIP.Valid {
		g.PublicIP = publicIP.String
	}
	if wgIP.Valid {
		g.WireGuardIP = wgIP.String
	}
	if enrollToken.Valid {
		g.EnrollmentToken = enrollToken.String
	}
	if metadata.Valid {
		g.Metadata = json.RawMessage(metadata.String)
	}
	if lastHeartbeat.Valid {
		g.LastHeartbeat = &lastHeartbeat.Time
	}
}
