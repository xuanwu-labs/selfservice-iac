package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/xuanwu-labs/selfservice-iac/server/core/queue"
	otelinternal "github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
)

// otelInitForTest initializes a real OTel SDK + in-memory span exporter so
// tests can assert on emitted spans (e.g. worker trace_id == producer trace_id).
func otelInitForTest() (*otelinternal.SDK, *tracetest.InMemoryExporter, error) {
	sdk, err := otelinternal.Init(context.Background(), "queue-test", "test", "")
	if err != nil {
		return nil, nil, err
	}
	exporter := tracetest.NewInMemoryExporter()
	sdk.TracerProvider.RegisterSpanProcessor(tracesdk.NewSimpleSpanProcessor(exporter))
	return sdk, exporter, nil
}

// otelTracer returns the global tracer used by the propagator.
func otelTracer() trace.Tracer { return otel.Tracer("queue-test") }

// setupRiverDB returns a pool with River's schema migrated, plus the pool for
// direct inserts. River needs its own tables (river_job etc.) in addition to
// any app schema.
func setupRiverDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.New(t)
	ctx := context.Background()

	// Run River migrations to create river_job and related tables.
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	require.NoError(t, err)
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{})
	require.NoError(t, err, "river migrations must succeed")
	return pool
}

// waitFor polls until cond returns true or timeout — River processes
// asynchronously so the test must wait for the worker to finish.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting: %s", msg)
}

// TestRiverEnqueueAndProcess verifies the full pipeline: insert a job, the
// registered worker processes it, and the processed counter increments.
func TestRiverEnqueueAndProcess(t *testing.T) {
	pool := setupRiverDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processor := queue.NewExampleProcessor()
	workers := river.NewWorkers()
	river.AddWorker(workers, processor)

	client, err := queue.NewClient(ctx, pool, workers)
	require.NoError(t, err)

	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { _ = client.Stop(ctx) })

	// Insert a job (traced, carrying the current context).
	_, err = queue.TracedInsert(ctx, client, queue.ExampleJob{Message: "hello"}, nil)
	require.NoError(t, err)

	// Wait for the worker to process it.
	waitFor(t, 10*time.Second, func() bool { return processor.ProcessedCount() == 1 }, "job processed")
	assert.Equal(t, int64(1), processor.ProcessedCount(), "exactly one job must be worked")
}

// TestTracedInsertCarriesTraceContext verifies that a span started in the
// producer context is propagated into the worker via job metadata, so the
// worker span shares the producer's trace id (D41 / task 9.3 P0-3).
//
// This is a REAL assertion: it captures the worker span via an in-memory
// exporter and asserts workerSpan.TraceID == producerSpan.TraceID.
func TestTracedInsertCarriesTraceContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in -short mode")
	}
	sdk, exporter, err := otelInitForTest()
	require.NoError(t, err)
	defer func() { _ = sdk.Shutdown(context.Background()) }()

	pool := setupRiverDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processor := queue.NewExampleProcessor()
	workers := river.NewWorkers()
	river.AddWorker(workers, processor)

	client, err := queue.NewClient(ctx, pool, workers)
	require.NoError(t, err)

	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { _ = client.Stop(ctx) })

	// Start a producer span, insert the job inside it — TracedInsert must
	// serialize the producer's trace context into job metadata.
	producerCtx, producerSpan := otelTracer().Start(ctx, "test-producer")
	_, err = queue.TracedInsert(producerCtx, client, queue.ExampleJob{Message: "traced"}, nil)
	require.NoError(t, err)
	producerTraceID := producerSpan.SpanContext().TraceID()
	require.True(t, producerTraceID.IsValid(), "producer span must have a valid trace id")
	producerSpan.End()

	// Wait for the worker to process the job.
	waitFor(t, 15*time.Second, func() bool { return processor.ProcessedCount() == 1 }, "traced job processed")

	// Flush spans and find the worker span (named "river.work.example").
	require.NoError(t, sdk.TracerProvider.ForceFlush(ctx))
	spans := exporter.GetSpans()
	var workerSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "river.work.example" {
			workerSpan = &spans[i]
			break
		}
	}

	// P0-3 assertion: worker span trace_id MUST equal producer span trace_id.
	// This proves traceparent survived: producer ctx → job metadata → worker ctx.
	require.NotNil(t, workerSpan, "worker span 'river.work.example' must be captured")
	assert.Equal(t, producerTraceID, workerSpan.SpanContext.TraceID(),
		"worker span must share the producer's trace_id (trace propagated via job metadata)")
}
