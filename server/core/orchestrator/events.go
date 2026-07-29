package orchestrator

import (
	"context"
	"sync"
	"time"
)

// EventRecorder is the persistence port behind EventLogger: it appends a row
// to request_events. The real implementation (data layer, W1-02) writes via
// sqlc queries against the request_events table (migration 005). Defining it
// here keeps the orchestrator free of a data-layer import and lets tests use
// the in-memory implementation below.
type EventRecorder interface {
	RecordEvent(ctx context.Context, requestID int64, fromStatus, toStatus *string, actor string, occurredAt time.Time) error
}

// eventLogger is the default EventLogger: it adapts EventLogger.LogEvent's
// map[string]any context down to the row-level fields the recorder persists,
// and stamps occurredAt with wall-clock now(). The from/to pair becomes the
// row's from_status / to_status (nullable for non-transition log calls).
type eventLogger struct {
	recorder EventRecorder
}

// NewEventLogger binds an EventLogger to a recorder. Used by wire.
func NewEventLogger(recorder EventRecorder) EventLogger {
	return &eventLogger{recorder: recorder}
}

// LogEvent implements EventLogger. from / to are passed as pointers to the
// recorder so it can store NULL when either side of the transition is absent
// (e.g. a pure log line that is not a state_transition).
func (l *eventLogger) LogEvent(ctx context.Context, requestID int64, from, to, actor string, _ map[string]any) error {
	var fromPtr, toPtr *string
	if from != "" {
		fromPtr = strPtr(from)
	}
	if to != "" {
		toPtr = strPtr(to)
	}
	return l.recorder.RecordEvent(ctx, requestID, fromPtr, toPtr, actor, time.Now())
}

func strPtr(s string) *string { return &s }

// MemEventRecorder is an in-memory EventRecorder for tests. It records every
// call in order behind a mutex so concurrent Pipeline / ApprovalService code
// can be asserted on deterministically. It never returns an error.
type MemEventRecorder struct {
	mu      sync.Mutex
	Events  []RecordedEvent
	FailErr error // if non-nil, RecordEvent returns it (for error-path tests)
}

// RecordedEvent is one row as captured by MemEventRecorder. Time is captured
// for ordering assertions; the from/to pointers mirror the row schema.
type RecordedEvent struct {
	RequestID  int64
	FromStatus *string
	ToStatus   *string
	Actor      string
	OccurredAt time.Time
}

// RecordEvent appends to the in-memory log. Thread-safe.
func (m *MemEventRecorder) RecordEvent(_ context.Context, requestID int64, from, to *string, actor string, occurredAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailErr != nil {
		return m.FailErr
	}
	m.Events = append(m.Events, RecordedEvent{
		RequestID:  requestID,
		FromStatus: from,
		ToStatus:   to,
		Actor:      actor,
		OccurredAt: occurredAt,
	})
	return nil
}

// Last returns the most recently recorded event, or nil if none. Useful for
// terse assertions in tests.
func (m *MemEventRecorder) Last() *RecordedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Events) == 0 {
		return nil
	}
	e := m.Events[len(m.Events)-1]
	return &e
}

// Count returns the number of recorded events. Thread-safe.
func (m *MemEventRecorder) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Events)
}
