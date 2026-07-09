// Package queue: example_worker.go — a minimal job + worker that demonstrates
// the enqueue-then-process pattern with trace propagation (task 9.2).
//
// ExampleJob is intentionally trivial (records a processed count); it exists
// to verify the full River + trace pipeline works end to end in tests.
package queue

import (
	"context"
	"sync/atomic"

	"github.com/riverqueue/river"
)

// ExampleJob is a sample job args struct. The Kind string is the stable job
// identifier (survives type renames).
type ExampleJob struct {
	Message string `json:"message"`
}

func (ExampleJob) Kind() string { return "example" }

// ExampleProcessor is a singleton that records how many ExampleJobs have been
// worked. Tests inspect ProcessedCount to assert the worker ran.
type ExampleProcessor struct {
	// river.WorkerDefaults satisfies Middleware/NextRetry boilerplate.
	river.WorkerDefaults[ExampleJob]

	// TracedWorker gives us trace extraction (task 9.3).
	TracedWorker

	count atomic.Int64
}

// NewExampleProcessor returns a processor ready to register with River workers.
func NewExampleProcessor() *ExampleProcessor {
	return &ExampleProcessor{
		TracedWorker: NewTracedWorker("example-worker"),
	}
}

// ProcessedCount returns the number of jobs this processor has completed.
func (p *ExampleProcessor) ProcessedCount() int64 { return p.count.Load() }

// Work fulfills river.Worker[ExampleJob]. It starts a trace span (resumed from
// the producer), records the message, and increments the processed counter.
func (p *ExampleProcessor) Work(ctx context.Context, job *river.Job[ExampleJob]) error {
	// ctx carries the resumed trace context — real workers pass it to DB/cloud
	// calls so their spans are children of the producer's trace.
	workCtx, span := p.StartSpan(ctx, job.JobRow)
	defer span.End()
	_ = workCtx //骨架: no downstream call yet; real workers use workCtx.

	// In a real worker this is where the side effect lives (codegen, apply,
	// drift scan, send-webhook). Here we just count.
	_ = job.Args.Message
	p.count.Add(1)
	return nil
}
