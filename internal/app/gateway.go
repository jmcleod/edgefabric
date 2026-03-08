package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// RunGateway starts the gateway process.
func RunGateway(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric gateway",
		slog.String("controller_addr", cfg.Gateway.ControllerAddr),
	)

	// TODO: Connect to controller over WireGuard.
	// TODO: Start route forwarding.

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("shutting down gateway")
	return nil
}
