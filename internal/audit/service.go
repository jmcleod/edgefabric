// Package audit provides audit logging for all state-changing operations.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Logger records audit events to persistent storage and structured logs.
type Logger interface {
	Log(ctx context.Context, event Event) error
	List(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.AuditEvent, int, error)
}

// Event is the input for recording an audit event.
type Event struct {
	TenantID *domain.ID
	UserID   *domain.ID
	APIKeyID *domain.ID
	Action   string
	Resource string
	Details  any
	SourceIP string
}

// DefaultLogger implements Logger using a Store and slog.
type DefaultLogger struct {
	store  storage.AuditEventStore
	logger *slog.Logger
}

// NewLogger creates an audit logger.
func NewLogger(store storage.AuditEventStore, logger *slog.Logger) Logger {
	return &DefaultLogger{store: store, logger: logger}
}

// Log records an audit event.
func (l *DefaultLogger) Log(ctx context.Context, event Event) error {
	detailsJSON, _ := json.Marshal(event.Details)

	ae := &domain.AuditEvent{
		ID:        domain.NewID(),
		TenantID:  event.TenantID,
		UserID:    event.UserID,
		APIKeyID:  event.APIKeyID,
		Action:    event.Action,
		Resource:  event.Resource,
		Details:   detailsJSON,
		SourceIP:  event.SourceIP,
		Timestamp: time.Now().UTC(),
	}

	l.logger.InfoContext(ctx, "audit",
		slog.String("action", event.Action),
		slog.String("resource", event.Resource),
		slog.String("source_ip", event.SourceIP),
	)

	return l.store.CreateAuditEvent(ctx, ae)
}

// List returns audit events with optional tenant filter.
func (l *DefaultLogger) List(ctx context.Context, tenantID *domain.ID, params storage.ListParams) ([]*domain.AuditEvent, int, error) {
	return l.store.ListAuditEvents(ctx, tenantID, params)
}
