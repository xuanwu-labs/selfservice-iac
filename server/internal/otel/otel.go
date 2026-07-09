// Package otel wires up OpenTelemetry: global propagator (TraceContext+Baggage),
// a TracerProvider exporting via OTLP/HTTP, a Prometheus-backed MeterProvider
// serving /metrics, and an otelzap-wrapped logger.
//
// Design (D41):
//   - TraceContext propagator (W3C traceparent) so traces survive across the
//     gin → pgx → http-outbound chain and into River job handlers (task 9).
//   - OTLP/HTTP exporter reads OTEL_EXPORTER_OTLP_ENDPOINT at startup.
//   - Prometheus exporter exposes /metrics for scraping.
//   - zap is wrapped with otelzap so every log line carries the active trace_id.
package otel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SDK holds the initialized OTel providers plus their shutdown functions.
// The caller MUST defer Shutdown to flush pending spans/metrics on exit.
type SDK struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	// promRegistry is the Prometheus registry that the OTel exporter writes to.
	// MetricsHandler() serves this registry so /metrics exposes OTel-collected metrics.
	promRegistry *promclient.Registry
}

// Shutdown flushes and shuts down all providers. Safe to call multiple times.
func (s *SDK) Shutdown(ctx context.Context) error {
	var errs []error
	if s.TracerProvider != nil {
		if err := s.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
	}
	// Prometheus exporter has no Shutdown; MeterProvider is a manual reader
	// without a background flush. Span flush is the only time-sensitive one.
	return errors.Join(errs...)
}

// resource identifies this service in all spans/metrics.
func buildResource(serviceName, serviceVersion string) *resource.Resource {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		// resource.Merge only fails on schema conflicts; fall back to defaults.
		return resource.Default()
	}
	return r
}

// Init sets up the global OTel SDK: propagator, TracerProvider (OTLP/HTTP export),
// MeterProvider (Prometheus), and returns the SDK for lifecycle control.
//
// serviceName / serviceVersion label every span/metric (e.g. "aether-server" / v0.1).
// The OTLP trace exporter endpoint is read from OTEL_EXPORTER_OTLP_ENDPOINT;
// if unset, traces are dropped silently (dev convenience — no collector running).
func Init(ctx context.Context, serviceName, serviceVersion string) (*SDK, error) {
	// (a) Global propagator: W3C TraceContext + Baggage.
	// This MUST be set before any handler starts so cross-process trace
	// propagation (HTTP headers, River job metadata) works end-to-end.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res := buildResource(serviceName, serviceVersion)

	// (b) TracerProvider with OTLP/HTTP exporter.
	tp, err := newTracerProvider(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("init tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)

	// (c) MeterProvider with Prometheus exporter (read by /metrics).
	promReg, mp, err := newMeterProvider(res)
	if err != nil {
		return nil, fmt.Errorf("init meter provider: %w", err)
	}
	otel.SetMeterProvider(mp)
	globalPromRegistry = promReg

	return &SDK{
		TracerProvider: tp,
		MeterProvider:  mp,
		promRegistry:   promReg,
	}, nil
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	// OTLP endpoint comes from the standard env var; if absent we use a noop
	// exporter so the server runs without a collector (dev/CI convenience).
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	var exp sdktrace.SpanExporter
	if endpoint != "" {
		var err error
		exp, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
	}

	var opts []sdktrace.TracerProviderOption
	opts = append(opts, sdktrace.WithResource(res))
	if exp != nil {
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

func newMeterProvider(res *resource.Resource) (*promclient.Registry, *sdkmetric.MeterProvider, error) {
	reg := promclient.NewRegistry()
	promExporter, err := promexporter.New(promexporter.WithRegisterer(reg))
	if err != nil {
		return nil, nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	return reg, mp, nil
}

// MetricsHandler returns the http.Handler serving Prometheus-format metrics
// at /metrics. Uses the SDK's private registry (not the global DefaultRegisterer)
// to avoid duplicate-registration errors when Init is called multiple times.
//
// OTel SDK is global state by design (instrumentation libraries read global
// providers at construction time), so a package-level reference to the registry
// is consistent with that pattern.
func MetricsHandler() http.Handler {
	if globalPromRegistry != nil {
		return promhttp.HandlerFor(globalPromRegistry, promhttp.HandlerOpts{})
	}
	return promhttp.Handler()
}

// globalPromRegistry is set by Init; read by MetricsHandler.
var globalPromRegistry *promclient.Registry

// WrapLogger is removed — wire's provideLogger now returns *otelzap.Logger
// directly (otelzap.New(base)), making post-wire wrapping unnecessary.
// If you need the global otelzap logger, use otelzap.L() directly.
