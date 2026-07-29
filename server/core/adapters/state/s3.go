// Package state: s3.go — S3Backend in-memory mock implementation (D1).
//
// Phase 1 deliberately does NOT connect to a real S3 / OSS endpoint. The
// production StateBackend contract is exercised through this mock, which keeps
// an in-memory map of objects plus a per-key lock table. Real S3 SDK
// integration is a Phase 2 / W3 concern (D1, D-NonGoal: "不连真实 S3").
//
// Wire default REMAINS NoopState (P2-10: a mock that silently persists in
// memory must never become the wire default — that would lose production state
// without erroring). S3Backend is intended for tests; production callers
// opt-in via an explicit wire.Bind (not added here).
package state

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// S3Backend is a StateBackend backed by in-memory maps (D1 mock).
//
// The zero value is NOT safe to use; construct via NewS3Backend.
type S3Backend struct {
	mu sync.Mutex

	// objects stores state JSON keyed by object key.
	objects map[string][]byte
	// locks maps key -> lockID currently holding the lock.
	// Absent key == unlocked.
	locks map[string]string
}

// Compile-time: S3Backend satisfies StateBackend.
var _ StateBackend = (*S3Backend)(nil)

// NewS3Backend constructs an empty in-memory S3Backend suitable for tests.
func NewS3Backend() *S3Backend {
	return &S3Backend{
		objects: make(map[string][]byte),
		locks:   make(map[string]string),
	}
}

// Read returns the stored bytes for key, or an error if the object is missing.
func (s *S3Backend) Read(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.objects[key]
	if !ok {
		return nil, fmt.Errorf("state: object not found: %s", key)
	}
	// Return a copy so callers cannot mutate the stored slice.
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// Write stores data under key, copying the slice so later caller mutations do
// not affect the stored value.
func (s *S3Backend) Write(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := make([]byte, len(data))
	copy(stored, data)
	s.objects[key] = stored
	return nil
}

// Delete removes the object for key. Deleting a missing key is a no-op (S3
// semantics: idempotent DELETE).
func (s *S3Backend) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, key)
	return nil
}

// Lock acquires a lock for key, returning the lockID the caller MUST pass to
// Unlock. If key is already locked, Lock returns an error.
func (s *S3Backend) Lock(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, held := s.locks[key]; held {
		return "", fmt.Errorf("state: lock already held for key %s", key)
	}

	lockID := uuid.NewString()
	s.locks[key] = lockID
	return lockID, nil
}

// Unlock releases the lock for key if lockID matches the holder. A mismatched
// lockID (or unlocking an unheld key) is an error.
func (s *S3Backend) Unlock(_ context.Context, key, lockID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, held := s.locks[key]
	if !held {
		return fmt.Errorf("state: unlock failed: key %s not locked", key)
	}
	if current != lockID {
		return fmt.Errorf("state: unlock failed: lockID mismatch for key %s", key)
	}
	delete(s.locks, key)
	return nil
}
