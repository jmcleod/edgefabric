package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateRoute(ctx context.Context, r *domain.Route) error {
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	if r.Status == "" {
		r.Status = domain.RouteStatusActive
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO routes (id, tenant_id, name, protocol, entry_ip, entry_port,
		 gateway_id, destination_ip, destination_port, node_group_id, status,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID.String(), r.TenantID.String(), r.Name,
		string(r.Protocol), r.EntryIP, nullIntPtr(r.EntryPort),
		r.GatewayID.String(), r.DestinationIP, nullIntPtr(r.DestinationPort),
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

func (s *SQLiteStore) GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error) {
	r := &domain.Route{}
	var entryPort, destPort sql.NullInt64
	var nodeGroupID sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port,
		        gateway_id, destination_ip, destination_port, node_group_id,
		        status, created_at, updated_at
		 FROM routes WHERE id = ?`, id.String(),
	).Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol,
		&r.EntryIP, &entryPort, &r.GatewayID, &r.DestinationIP,
		&destPort, &nodeGroupID, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}

	applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
	return r, nil
}

func (s *SQLiteStore) ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM routes WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count routes: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port,
		        gateway_id, destination_ip, destination_port, node_group_id,
		        status, created_at, updated_at
		 FROM routes WHERE tenant_id = ? ORDER BY name ASC LIMIT ? OFFSET ?`,
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

		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol,
			&r.EntryIP, &entryPort, &r.GatewayID, &r.DestinationIP,
			&destPort, &nodeGroupID, &r.Status, &r.CreatedAt,
			&r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan route: %w", err)
		}
		applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
		routes = append(routes, r)
	}
	return routes, total, rows.Err()
}

func (s *SQLiteStore) UpdateRoute(ctx context.Context, r *domain.Route) error {
	r.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE routes SET name = ?, protocol = ?, entry_ip = ?, entry_port = ?,
		 gateway_id = ?, destination_ip = ?, destination_port = ?,
		 node_group_id = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		r.Name, string(r.Protocol), r.EntryIP, nullIntPtr(r.EntryPort),
		r.GatewayID.String(), r.DestinationIP, nullIntPtr(r.DestinationPort),
		nullIDString(r.NodeGroupID), string(r.Status),
		r.UpdatedAt, r.ID.String(),
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

func (s *SQLiteStore) DeleteRoute(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListRoutesByGateway(ctx context.Context, gatewayID domain.ID) ([]*domain.Route, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, protocol, entry_ip, entry_port,
		        gateway_id, destination_ip, destination_port, node_group_id,
		        status, created_at, updated_at
		 FROM routes WHERE gateway_id = ? AND status = ? ORDER BY name ASC`,
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

		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Protocol,
			&r.EntryIP, &entryPort, &r.GatewayID, &r.DestinationIP,
			&destPort, &nodeGroupID, &r.Status, &r.CreatedAt,
			&r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		applyNullableRouteFields(r, entryPort, destPort, nodeGroupID)
		routes = append(routes, r)
	}
	return routes, rows.Err()
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
		id, err := domain.ParseID(nodeGroupID.String)
		if err == nil {
			r.NodeGroupID = &id
		}
	}
}

// nullIntPtr converts an *int to sql.NullInt64.
func nullIntPtr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
