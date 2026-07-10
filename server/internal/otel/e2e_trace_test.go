// Package otel: e2e_trace_test.go — end-to-end trace tree assertion (task 11.7).
//
// Verifies a request's span tree spans gin → pgx with the same trace_id:
// the gin middleware starts a span, the handler runs a pgx query (traced by
// otelpgx), and both spans share one trace. This is the D41 "one trace across
// the stack" guarantee, tested for real against a testcontainers PG.
package otel_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exaring/otelpgx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	otelinternal "github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
)

// TestEndToEndTraceGinToPgx verifies that a gin request + a pgx query inside it
// produce spans sharing the same trace_id (D41).
//
// Pipeline: otelgin (server span) → handler → pool.QueryRow (otelpgx span).
// Both spans are captured by an in-memory span exporter and asserted to share
// a trace, with the pgx span being a child of the gin span.
func TestEndToEndTraceGinToPgx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in -short mode")
	}

	// 1. Set up OTel with an in-memory exporter to capture spans.
	sdk, err := otelinternal.Init(context.Background(), "trace-test", "test", "")
	require.NoError(t, err)
	defer func() { _ = sdk.Shutdown(context.Background()) }()

	// Attach an in-memory exporter to the TracerProvider so we can inspect
	// spans emitted during the request.
	exporter := tracetest.NewInMemoryExporter()
	sdk.TracerProvider.RegisterSpanProcessor(tracesdk.NewSimpleSpanProcessor(exporter))

	// 2. Start a real PG and build a pool WITH otelpgx tracer (testdb.New gives
	// a plain pool; we need the tracer for the pgx span to be emitted).
	dsn := testdb.NewDSN(t)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	require.NoError(t, err)
	defer pool.Close()

	// 3. Build a gin router with otelgin middleware + a handler that runs a query.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(otelgin.Middleware("aether-test"))
	r.GET("/probe", func(c *gin.Context) {
		// Run a pgx query using the request context so the otelpgx span is a
		// child of the gin span.
		var result int
		err := pool.QueryRow(c.Request.Context(), "SELECT 1").Scan(&result)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"result": result})
	})

	// 4. Make a request — gin creates the root server span.
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	// Inject a traceparent so we control the expected trace id.
	expectedTraceID := oteltrace.TraceID{0x01, 0x02, 0x03}
	headers := map[string]string{
		"traceparent": "00-01020300000000000000000000000000-0102030405060708-01",
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "probe request must succeed")

	// 5. Force-flush spans and inspect.
	require.NoError(t, sdk.TracerProvider.ForceFlush(context.Background()))
	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "at least one span must be emitted")

	// Find the gin server span and the pgx query span.
	var ginSpan, pgxSpan *tracetest.SpanStub
	for i := range spans {
		s := spans[i]
		// otelgin emits a SpanKindServer span for the HTTP route.
		if s.SpanKind == oteltrace.SpanKindServer {
			ginSpan = &spans[i]
		}
		// otelpgx emits query spans with db.* attributes.
		if hasDBAttr(s.Attributes) && s.Name != "connect" && s.Name != "pool.acquire" {
			pgxSpan = &spans[i]
		}
	}

	require.NotNil(t, ginSpan, "gin server span must be captured")
	require.NotNil(t, pgxSpan, "pgx query span must be captured")

	// 6. Assert: both spans share the same trace id (the propagated one).
	assert.Equal(t, ginSpan.SpanContext.TraceID(), pgxSpan.SpanContext.TraceID(),
		"gin span and pgx span must share the same trace_id (end-to-end trace)")
	assert.Equal(t, expectedTraceID, ginSpan.SpanContext.TraceID(),
		"trace id must be the one propagated via traceparent header")

	// pgx span should be a child of the gin span (parent span id matches).
	if pgxSpan.Parent.SpanID().IsValid() {
		assert.Equal(t, ginSpan.SpanContext.SpanID(), pgxSpan.Parent.SpanID(),
			"pgx span must be a child of the gin span")
	}
}

// hasDBAttr checks whether the span attributes contain any db.* key
// (otelpgx annotates query spans with db.system, db.statement, etc.).
func hasDBAttr(attrs []attribute.KeyValue) bool {
	for _, a := range attrs {
		if len(a.Key) > 3 && string(a.Key[:3]) == "db." {
			return true
		}
	}
	return false
}

// keep imports referenced (otel global used for propagator side-effects).
var _ = otel.Tracer
