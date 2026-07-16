// Package notify defines the Notifier adapter interface (D7).
//
// The events module uses this to send notifications (IM, email, webhook)
// for request lifecycle events, approval prompts, and drift alerts.
// The default implementation is a noop stub.
package notify

import (
	"context"
	"fmt"
)

// Notification represents a single notification payload.
type Notification struct {
	Type    string // e.g. "approval_requested", "apply_succeeded", "drift_detected"
	Title   string
	Message string
}

// Notifier abstracts notification delivery so the platform can use
// Slack, DingTalk, email, or any other channel.
type Notifier interface {
	// Notify sends a single notification. Implementations should be
	// non-blocking or use a queue; callers should not wait on delivery.
	Notify(ctx context.Context, event Notification) error
}

// NoopNotifier is the default stub.
type NoopNotifier struct{}

// Notify returns a structured error.
func (NoopNotifier) Notify(_ context.Context, _ Notification) error {
	return fmt.Errorf("notify adapter not configured: set adapters.notify.impl in config")
}
