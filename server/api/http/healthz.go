package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthzHandler checks DB connectivity and returns 200 (healthy) or 503 (unhealthy).
// The request context is propagated to PingFunc so the pgx span is a child of
// the gin span (D41 end-to-end trace).
func HealthzHandler(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.PingFunc != nil {
			if err := deps.PingFunc(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "unhealthy",
					"error":  err.Error(),
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}
