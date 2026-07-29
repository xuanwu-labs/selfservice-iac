// Package drift: provider.go — wire ProviderSet for the drift module (W2 final).
//
// Only NewScheduler is exposed to the wire graph. Its collaborators
// (clock.Clock, StackLister, StackChecker) are bound elsewhere — clock via
// clock.ProviderSet, the stack lister / checker via the modules that own the
// concrete types (catalog/stackmodel + this package's Worker adapter). Adding
// the full binding set in core.go is a Phase 2 wiring step; Phase 1 keeps the
// module buildable and testable in isolation.
package drift

import "github.com/google/wire"

// ProviderSet exposes NewScheduler to the wire dependency graph.
var ProviderSet = wire.NewSet(NewScheduler)
