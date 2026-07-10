// Package config provides configuration loading via viper.
// providerset.go is the wire ProviderSet entry for config.
//
// Load is NOT a wire provider — it's called manually in main.go (before wire)
// because it reads command-line flags (-config, -env) that wire can't inject.
// The already-loaded *Config is passed into wire via a binding in wire.go.
package config

import "github.com/google/wire"

// ProviderSet is empty — *Config is injected from main.go via wire.Bind.
var ProviderSet = wire.NewSet()
