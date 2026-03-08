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

// RunNode starts the node process.
func RunNode(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric node",
		slog.String("controller_addr", cfg.Node.ControllerAddr),
	)

	// TODO: Connect to controller over WireGuard.
	// TODO: Start BGP, DNS, CDN, Route services as configured.

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("shutting down node")
	return nil
}
