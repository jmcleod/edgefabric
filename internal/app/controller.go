// Package app contains the application wiring for each binary mode.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// RunController starts the controller process.
func RunController(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric controller",
		slog.String("listen_addr", cfg.Controller.ListenAddr),
		slog.String("storage_driver", cfg.Controller.Storage.Driver),
	)

	// Initialize observability.
	metrics := observability.NewMetrics()
	health := observability.NewHealthChecker()

	// TODO: Initialize storage.
	// TODO: Initialize services.
	// TODO: Initialize API router.

	// Build HTTP mux.
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.Handler())
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","message":"EdgeFabric Controller API v1"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>EdgeFabric</title></head><body><h1>EdgeFabric Controller</h1><p>API: <a href="/api/v1/">/api/v1/</a></p></body></html>`)
	})

	srv := &http.Server{
		Addr:         cfg.Controller.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", slog.String("addr", cfg.Controller.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down controller")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
