package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateAuditEvent(ctx context.Context, e *domain.AuditEvent) error {
	details := sql.NullString{}
	if e.Details != nil {
		details = sql.NullString{String: string(e.Details), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events (id, tenant_id, user_id, api_key_id, action, resource, details, source_ip, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID.String(), nullIDString(e.TenantID), nullIDString(e.UserID), nullIDString(e.APIKeyID),
		e.Action, e.Resource, details, e.SourceIP, e.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListAuditEvents(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.AuditEvent, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if tenantID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE tenant_id = ?`, tenantID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM audit_events`,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if tenantID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, user_id, api_key_id, action, resource, details, source_ip, timestamp
			 FROM audit_events WHERE tenant_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
			tenantID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, tenant_id, user_id, api_key_id, action, resource, details, source_ip, timestamp
			 FROM audit_events ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
			params.Limit, params.Offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		e := &domain.AuditEvent{}
		var tid, uid, akid sql.NullString
		var details sql.NullString

		if err := rows.Scan(&e.ID, &tid, &uid, &akid, &e.Action, &e.Resource, &details, &e.SourceIP, &e.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("scan audit event: %w", err)
		}
		if tid.Valid {
			id, err := uuid.Parse(tid.String)
			if err == nil {
				e.TenantID = &id
			}
		}
		if uid.Valid {
			id, err := uuid.Parse(uid.String)
			if err == nil {
				e.UserID = &id
			}
		}
		if akid.Valid {
			id, err := uuid.Parse(akid.String)
			if err == nil {
				e.APIKeyID = &id
			}
		}
		if details.Valid {
			e.Details = json.RawMessage(details.String)
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}
