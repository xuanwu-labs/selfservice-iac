package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/api/http/middleware"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRateLimitAllowsUnderBudget verifies requests under the limit succeed.
func TestRateLimitAllowsUnderBudget(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             5,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, 200, w.Code, "request %d should pass", i)
	}
}

// TestRateLimitReturns429 verifies that exceeding burst returns 429.
func TestRateLimitReturns429(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 1, // refill 1 token/sec
		Burst:             2,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	// First two consume the burst budget.
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, w2.Code)

	// Third is over budget → 429.
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, 429, w3.Code, "third request must be rate-limited")
}

// TestRateLimitIsolationPerIP verifies different IPs get independent budgets.
func TestRateLimitIsolationPerIP(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	// IP A exhausts its budget.
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "10.0.0.1:1234"
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	require.Equal(t, 200, wA.Code)

	reqA2 := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA2.RemoteAddr = "10.0.0.1:1234"
	wA2 := httptest.NewRecorder()
	r.ServeHTTP(wA2, reqA2)
	assert.Equal(t, 429, wA2.Code, "IP A second request must be limited")

	// IP B has its own fresh budget.
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "10.0.0.2:5678"
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	assert.Equal(t, 200, wB.Code, "IP B first request must pass")
}

// TestRateLimitHonorsXForwardedFor verifies the proxy header is honored.
func TestRateLimitHonorsXForwardedFor(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	mkReq := func(xff string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.9:9999"
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		return req
	}

	// XFF "client-a" — first ok, second limited.
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, mkReq("client-a"))
	require.Equal(t, 200, w1.Code)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, mkReq("client-a"))
	require.Equal(t, 429, w2.Code)

	// XFF "client-b" — independent.
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, mkReq("client-b"))
	assert.Equal(t, 200, w3.Code)
}
