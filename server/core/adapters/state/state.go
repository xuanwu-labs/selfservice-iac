// Package state defines the StateBackend adapter interface (D7).
//
// The codegen module (05) uses this to read/write Terraform state in object
// storage (S3/OSS). The default implementation is a noop stub.
package state

import (
	"context"
	"fmt"
)

// StateBackend abstracts remote state storage operations.
type StateBackend interface {
	// Read returns the raw state JSON for the given key.
	Read(ctx context.Context, key string) ([]byte, error)
	// Write stores the raw state JSON under the given key.
	Write(ctx context.Context, key string, data []byte) error
	// Delete removes the state object for the given key.
	Delete(ctx context.Context, key string) error
	// Lock acquires a distributed lock on the state object.
	// Returns a lock ID that must be passed to Unlock.
	Lock(ctx context.Context, key string) (string, error)
	// Unlock releases a previously acquired lock.
	Unlock(ctx context.Context, key, lockID string) error
}

// NoopState is the default stub.
type NoopState struct{}

// Read returns a structured error.
func (NoopState) Read(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("state adapter not configured: set adapters.state.impl in config")
}

// Write returns a structured error.
func (NoopState) Write(_ context.Context, _ string, _ []byte) error {
	return fmt.Errorf("state adapter not configured: set adapters.state.impl in config")
}

// Delete returns a structured error.
func (NoopState) Delete(_ context.Context, _ string) error {
	return fmt.Errorf("state adapter not configured: set adapters.state.impl in config")
}

// Lock returns a structured error.
func (NoopState) Lock(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("state adapter not configured: set adapters.state.impl in config")
}

// Unlock returns a structured error.
func (NoopState) Unlock(_ context.Context, _, _ string) error {
	return fmt.Errorf("state adapter not configured: set adapters.state.impl in config")
}
