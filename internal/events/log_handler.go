package events

import (
	"context"
	"log/slog"
)

// NewLogHandler returns a Handler that logs events using structured logging.
// Register this handler for event types where you want detailed log output
// beyond what the bus's built-in Publish logging provides.
func NewLogHandler(logger *slog.Logger) Handler {
	return func(ctx context.Context, event Event) error {
		attrs := []any{
			slog.String("type", string(event.Type)),
			slog.String("severity", string(event.Severity)),
			slog.String("resource", event.Resource),
		}
		for k, v := range event.Details {
			attrs = append(attrs, slog.String("detail."+k, v))
		}

		switch event.Severity {
		case SeverityCritical:
			logger.Error("system event", attrs...)
		case SeverityWarning:
			logger.Warn("system event", attrs...)
		default:
			logger.Info("system event", attrs...)
		}
		return nil
	}
}
