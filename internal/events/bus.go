package events

import (
	"context"
	"log/slog"
	"sync"
)

// Handler processes a published event. Handlers run asynchronously and
// errors are logged but do not affect other subscribers.
type Handler func(ctx context.Context, event Event) error

// Bus is an in-process event bus that delivers events to registered handlers.
// It is safe for concurrent use.
//
// FUTURE: Add webhook, Slack, PagerDuty notification handlers.
type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
	logger   *slog.Logger
}

// NewBus creates a new event bus. A default logging handler is registered
// automatically for all event types.
func NewBus(logger *slog.Logger) *Bus {
	b := &Bus{
		handlers: make(map[EventType][]Handler),
		logger:   logger,
	}
	return b
}

// Subscribe registers a handler for the given event type.
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish sends an event to all registered handlers for its type.
// Handlers run asynchronously (fire-and-forget). Errors from individual
// handlers are logged but do not affect other subscribers.
func (b *Bus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	// Always log the event at info level.
	b.logger.Info("event published",
		slog.String("type", string(event.Type)),
		slog.String("severity", string(event.Severity)),
		slog.String("resource", event.Resource),
	)

	for _, h := range handlers {
		go func(handler Handler) {
			if err := handler(ctx, event); err != nil {
				b.logger.Error("event handler failed",
					slog.String("type", string(event.Type)),
					slog.String("error", err.Error()),
				)
			}
		}(h)
	}
}
