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

func (s *PostgresStore) CreateGateway(ctx context.Context, g *domain.Gateway) error {
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
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
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

func (s *PostgresStore) GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error) {
	g := &domain.Gateway{}
	var publicIP, wgIP, enrollToken, metadata sql.NullString
	var lastHeartbeat, lastConfigSync sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, public_ip, wireguard_ip, status, enrollment_token, last_heartbeat, last_config_sync, metadata, created_at, updated_at
		 FROM gateways WHERE id = $1`, id.String(),
	).Scan(&g.ID, &g.TenantID, &g.Name, &publicIP, &wgIP, &g.Status,
		&enrollToken, &lastHeartbeat, &lastConfigSync, &metadata, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway: %w", err)
	}

	applyNullableGatewayFields(g, publicIP, wgIP, enrollToken, metadata, lastHeartbeat, lastConfigSync)
	return g, nil
}

func (s *PostgresStore) ListGateways(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Gateway, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM gateways WHERE tenant_id = $1`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count gateways: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, public_ip, wireguard_ip, status, enrollment_token, last_heartbeat, last_config_sync, metadata, created_at, updated_at
		 FROM gateways WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
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
		var lastHeartbeat, lastConfigSync sql.NullTime

		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &publicIP, &wgIP, &g.Status,
			&enrollToken, &lastHeartbeat, &lastConfigSync, &metadata, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan gateway: %w", err)
		}
		applyNullableGatewayFields(g, publicIP, wgIP, enrollToken, metadata, lastHeartbeat, lastConfigSync)
		gateways = append(gateways, g)
	}
	return gateways, total, rows.Err()
}

func (s *PostgresStore) UpdateGateway(ctx context.Context, g *domain.Gateway) error {
	g.UpdatedAt = time.Now().UTC()

	metadata := sql.NullString{}
	if g.Metadata != nil {
		metadata = sql.NullString{String: string(g.Metadata), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE gateways SET name = $1, public_ip = $2, wireguard_ip = $3, status = $4,
		 enrollment_token = $5, metadata = $6, updated_at = $7
		 WHERE id = $8`,
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

func (s *PostgresStore) DeleteGateway(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM gateways WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete gateway: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateGatewayHeartbeat(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE gateways SET last_heartbeat = $1, status = $2, updated_at = $3 WHERE id = $4`,
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

func (s *PostgresStore) UpdateGatewayConfigSync(ctx context.Context, id domain.ID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE gateways SET last_config_sync = $1, updated_at = $2 WHERE id = $3`,
		now, now, id.String(),
	)
	if err != nil {
		return fmt.Errorf("update gateway config sync: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// applyNullableGatewayFields applies nullable SQL scan results to a Gateway.
func applyNullableGatewayFields(g *domain.Gateway, publicIP, wgIP, enrollToken, metadata sql.NullString, lastHeartbeat, lastConfigSync sql.NullTime) {
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
	if lastConfigSync.Valid {
		g.LastConfigSync = &lastConfigSync.Time
	}
}
