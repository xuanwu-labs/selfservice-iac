// Package config provides configuration loading via viper.
// This file also serves as the wire ProviderSet entry for config.
package config

import "github.com/google/wire"

// ProviderSet provides config-layer dependencies for wire.
var ProviderSet = wire.NewSet(
	Load,
)
