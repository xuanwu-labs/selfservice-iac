package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/state"
)

// newBE returns a fresh S3Backend + context for a test.
func newBE(t *testing.T) (*state.S3Backend, context.Context) {
	t.Helper()
	return state.NewS3Backend(), context.Background()
}

func TestS3Backend_ReadWriteDeleteRoundTrip(t *testing.T) {
	backend, ctx := newBE(t)
	key := "envs/prod/global/default.tfstate"
	payload := []byte(`{"version":4,"serial":1}`)

	// Write then read back.
	require.NoError(t, backend.Write(ctx, key, payload))
	got, err := backend.Read(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	// Mutating the returned slice must not corrupt stored state.
	got[0] = 'X'
	got2, err := backend.Read(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, payload, got2, "Read must return a defensive copy")

	// Delete then Read should fail.
	require.NoError(t, backend.Delete(ctx, key))
	_, err = backend.Read(ctx, key)
	require.Error(t, err, "Read after Delete must error")
}

func TestS3Backend_WriteCopyIsDefensive(t *testing.T) {
	backend, ctx := newBE(t)
	key := "k.tfstate"
	payload := []byte("hello")

	require.NoError(t, backend.Write(ctx, key, payload))
	// Mutate the caller's slice after Write.
	payload[0] = 'J'

	got, err := backend.Read(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got), "Write must copy, not alias")
}

func TestS3Backend_DeleteMissingKeyIsNoop(t *testing.T) {
	backend, ctx := newBE(t)
	// S3 semantics: idempotent delete.
	assert.NoError(t, backend.Delete(ctx, "does-not-exist"))
}

func TestS3Backend_LockReturnsLockID(t *testing.T) {
	backend, ctx := newBE(t)
	key := "lock-a"

	lockID, err := backend.Lock(ctx, key)
	require.NoError(t, err)
	assert.NotEmpty(t, lockID, "Lock must return a non-empty lockID")
}

func TestS3Backend_ReLockErrors(t *testing.T) {
	backend, ctx := newBE(t)
	key := "lock-b"

	first, err := backend.Lock(ctx, key)
	require.NoError(t, err)

	// A second Lock on the same key must fail.
	second, err := backend.Lock(ctx, key)
	require.Error(t, err, "re-Lock must error")
	assert.Equal(t, "", second, "failed Lock must return empty lockID")

	// Original lockID is unchanged.
	third, err := backend.Lock(ctx, key)
	require.Error(t, err)
	assert.NotEqual(t, first, third)
}

func TestS3Backend_UnlockWrongLockIDErrors(t *testing.T) {
	backend, ctx := newBE(t)
	key := "lock-c"

	lockID, err := backend.Lock(ctx, key)
	require.NoError(t, err)

	err = backend.Unlock(ctx, key, lockID+"-bogus")
	require.Error(t, err, "mismatched lockID must error")

	// Key is still locked: a fresh Lock attempt must fail.
	_, err = backend.Lock(ctx, key)
	require.Error(t, err, "wrong-Unlock must not release the lock")
}

func TestS3Backend_UnlockUnheldKeyErrors(t *testing.T) {
	backend, ctx := newBE(t)
	err := backend.Unlock(ctx, "never-locked", "any-id")
	require.Error(t, err, "Unlock on unheld key must error")
}

func TestS3Backend_UnlockThenRelockWorks(t *testing.T) {
	backend, ctx := newBE(t)
	key := "lock-d"

	first, err := backend.Lock(ctx, key)
	require.NoError(t, err)

	require.NoError(t, backend.Unlock(ctx, key, first))

	// After a correct Unlock, Lock must succeed with a new lockID.
	second, err := backend.Lock(ctx, key)
	require.NoError(t, err)
	assert.NotEqual(t, first, second, "new Lock should mint a fresh lockID")
}

// Compile-time: S3Backend must satisfy the StateBackend interface.
func TestS3Backend_SatisfiesStateBackend(t *testing.T) {
	var _ state.StateBackend = state.NewS3Backend()
}
