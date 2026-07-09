// Package middleware provides gin HTTP middleware for the Aether server.
// Currently: CORS (request_id and recovery are wired in api/http/http.go).
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
)

// CORS returns a gin middleware that applies a permissive CORS policy for
// browser clients (the React frontend in web/). AllowedOrigins defaults to
// the explicit list; pass nil to allow the local dev origin.
//
// This wraps rs/cors so the standard CORS preflight/headers handling is reused
// rather than reimplemented.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300, // seconds — preflight cache
	})
	return func(ctx *gin.Context) {
		c.HandlerFunc(ctx.Writer, ctx.Request)
		// If cors short-circuited a preflight (OPTIONS), abort the chain.
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
		}
	}
}
