package cmdutil

import "errors"

// ErrNoClient is returned when no API client constructor is configured.
var ErrNoClient = errors.New("no API client constructor configured")

// APIClient is the interface the CLI uses to talk to the Aether platform.
// In骨架 phase it's a minimal interface; task 15 (Connect-RPC) will expand it.
type APIClient interface {
	// TODO(task-15): add Connect-RPC service methods
}

// defaultNewClient builds a real API client from config (placeholder until task 15).
func defaultNewClient(cfg *Config) (APIClient, error) {
	// TODO(task-15): return connect client from cfg.ServerURL + cfg.Token
	return nil, errors.New("API client not yet implemented (task 15)")
}
