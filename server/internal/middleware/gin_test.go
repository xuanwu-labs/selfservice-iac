package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRateLimitReturns429 verifies that exceeding burst returns 429.
func TestRateLimitReturns429(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             2,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, w2.Code)

	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, 429, w3.Code)
}

// TestRateLimitIsolationPerIP verifies different IPs get independent budgets.
func TestRateLimitIsolationPerIP(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(200) })

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "10.0.0.1:1234"
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	require.Equal(t, 200, wA.Code)

	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "10.0.0.2:5678"
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	assert.Equal(t, 200, wB.Code, "IP B must have its own budget")
}
