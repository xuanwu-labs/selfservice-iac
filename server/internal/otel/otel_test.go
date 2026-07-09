package otel_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelglobal "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	otelinternal "github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
)

// TestInitSetsGlobalPropagator verifies that Init() installs the W3C
// TraceContext+Baggage propagator globally (D41 "pit #1: propagator must be set").
func TestInitSetsGlobalPropagator(t *testing.T) {
	sdk, err := otelinternal.Init(context.Background(), "aether-test", "test")
	require.NoError(t, err)
	defer func() { _ = sdk.Shutdown(context.Background()) }()

	p := otelglobal.GetTextMapPropagator()
	require.NotNil(t, p)

	// The propagator must understand W3C traceparent headers.
	carrier := propagation.MapCarrier{}
	p.Inject(context.Background(), carrier)

	// If TraceContext is active, injecting into an empty context produces no
	// traceparent (no active span). But the propagator must not panic and
	// must Extract traceparent-formatted values.
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	carrier.Set("traceparent", tp)
	ctx := p.Extract(context.Background(), carrier)
	sc := trace.SpanContextFromContext(ctx)
	assert.True(t, sc.IsValid(), "TraceContext propagator must parse traceparent header")
}

// TestMetricsHandlerServesPrometheus verifies /metrics returns Prometheus
// exposition format (D41: /metrics endpoint).
func TestMetricsHandlerServesPrometheus(t *testing.T) {
	sdk, err := otelinternal.Init(context.Background(), "aether-test", "test")
	require.NoError(t, err)
	defer func() { _ = sdk.Shutdown(context.Background()) }()

	handler := otelinternal.MetricsHandler()
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	// Prometheus exposition format: at least one HELP or TYPE line, or empty-ish.
	// The registry may be sparse at init; accept 200 + valid Prometheus-ish content.
	content := string(body)
	assert.True(t,
		strings.Contains(content, "# HELP") || strings.Contains(content, "# TYPE") || strings.TrimSpace(content) == "",
		"metrics body should be Prometheus format, got: %s", content[:min(200, len(content))],
	)
}

// TestTracerProviderInitialized verifies Init returns a working TracerProvider
// that produces valid spans.
func TestTracerProviderInitialized(t *testing.T) {
	sdk, err := otelinternal.Init(context.Background(), "aether-test", "test")
	require.NoError(t, err)
	defer func() { _ = sdk.Shutdown(context.Background()) }()

	tracer := sdk.TracerProvider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	assert.True(t, span.SpanContext().IsValid(), "span must have valid trace/span id")
	assert.True(t, span.SpanContext().TraceID().IsValid())

	// Propagate the span context and extract on the other side (simulates
	// cross-process propagation via HTTP headers).
	carrier := propagation.MapCarrier{}
	otelglobal.GetTextMapPropagator().Inject(ctx, carrier)
	extractedCtx := otelglobal.GetTextMapPropagator().Extract(context.Background(), carrier)
	extractedSC := trace.SpanContextFromContext(extractedCtx)

	assert.Equal(t, span.SpanContext().TraceID(), extractedSC.TraceID(),
		"extracted trace id must match the original (W3C propagation works)")
}

// TestShutdownIsIdempotent verifies Shutdown can be called multiple times
// without error (defer-safety).
func TestShutdownIsIdempotent(t *testing.T) {
	sdk, err := otelinternal.Init(context.Background(), "aether-test", "test")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, sdk.Shutdown(ctx))
	require.NoError(t, sdk.Shutdown(ctx), "second Shutdown must not error")
}

// TestNoExporterEndpointWorks verifies Init succeeds even when
// OTEL_EXPORTER_OTLP_ENDPOINT is unset (dev convenience: no collector running).
func TestNoExporterEndpointWorks(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	sdk, err := otelinternal.Init(context.Background(), "aether-test", "test")
	require.NoError(t, err, "Init must succeed without a collector endpoint")
	defer func() { _ = sdk.Shutdown(context.Background()) }()
}
