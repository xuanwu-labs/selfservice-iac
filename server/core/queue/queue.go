// Package queue wraps riverqueue/river for async job processing (D39).
//
// Two responsibilities:
//   - Provide River client(s) sharing the pgxpool, with a pre-split API/worker
//     pool signature so connection isolation can be added later without
//     rewriting callers (task 9.1).
//   - Trace propagation: jobs carry their producer's trace context in job
//     metadata, and workers extract it so the worker span is a child of the
//     enqueue span (task 9.3, P0-3).
package queue

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewClient creates a River client bound to the given pool. Workers must be
// added to the returned client via AddWorkers before Start.
//
// Phase 1: API inserts and worker processing share one pool. The signature
// takes the pool explicitly (rather than a config struct) so a future split
// into NewAPIPool/NewWorkerPool is a drop-in change at the wire layer.
func NewClient(ctx context.Context, pool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	if workers == nil {
		workers = river.NewWorkers()
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		// River requires explicit queue configuration to start working. The
		// "default" queue is the standard target for Insert without a Queue opt.
		Queues: map[string]river.QueueConfig{
			"default": {MaxWorkers: 10},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return client, nil
}

// NewAPIPool returns the pool used for job inserts (Phase 1: same pool as
// workers). Wire may later substitute a dedicated pool without changing the
// insert call sites.
func NewAPIPool(pool *pgxpool.Pool) *pgxpool.Pool { return pool }

// NewWorkerPool returns the pool used by workers (Phase 1: same pool as API).
func NewWorkerPool(pool *pgxpool.Pool) *pgxpool.Pool { return pool }
