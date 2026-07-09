// Package queue: traced.go — OpenTelemetry trace propagation across the
// job queue boundary (D39 / task 9.3, P0-3).
//
// Problem: a job is enqueued in one context (e.g. an HTTP request handler
// with an active trace span) and worked in another (a worker goroutine with a
// fresh context). Without propagation, the worker span starts a new trace —
// the gin→pgx→insert→work chain breaks.
//
// Solution: TracedInsert serializes the active trace context (W3C traceparent)
// into the job's Metadata; TracedWorker extracts it on the worker side and
// starts the work span as a child of the original trace. All job handlers MUST
// embed TracedWorker (or implement equivalent extraction).
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// metadataKey is the JSON key under which the serialized trace context lives
// inside the job's Metadata blob.
const metadataKey = "traceparent"

// traceCarrier adapts a map[string]string to otel's TextMapCarrier/Setter so
// the W3C propagator can inject/extract into job metadata.
type traceCarrier map[string]string

func (c traceCarrier) Get(key string) string { return c[key] }

func (c traceCarrier) Set(key, value string) { c[key] = value }

func (c traceCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}

// TracedInsert inserts a job carrying the current trace context in its
// Metadata, so the worker that later picks it up can resume the trace.
//
// Use this instead of client.Insert when you want trace continuity (always,
// per D41). The opts.Metadata field is preserved and merged with the trace
// data.
func TracedInsert[T river.JobArgs](ctx context.Context, client *river.Client[pgx.Tx], args T, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	carrier := traceCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	tp := carrier[metadataKey]
	if tp == "" {
		// No active span — insert without trace metadata (still valid).
		return client.Insert(ctx, args, opts)
	}

	// Merge traceparent into existing metadata (if any).
	meta := loadMetadata(opts)
	meta[metadataKey] = tp
	mergedOpts := withMetadata(opts, meta)
	return client.Insert(ctx, args, mergedOpts)
}

// extractTraceContext rebuilds a context carrying the trace context stored in
// the job's metadata. Returns the original ctx if no trace context is found.
func extractTraceContext(ctx context.Context, job *rivertype.JobRow) context.Context {
	if len(job.Metadata) == 0 {
		return ctx
	}
	var raw map[string]any
	if err := json.Unmarshal(job.Metadata, &raw); err != nil {
		return ctx
	}
	tpAny, ok := raw[metadataKey]
	if !ok {
		return ctx
	}
	tp, ok := tpAny.(string)
	if !ok || tp == "" {
		return ctx
	}
	carrier := traceCarrier{metadataKey: tp}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// TracedWorker is a base that job workers embed to get trace-context
// extraction for free. Embed it in your concrete worker and it handles the
// Start span; your Work method runs inside the resumed trace.
//
//	type MyWorker struct {
//	    queue.TracedWorker
//	}
type TracedWorker struct {
	tracer trace.Tracer
}

// NewTracedWorker returns a TracedWorker using a tracer named after the job
// kind. Workers typically embed a zero TracedWorker and call these methods.
func NewTracedWorker(name string) TracedWorker {
	return TracedWorker{tracer: otel.Tracer(name)}
}

// StartSpan extracts the producer's trace context from the job and starts a
// child span for the work. Returns the span-ended context. Call at the top of
// Work and defer span.End().
func (w TracedWorker) StartSpan(ctx context.Context, job *rivertype.JobRow) (context.Context, trace.Span) {
	ctx = extractTraceContext(ctx, job)
	tracer := w.tracer
	if tracer == nil {
		tracer = otel.Tracer("river-worker")
	}
	spanName := fmt.Sprintf("river.work.%s", job.Kind)
	return tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindConsumer))
}

// --- metadata helpers ---

func loadMetadata(opts *river.InsertOpts) map[string]any {
	if opts == nil || len(opts.Metadata) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(opts.Metadata, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func withMetadata(opts *river.InsertOpts, meta map[string]any) *river.InsertOpts {
	if opts == nil {
		opts = &river.InsertOpts{}
	}
	buf, err := json.Marshal(meta)
	if err != nil {
		return opts
	}
	merged := *opts
	merged.Metadata = buf
	return &merged
}
