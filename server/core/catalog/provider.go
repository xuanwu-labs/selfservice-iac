// Package catalog: provider.go — wire ProviderSet for the JSON Schema
// validator (D40).
package catalog

import "github.com/google/wire"

// ProviderSet provides the catalog schema Validator.
var ProviderSet = wire.NewSet(NewValidator)
