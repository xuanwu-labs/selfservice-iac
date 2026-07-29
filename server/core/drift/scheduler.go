// Package drift: scheduler.go — DriftScheduler (D2 / D5).
//
// Phase 1 embeds the scheduler in the main process: per-layer time.Ticker
// goroutines that, on each tick, list the stacks in that layer and dispatch a
// read-only drift check per stack, throttled by a per-layer token bucket
// (golang.org/x/time/rate, D5 / P2-11). Phase 2 will move this to River jobs
// + leader election (Non-Goal here).
//
// Time is abstracted through clock.Clock (D44 / P3-14): the scheduler never
// calls time.Now directly for timestamps. The ticker itself uses the injected
// per-layer interval (100ms in tests — D5 "intervals injectable").
package drift

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/xuanwu-labs/selfservice-iac/server/core/clock"
)

// Layer names used by the default schedule (D5).
const (
	LayerGlobal      = "global"
	LayerMiddleware  = "middleware"
	LayerApplication = "application"
)

// DefaultIntervals returns the production per-layer drift schedule (D5):
// global stacks (slowest), application stacks (most volatile).
func DefaultIntervals() map[string]time.Duration {
	return map[string]time.Duration{
		LayerGlobal:      24 * time.Hour,
		LayerMiddleware:  12 * time.Hour,
		LayerApplication: 6 * time.Hour,
	}
}

// DefaultConcurrency returns the production per-layer token-bucket sizes (D5):
// global layers have few stacks (2 concurrent), application layers have many
// (10 concurrent).
func DefaultConcurrency() map[string]int {
	return map[string]int{
		LayerGlobal:      2,
		LayerMiddleware:  5,
		LayerApplication: 10,
	}
}

// StackChecker runs a single read-only drift check for one stack. The
// scheduler dispatches CheckStack calls throttled by the per-layer limiter.
// It is satisfied by *Worker (via a thin adapter) in production wiring.
type StackChecker interface {
	CheckStack(ctx context.Context, stackID int64) error
}

// StackLister enumerates the stack IDs that belong to a given layer. The
// scheduler needs this to know what to dispatch on each tick.
type StackLister interface {
	ListStacks(ctx context.Context, layer string) ([]int64, error)
}

// Scheduler is the Phase 1 drift scheduler (D2).
type Scheduler struct {
	intervals   map[string]time.Duration
	concurrency map[string]int
	limiters    map[string]*rate.Limiter
	clock       clock.Clock
	checker     StackChecker
	lister      StackLister

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
}

// NewScheduler constructs a Scheduler. intervals and concurrency are keyed by
// layer; each layer must be present in both maps. clk timestamps ticks (never
// time.Now). checker runs per-stack drift checks; lister enumerates stacks per
// layer.
func NewScheduler(
	intervals map[string]time.Duration,
	concurrency map[string]int,
	clk clock.Clock,
	lister StackLister,
	checker StackChecker,
) *Scheduler {
	limiters := make(map[string]*rate.Limiter, len(concurrency))
	for layer, n := range concurrency {
		// rate.Limiter with burst n and refill rate n/sec => at most n
		// concurrent in-flight checks per layer (token bucket, D5).
		limiters[layer] = rate.NewLimiter(rate.Limit(n), n)
	}
	return &Scheduler{
		intervals:   intervals,
		concurrency: concurrency,
		limiters:    limiters,
		clock:       clk,
		checker:     checker,
		lister:      lister,
	}
}

// Start launches one ticker goroutine per layer. It is idempotent: a second
// call is a no-op. The goroutines exit when ctx (or Stop's cancellation)
// fires.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if s.clock == nil {
		return fmt.Errorf("drift: scheduler requires a clock")
	}
	if s.checker == nil {
		return fmt.Errorf("drift: scheduler requires a StackChecker")
	}
	if s.lister == nil {
		return fmt.Errorf("drift: scheduler requires a StackLister")
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.started = true

	for layer, interval := range s.intervals {
		s.wg.Add(1)
		go s.runLayer(runCtx, layer, interval)
	}
	return nil
}

// Stop signals every layer goroutine to drain and waits for them to exit. It
// is idempotent and safe to call multiple times.
func (s *Scheduler) Stop(_ context.Context) {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	s.wg.Wait()

	s.mu.Lock()
	s.started = false
	s.cancel = nil
	s.mu.Unlock()
}

// runLayer runs a single layer's ticker loop. On each tick it lists the
// layer's stacks and dispatches a throttled CheckStack per stack.
func (s *Scheduler) runLayer(ctx context.Context, layer string, interval time.Duration) {
	defer s.wg.Done()

	// Use time.Ticker for cadence; clock.Clock is used for any timestamping
	// the scheduler emits (D44: never time.Now for domain timestamps).
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	limiter := s.limiters[layer]
	if limiter == nil {
		// No limiter configured for this layer: treat as unlimited.
		limiter = rate.NewLimiter(rate.Inf, 0)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx, layer, limiter)
		}
	}
}

// tick lists the layer's stacks and dispatches checks. Acquiring a token from
// limiter throttles concurrency; the check itself runs in a goroutine bounded
// by the limiter (Wait blocks until a token is available).
func (s *Scheduler) tick(ctx context.Context, layer string, limiter *rate.Limiter) {
	stackIDs, err := s.lister.ListStacks(ctx, layer)
	if err != nil {
		// A listing failure should not kill the scheduler; the next tick
		// retries. Phase 2 will surface this via metrics.
		return
	}
	for _, id := range stackIDs {
		stackID := id // capture for goroutine
		if waitErr := limiter.Wait(ctx); waitErr != nil {
			// Context cancelled while waiting for a token: stop dispatching.
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			// Best-effort: a per-stack error does not stop other stacks.
			_ = s.checker.CheckStack(ctx, stackID)
		}()
	}
}
