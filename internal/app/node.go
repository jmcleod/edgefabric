package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmcleod/edgefabric/internal/bgp"
	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// RunNode starts the node process.
func RunNode(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric node",
		slog.String("controller_addr", cfg.Node.ControllerAddr),
		slog.Bool("bgp_enabled", cfg.Node.BGP.Enabled),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize BGP service if enabled.
	var bgpSvc bgp.Service
	if cfg.Node.BGP.Enabled {
		bgpSvc = initBGPService(cfg.Node.BGP, logger)
		if bgpSvc != nil {
			routerID := cfg.Node.BGP.RouterID
			if routerID == "" {
				routerID = "127.0.0.1" // Fallback; real setup uses node's WireGuard IP.
			}
			localASN := cfg.Node.BGP.LocalASN
			if localASN == 0 {
				localASN = 65000 // Default private ASN.
			}

			if err := bgpSvc.Start(ctx, routerID, localASN); err != nil {
				logger.Error("failed to start BGP service", slog.String("error", err.Error()))
			} else {
				logger.Info("BGP service started",
					slog.String("router_id", routerID),
					slog.Uint64("local_asn", uint64(localASN)),
					slog.String("mode", cfg.Node.BGP.Mode),
				)

				// Start BGP reconciliation loop.
				go bgpReconcileLoop(ctx, bgpSvc, cfg.Node.ControllerAddr, logger)
			}
		}
	}

	// TODO: Connect to controller over WireGuard.
	// TODO: Start DNS, CDN, Route services as configured.

	<-ctx.Done()
	logger.Info("shutting down node")

	// Graceful shutdown of BGP.
	if bgpSvc != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := bgpSvc.Stop(shutdownCtx); err != nil {
			logger.Error("failed to stop BGP service", slog.String("error", err.Error()))
		} else {
			logger.Info("BGP service stopped")
		}
	}

	return nil
}

// initBGPService creates the appropriate BGP service based on config mode.
func initBGPService(cfg config.BGPConfig, logger *slog.Logger) bgp.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	switch mode {
	case "gobgp":
		logger.Info("using GoBGP BGP service")
		return bgp.NewGoBGPService()
	case "noop":
		logger.Info("using noop BGP service (demo mode)")
		return bgp.NewNoopService()
	default:
		logger.Error("unknown BGP mode, falling back to noop", slog.String("mode", mode))
		return bgp.NewNoopService()
	}
}

// bgpReconcileLoop periodically polls the controller for desired BGP state
// and reconciles the local BGP service to match. Runs every 30 seconds.
func bgpReconcileLoop(ctx context.Context, svc bgp.Service, controllerAddr string, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: Poll GET /api/v1/nodes/{id}/config/bgp from controller
			// and call svc.Reconcile(ctx, sessions).
			// For now, just log that reconciliation would happen.
			logger.Debug("BGP reconciliation tick",
				slog.String("controller", controllerAddr),
			)
		}
	}
}
