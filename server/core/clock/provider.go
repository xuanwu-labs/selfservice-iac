// Package clock: provider.go — wire ProviderSet for the Clock abstraction (D44).
package clock

import "github.com/google/wire"

// ProviderSet provides the production Clock (realClock, UTC).
// FakeClock is test-only and NOT in the ProviderSet.
var ProviderSet = wire.NewSet(New)
