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

func (s *PostgresStore) CreateRoute(ctx context.Context, r *domain.Route) error {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	if r.Status == "" {
		r.Status = domain.RouteStatusActive
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO routes (id, tenant_id, name, protocol, entry_ip, entry_port, gateway_id,
		 destination_ip, destination_port, node_group_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		r.ID.String(), r.TenantID.String(), r.Name, string(r.Protocol),
		r.EntryIP, nullIntPtr(r.EntryPort), r.GatewayID.String(),
		r.DestinationIP, nullIntPtr(r.DestinationPort),
		nullIDString(r.NodeGroupID), string(r.Status),
		r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: route already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert route: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error) {
	r := &domain.Route{}
	var entryPort, destPort sql.NullInt64
	var nodeGroupID sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port, gateway_id,
		        destination_ip, destination_port, node_group_id, status, created_at, updated_at
		 FROM routes WHERE id = $1`, id.String(),
	).Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol, &r.EntryIP, &entryPort,
		&r.GatewayID, &r.DestinationIP, &destPort, &nodeGroupID,
		&r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}

	applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
	return r, nil
}

func (s *PostgresStore) ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM routes WHERE tenant_id = $1`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count routes: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port, gateway_id,
		        destination_ip, destination_port, node_group_id, status, created_at, updated_at
		 FROM routes WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.Route
	for rows.Next() {
		r := &domain.Route{}
		var entryPort, destPort sql.NullInt64
		var nodeGroupID sql.NullString

		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol, &r.EntryIP, &entryPort,
			&r.GatewayID, &r.DestinationIP, &destPort, &nodeGroupID,
			&r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan route: %w", err)
		}

		applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
		routes = append(routes, r)
	}
	return routes, total, rows.Err()
}

func (s *PostgresStore) ListRoutesByGateway(ctx context.Context, gatewayID domain.ID) ([]*domain.Route, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port, gateway_id,
		        destination_ip, destination_port, node_group_id, status, created_at, updated_at
		 FROM routes WHERE gateway_id = $1 AND status = $2`,
		gatewayID.String(), string(domain.RouteStatusActive),
	)
	if err != nil {
		return nil, fmt.Errorf("list routes by gateway: %w", err)
	}
	defer rows.Close()

	var routes []*domain.Route
	for rows.Next() {
		r := &domain.Route{}
		var entryPort, destPort sql.NullInt64
		var nodeGroupID sql.NullString

		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol, &r.EntryIP, &entryPort,
			&r.GatewayID, &r.DestinationIP, &destPort, &nodeGroupID,
			&r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}

		applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

func (s *PostgresStore) UpdateRoute(ctx context.Context, r *domain.Route) error {
	r.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE routes SET name = $1, protocol = $2, entry_ip = $3, entry_port = $4,
		 gateway_id = $5, destination_ip = $6, destination_port = $7, node_group_id = $8,
		 status = $9, updated_at = $10
		 WHERE id = $11`,
		r.Name, string(r.Protocol), r.EntryIP, nullIntPtr(r.EntryPort),
		r.GatewayID.String(), r.DestinationIP, nullIntPtr(r.DestinationPort),
		nullIDString(r.NodeGroupID), string(r.Status), r.UpdatedAt, r.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteRoute(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// applyNullableRouteFields applies nullable SQL scan results to a Route.
func applyNullableRouteFields(r *domain.Route, entryPort, destPort sql.NullInt64, nodeGroupID sql.NullString) {
	if entryPort.Valid {
		v := int(entryPort.Int64)
		r.EntryPort = &v
	}
	if destPort.Valid {
		v := int(destPort.Int64)
		r.DestinationPort = &v
	}
	if nodeGroupID.Valid {
		parsed, err := uuid.Parse(nodeGroupID.String)
		if err == nil {
			r.NodeGroupID = &parsed
		}
	}
}
