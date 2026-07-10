// Package main is the Aether platform server entry point.
// Thin: OTel init → wire → server.Run → graceful shutdown.
// All transport assembly (gin + Connect + mux) lives in internal/server/.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	// Load config BEFORE wire — OTel needs the collector endpoint, and wire
	// providers (pgxpool/gin) read the global TracerProvider at construction.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// OTel SDK must be initialized BEFORE wire (D41).
	// Endpoint from config: empty = noop (dev convenience), set = push to collector.
	otelSDK, err := otel.Init(ctx, cfg.OTel.ServiceName, "0.1.0", cfg.OTel.Endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init otel: %v\n", err)
		os.Exit(1)
	}

	// Wire-generated initialization (compile-time DI).
	// OTel was initialized above, so wire's provideLogger already returns a
	// trace-aware *otelzap.Logger — no post-wire wrapping needed (D41).
	app, cleanup, err := InitializeApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	logger := app.Logger
	logger.Info("Aether platform starting",
		zap.String("http_addr", app.Config.HTTPAddr),
		zap.Bool("connect_enabled", app.Config.Connect.Enabled),
	)

	// Start HTTP server (gin + Connect on one port).
	errCh := make(chan error, 1)
	go func() {
		if err := app.Server.Run(); err != nil {
			errCh <- err
		}
	}()

	// Wait for signal or fatal server error.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		// graceful shutdown
	case err := <-errCh:
		logger.Error("server failed", zap.Error(err))
	}

	// Graceful shutdown.
	logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server (stops accepting new connections, waits for in-flight).
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", zap.Error(err))
	}
	// Flush pending spans before exit (D41: tp.Shutdown is a must-do).
	if err := otelSDK.Shutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
}
