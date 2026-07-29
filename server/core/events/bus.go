// Package events provides the Phase 1 in-process event bus (design D5).
//
// Phase 1 ships a synchronous, in-process bus: Publish walks the registered
// handlers in registration order, invoking each in turn on the caller's
// goroutine. There is no queue, no retries, no at-least-once delivery —
// durable delivery is the job of the outbox_events table + a Phase 2 relay.
//
// The bus is error-tolerant: a single handler error is logged and does not
// abort the remaining handlers (one bad handler must not poison the others).
// The caller of Publish never sees handler errors — those are surfaced via
// the bus's logger only. This is intentional: events are fire-and-forget
// telemetry/audit fan-out; failures here must not roll back the business
// operation that triggered them.
package events

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Event is the canonical envelope published to the bus. Type is a free-form
// verb (e.g. "request.created", "approval.decided"); Payload carries the
// domain-specific fields the handler knows how to interpret; CorrelationID
// threads a trace through all handlers (typically read from ctx, but
// publishers may override).
type Event struct {
	Type          string
	Payload       map[string]any
	CorrelationID string
}

// EventHandler handles one Event. Implementations are responsible for their
// own idempotency — the bus does not deduplicate (Phase 1 single-process, so
// at-most-once in practice; Phase 2 relay will handle at-least-once).
type EventHandler func(ctx context.Context, event Event) error

// EventBus fans Events out to all registered handlers in-process.
// Safe for concurrent Register + Publish; handlers run serially within a
// single Publish call (no per-handler goroutine) so the caller controls the
// concurrency boundary.
type EventBus struct {
	mu       sync.RWMutex
	handlers []EventHandler
	logger   *zap.Logger
}

// NewEventBus constructs an empty bus. logger may be nil — a no-op logger is
// used in that case (handler errors are then silently swallowed, which is
// appropriate for unit tests but not for production).
func NewEventBus(logger *zap.Logger) *EventBus {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EventBus{logger: logger}
}

// Register appends a handler. Registrations made after Publish is in flight
// on another goroutine will be visible to subsequent Publish calls but not to
// the in-flight one (RLock snapshot semantics).
func (b *EventBus) Register(handler EventHandler) {
	if handler == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// Publish dispatches event to every registered handler in order. Errors from
// one handler are logged and do not stop the dispatch; Publish itself never
// returns an error (see package doc for the rationale).
func (b *EventBus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	handlers := b.handlers
	b.mu.RUnlock()

	// Inherit correlation id from context when the publisher did not set one.
	if event.CorrelationID == "" {
		event.CorrelationID = CorrelationIDFromContext(ctx)
	}

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			b.logger.Warn("event handler failed",
				zap.String("event_type", event.Type),
				zap.String("correlation_id", event.CorrelationID),
				zap.Error(err),
			)
		}
	}
}

// correlationIDKey is an unexported context-key type so callers cannot
// collide with us.
type correlationIDKey struct{}

// WithCorrelationID returns ctx annotated with a correlation id. AuditLogger,
// the bus, and Connect handlers all read it back via CorrelationIDFromContext
// so a single trace id flows end-to-end.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// CorrelationIDFromContext returns the correlation id stored via
// WithCorrelationID, or "" when none is set.
func CorrelationIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return v
	}
	return ""
}
