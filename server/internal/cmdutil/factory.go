// Package cli provides the Aether CLI command tree, organized as an importable
// package so it can be wired (D38) and tested from external packages.
//
// Design: mirrors gh (cli/cli) factory pattern + multica (internal/cli/) layering.
// The Factory struct carries lazy deps (API client, config, IO). Commands pull
// deps inside RunE (not at construction) so --help/--version never touch network.
package cmdutil

import (
	"context"
	"io"
	"os"
)

// Build-time injected variables (set from cmd/aether/main.go).
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Factory carries every dependency a command can need.
// Fields are lazy: --help/--version never trigger API client or config load.
type Factory struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	ConfigPath string

	// Injected by wire; tests can stub without constructing the rest.
	NewClient func(cfg *Config) (APIClient, error)

	cfg    *Config
	client APIClient

	// Runtime overrides from flags
	flagServerURL string
	flagProfile   string
	flagOutput    string
	flagDebug     bool
}

// NewFactory creates a Factory with sensible defaults for production use.
func NewFactory() *Factory {
	return &Factory{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		ConfigPath:  defaultConfigPath(),
		NewClient:   defaultNewClient,
		flagOutput:  "table",
		flagProfile: "default",
	}
}

// Config lazily loads (and caches) the CLI config from disk.
func (f *Factory) Config() (*Config, error) {
	if f.cfg != nil {
		return f.cfg, nil
	}
	cfg, err := LoadConfig(f.ConfigPath)
	if err != nil {
		return nil, err
	}
	f.cfg = cfg
	return cfg, nil
}

// APIClient lazily builds the API client from the active profile.
func (f *Factory) APIClient(ctx context.Context) (APIClient, error) {
	if f.client != nil {
		return f.client, nil
	}
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}

	// Apply flag overrides
	if f.flagServerURL != "" {
		cfg.ServerURL = f.flagServerURL
	}

	if f.NewClient == nil {
		return nil, ErrNoClient
	}
	c, err := f.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	f.client = c
	return c, nil
}

// OutputFormat returns the active output format (table|json|yaml).
func (f *Factory) OutputFormat() string {
	return f.flagOutput
}

// applyFlagOverrides is called in PersistentPreRunE to promote flag values into the factory.
func (f *Factory) ApplyFlagOverrides(serverURL, profile, output string, debug bool) {
	if serverURL != "" {
		f.flagServerURL = serverURL
	}
	if profile != "" {
		f.flagProfile = profile
	}
	if output != "" {
		f.flagOutput = output
	}
	f.flagDebug = debug
}
