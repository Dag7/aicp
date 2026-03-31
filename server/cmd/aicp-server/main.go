// Package main is the entrypoint for the AICP protocol server.
// It wires together all protocol layers, starts the HTTP server
// and metrics endpoint, and handles graceful shutdown.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aicp-protocol/aicp/server/internal/config"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server terminated with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize structured logger.
	logger := newLogger(cfg.Server.LogLevel, cfg.Server.LogFormat)
	slog.SetDefault(logger)

	slog.Info("starting AICP server",
		"version", aicp.Version,
		"commit", aicp.Commit,
		"protocol", aicp.ProtocolVersion,
		"addr", cfg.Server.Addr(),
		"metrics_addr", cfg.Server.MetricsAddr(),
	)

	// Create root context that cancels on OS signals.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// TODO: Initialize PostgreSQL connection pool.
	// TODO: Initialize Redis client.
	// TODO: Initialize ledger backend.
	// TODO: Initialize identity layer.
	// TODO: Initialize discovery layer.
	// TODO: Initialize commerce layer (payment adapters).
	// TODO: Initialize onboarding layer.
	// TODO: Initialize event system.
	// TODO: Build chi router with all handlers and middleware.

	// Placeholder health handler until the full router is wired.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","version":"%s","protocol":"%s"}`, aicp.Version, aicp.ProtocolVersion)
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Check PG and Redis connectivity.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ready"}`)
	})

	// Main HTTP server.
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Metrics server (separate port for Prometheus scraping).
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:    cfg.Server.MetricsAddr(),
		Handler: metricsMux,
	}

	// Channel to collect server errors.
	errCh := make(chan error, 2)

	// Start metrics server.
	go func() {
		slog.Info("metrics server listening", "addr", cfg.Server.MetricsAddr())
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	// Start main HTTP server.
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.Server.Addr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining connections...")
	case err := <-errCh:
		return err
	}

	// Graceful shutdown with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown both servers concurrently.
	shutdownErrCh := make(chan error, 2)
	go func() { shutdownErrCh <- srv.Shutdown(shutdownCtx) }()
	go func() { shutdownErrCh <- metricsSrv.Shutdown(shutdownCtx) }()

	for range 2 {
		if err := <-shutdownErrCh; err != nil {
			slog.Error("error during shutdown", "error", err)
		}
	}

	// TODO: Close PostgreSQL pool.
	// TODO: Close Redis client.
	// TODO: Stop ledger consensus engine.
	// TODO: Stop event system.

	slog.Info("server stopped gracefully")
	return nil
}

// newLogger creates a structured slog.Logger based on config.
func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: lvl == slog.LevelDebug,
	}

	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
