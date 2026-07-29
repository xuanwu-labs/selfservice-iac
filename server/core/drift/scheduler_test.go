package drift_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/clock"
	"github.com/xuanwu-labs/selfservice-iac/server/core/drift"
)

// fakeChecker is a StackChecker stub that blocks until released, letting the
// test observe in-flight concurrency, and counts calls.
type fakeChecker struct {
	mu           sync.Mutex
	release      chan struct{}
	inflight     int32
	peakInflight int32
	calls        int64
	errors       int64
	started      chan struct{}
}

func newBlockingChecker() *fakeChecker {
	return &fakeChecker{
		release: make(chan struct{}),
		started: make(chan struct{}, 1024),
	}
}

func (f *fakeChecker) CheckStack(ctx context.Context, _ int64) error {
	atomic.AddInt64(&f.calls, 1)

	// Signal we have started a check (best-effort, non-blocking).
	select {
	case f.started <- struct{}{}:
	default:
	}

	cur := atomic.AddInt32(&f.inflight, 1)
	for {
		peak := atomic.LoadInt32(&f.peakInflight)
		if cur <= peak || atomic.CompareAndSwapInt32(&f.peakInflight, peak, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inflight, -1)

	select {
	case <-f.release:
		return nil
	case <-ctx.Done():
		atomic.AddInt64(&f.errors, 1)
		return ctx.Err()
	}
}

// fakeLister enumerates stacks by layer from a static map.
type fakeLister struct {
	stacks map[string][]int64
}

func (f *fakeLister) ListStacks(_ context.Context, layer string) ([]int64, error) {
	return f.stacks[layer], nil
}

func TestScheduler_StartAndTickDispatchesChecks(t *testing.T) {
	// 1 layer, 100ms interval, capacity 2, 1 stack -> at least one tick should
	// fire within ~500ms and the checker should be invoked.
	checker := newBlockingChecker()
	sched := drift.NewScheduler(
		map[string]time.Duration{"app": 100 * time.Millisecond},
		map[string]int{"app": 2},
		clock.New(),
		&fakeLister{stacks: map[string][]int64{"app": {1}}},
		checker,
	)

	ctx := context.Background()
	require.NoError(t, sched.Start(ctx))

	// Wait until at least one CheckStack starts.
	select {
	case <-checker.started:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not dispatch any check within 2s")
	}

	// Unblock + drain.
	close(checker.release)
	sched.Stop(ctx)

	assert.GreaterOrEqual(t, atomic.LoadInt64(&checker.calls), int64(1),
		"at least one check must have been dispatched")
}

func TestScheduler_RateLimiterBlocksExcess(t *testing.T) {
	// capacity 2, 4 stacks, blocking checker: peak in-flight must NOT exceed 2.
	checker := newBlockingChecker()
	sched := drift.NewScheduler(
		map[string]time.Duration{"app": 50 * time.Millisecond},
		map[string]int{"app": 2},
		clock.New(),
		&fakeLister{stacks: map[string][]int64{"app": {1, 2, 3, 4}}},
		checker,
	)

	ctx := context.Background()
	require.NoError(t, sched.Start(ctx))

	// Wait for two checks to start (the limiter's burst capacity).
	<-checker.started
	<-checker.started

	// Give the scheduler a beat to attempt a third dispatch that the limiter
	// should be blocking on.
	time.Sleep(100 * time.Millisecond)

	peak := atomic.LoadInt32(&checker.peakInflight)
	assert.LessOrEqual(t, peak, int32(2),
		"peak in-flight checks (%d) must not exceed limiter capacity (2)", peak)

	// Release everything and stop.
	close(checker.release)
	sched.Stop(ctx)
}

func TestScheduler_StopIsIdempotent(t *testing.T) {
	checker := newBlockingChecker()
	sched := drift.NewScheduler(
		map[string]time.Duration{"app": 10 * time.Second},
		map[string]int{"app": 1},
		clock.New(),
		&fakeLister{stacks: map[string][]int64{"app": {1}}},
		checker,
	)
	ctx := context.Background()
	require.NoError(t, sched.Start(ctx))
	close(checker.release)

	// Multiple Stop calls must not panic.
	sched.Stop(ctx)
	sched.Stop(ctx)
	sched.Stop(ctx)
}

func TestScheduler_StartTwiceIsNoop(t *testing.T) {
	checker := newBlockingChecker()
	sched := drift.NewScheduler(
		map[string]time.Duration{"app": 1 * time.Second},
		map[string]int{"app": 1},
		clock.New(),
		&fakeLister{stacks: map[string][]int64{"app": {1}}},
		checker,
	)
	ctx := context.Background()
	require.NoError(t, sched.Start(ctx))
	require.NoError(t, sched.Start(ctx), "second Start must be a no-op, not an error")
	close(checker.release)
	sched.Stop(ctx)
}

func TestScheduler_StartRejectsMissingDeps(t *testing.T) {
	err := drift.NewScheduler(
		map[string]time.Duration{"app": 100 * time.Millisecond},
		map[string]int{"app": 1},
		nil, // no clock
		&fakeLister{stacks: map[string][]int64{"app": {1}}},
		newBlockingChecker(),
	).Start(context.Background())
	require.Error(t, err)
}

func TestDefaultIntervalsAndConcurrency(t *testing.T) {
	intervals := drift.DefaultIntervals()
	assert.Equal(t, 24*time.Hour, intervals["global"])
	assert.Equal(t, 12*time.Hour, intervals["middleware"])
	assert.Equal(t, 6*time.Hour, intervals["application"])

	conc := drift.DefaultConcurrency()
	assert.Equal(t, 2, conc["global"])
	assert.Equal(t, 5, conc["middleware"])
	assert.Equal(t, 10, conc["application"])
}
