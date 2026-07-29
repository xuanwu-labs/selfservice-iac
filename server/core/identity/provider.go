// Package identity provider.go is the wire aggregation point for the identity
// package. Kept in a separate file from service.go so the DI surface is
// visible without scanning the implementation.
package identity

import "github.com/google/wire"

// ProviderSet binds NewIdentityService for wire. internal/auth + api/connect
// inject *IdentityService directly.
var ProviderSet = wire.NewSet(NewIdentityService)
