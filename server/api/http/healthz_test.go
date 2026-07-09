package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
)

func setupTestRouter(pingFn func(ctx context.Context) error) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(&Deps{
		Logger:      otelzap.New(zap.NewNop()),
		PingFunc:    pingFn,
		Middlewares: []gin.HandlerFunc{middleware.RequestID()},
	})
}

func TestHealthzOK(t *testing.T) {
	router := setupTestRouter(func(ctx context.Context) error { return nil })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestHealthzDBDown(t *testing.T) {
	router := setupTestRouter(func(ctx context.Context) error { return fmt.Errorf("connection refused") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "unhealthy")
}

func TestHealthzNoPingFunc(t *testing.T) {
	router := setupTestRouter(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestIDMiddleware(t *testing.T) {
	router := setupTestRouter(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Request-Id", "test-req-123")
	router.ServeHTTP(w, req)

	assert.Equal(t, "test-req-123", w.Header().Get("X-Request-Id"))
}
