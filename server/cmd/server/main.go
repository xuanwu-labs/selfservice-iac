// Package main is the Aether platform server entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	// OTel SDK must be initialized BEFORE wire, because pgxpool/gin read the
	// global TracerProvider + propagator at construction time (D41).
	otelSDK, err := otel.Init(ctx, "aether-server", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init otel: %v\n", err)
		os.Exit(1)
	}

	// Wire-generated initialization (compile-time DI)
	app, cleanup, err := InitializeApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// Wrap zap logger with otelzap so log entries carry trace_id.
	logger := otel.WrapLogger(app.Logger)

	logger.Info("Aether platform starting",
		zap.String("http_addr", app.Config.HTTPAddr),
	)

	// Root mux: Connect-RPC under /api/, gin for everything else.
	// One process, one port (task 15.6).
	mux := http.NewServeMux()
	if app.ConnectMux != nil {
		mux.Handle(app.ConnectMux.Path, app.ConnectMux.Mux)
	}
	mux.Handle("/", app.Router)

	srv := &http.Server{
		Addr:    app.Config.HTTPAddr,
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", zap.Error(err))
	}
	// Flush pending spans before exit (D41: tp.Shutdown is the "one of seven pits" must-do).
	if err := otelSDK.Shutdown(shutdownCtx); err != nil {
		logger.Error("otel shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
}
