// Package events provider.go is the wire aggregation point for the events
// package. Kept in a separate file from bus.go so the DI surface is visible
// without scanning the implementation.
package events

import "github.com/google/wire"

// ProviderSet binds NewEventBus for wire. Consumers (audit logger wiring,
// future notification handlers) inject *EventBus.
var ProviderSet = wire.NewSet(NewEventBus)
