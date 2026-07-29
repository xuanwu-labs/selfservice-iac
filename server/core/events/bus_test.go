package events

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
)

func TestEventBusPublishInvokesAllHandlers(t *testing.T) {
	bus := NewEventBus(zap.NewNop())
	var first, second atomic.Int32

	bus.Register(func(ctx context.Context, e Event) error {
		first.Add(1)
		return nil
	})
	bus.Register(func(ctx context.Context, e Event) error {
		second.Add(1)
		return nil
	})

	bus.Publish(context.Background(), Event{Type: "request.created", Payload: map[string]any{"id": "1"}})

	if first.Load() != 1 {
		t.Fatalf("first handler should be called once, got %d", first.Load())
	}
	if second.Load() != 1 {
		t.Fatalf("second handler should be called once, got %d", second.Load())
	}
}

func TestEventBusPublishIsErrorTolerant(t *testing.T) {
	bus := NewEventBus(zap.NewNop())
	var boom, after atomic.Int32

	// First handler fails; second handler must still run.
	bus.Register(func(ctx context.Context, e Event) error {
		boom.Add(1)
		return errors.New("boom")
	})
	bus.Register(func(ctx context.Context, e Event) error {
		after.Add(1)
		return nil
	})

	bus.Publish(context.Background(), Event{Type: "request.created"})

	if boom.Load() != 1 {
		t.Fatalf("failing handler should be called once, got %d", boom.Load())
	}
	if after.Load() != 1 {
		t.Fatalf("subsequent handler must still be called, got %d", after.Load())
	}
}

func TestEventBusCorrelationIDFromContext(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "trace-abc")
	if got := CorrelationIDFromContext(ctx); got != "trace-abc" {
		t.Fatalf("expected trace-abc, got %q", got)
	}
	if got := CorrelationIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestEventBusInheritsCorrelationIDFromContext(t *testing.T) {
	bus := NewEventBus(zap.NewNop())
	var seen Event
	bus.Register(func(ctx context.Context, e Event) error {
		seen = e
		return nil
	})

	ctx := WithCorrelationID(context.Background(), "from-ctx")
	bus.Publish(ctx, Event{Type: "x"})

	if seen.CorrelationID != "from-ctx" {
		t.Fatalf("expected from-ctx, got %q", seen.CorrelationID)
	}
}

func TestEventBusRegisterNilNoOp(t *testing.T) {
	bus := NewEventBus(zap.NewNop())
	bus.Register(nil) // must not panic
	bus.Publish(context.Background(), Event{Type: "x"})
}
