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

	"github.com/jmcleod/edgefabric/internal/api"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
	"github.com/jmcleod/edgefabric/web"
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

	// Initialize storage.
	store, err := sqlite.New(cfg.Controller.Storage.DSN)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrate storage: %w", err)
	}

	// Register storage health check.
	health.Register(observability.HealthCheck{
		Name: "storage",
		Check: func(ctx context.Context) error {
			return store.Ping(ctx)
		},
	})

	// Initialize secrets store.
	secretStore, err := secrets.NewStore(cfg.Controller.Secrets.EncryptionKey)
	if err != nil {
		return fmt.Errorf("init secrets: %w", err)
	}

	// Initialize services.
	authSvc := auth.NewService(store, store, secretStore, "EdgeFabric")
	tokenSvc := auth.NewTokenService(
		[]byte(cfg.Controller.Secrets.EncryptionKey), // Derive signing key from encryption key.
		24*time.Hour,
	)
	tenantSvc := tenant.NewService(store)
	userSvc := user.NewService(store, authSvc)
	authorizer := rbac.NewAuthorizer()
	auditLog := audit.NewLogger(store, logger)

	// Seed superuser on first boot.
	if err := SeedSuperUser(context.Background(), userSvc, store, logger); err != nil {
		return fmt.Errorf("seed superuser: %w", err)
	}

	// Assemble API router.
	handler := api.NewRouter(api.Services{
		AuthSvc:    authSvc,
		TokenSvc:   tokenSvc,
		TenantSvc:  tenantSvc,
		UserSvc:    userSvc,
		Authorizer: authorizer,
		AuditLog:   auditLog,
		APIKeys:    store,
		Health:     health,
		Metrics:    metrics,
		Logger:     logger,
		StaticFS:   web.StaticFiles,
	})

	srv := &http.Server{
		Addr:         cfg.Controller.ListenAddr,
		Handler:      handler,
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
