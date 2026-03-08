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
	"github.com/jmcleod/edgefabric/internal/gatewayrt"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// RunGateway starts the gateway process.
func RunGateway(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric gateway",
		slog.String("controller_addr", cfg.Gateway.ControllerAddr),
		slog.String("wireguard_ip", cfg.Gateway.WireGuardIP),
		slog.String("route_mode", cfg.Gateway.RouteMode),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO: Connect to controller over WireGuard.

	// Initialize gateway route forwarding service.
	gwRouteSvc := initGatewayRouteService(cfg.Gateway, logger)
	if gwRouteSvc != nil {
		if err := gwRouteSvc.Start(ctx); err != nil {
			logger.Error("failed to start gateway route service", slog.String("error", err.Error()))
		} else {
			logger.Info("gateway route service started",
				slog.String("mode", cfg.Gateway.RouteMode),
			)

			// Start gateway route reconciliation loop.
			go gatewayRouteReconcileLoop(ctx, gwRouteSvc, cfg.Gateway.ControllerAddr, logger)
		}
	}

	// Start health/metrics server for Prometheus scraping and health probes.
	healthSrv := startGatewayHealthServer(cfg.Gateway, gwRouteSvc, logger)

	<-ctx.Done()
	logger.Info("shutting down gateway")

	// Shutdown health server first.
	if healthSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("failed to stop health server", slog.String("error", err.Error()))
		} else {
			logger.Info("health server stopped")
		}
	}

	// Graceful shutdown of gateway route forwarding.
	if gwRouteSvc != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := gwRouteSvc.Stop(shutdownCtx); err != nil {
			logger.Error("failed to stop gateway route service", slog.String("error", err.Error()))
		} else {
			logger.Info("gateway route service stopped")
		}
	}

	return nil
}

// initGatewayRouteService creates the appropriate gateway route forwarding service.
func initGatewayRouteService(cfg config.GatewayConfig, logger *slog.Logger) gatewayrt.Service {
	mode := cfg.RouteMode
	if mode == "" {
		mode = "noop"
	}

	wireGuardIP := cfg.WireGuardIP
	if wireGuardIP == "" {
		wireGuardIP = "127.0.0.1" // Fallback; real setup uses gateway's WireGuard IP.
	}

	switch mode {
	case "forwarder":
		logger.Info("using gateway route forwarder service",
			slog.String("wireguard_ip", wireGuardIP),
		)
		return gatewayrt.NewForwarderService(wireGuardIP, logger)
	case "noop":
		logger.Info("using noop gateway route service (demo mode)")
		return gatewayrt.NewNoopService()
	default:
		logger.Error("unknown gateway route mode, falling back to noop", slog.String("mode", mode))
		return gatewayrt.NewNoopService()
	}
}

// startGatewayHealthServer creates and starts an HTTP server for health checks
// and Prometheus metrics on the gateway. Returns nil if the server fails to start.
func startGatewayHealthServer(
	cfg config.GatewayConfig,
	gwRouteSvc gatewayrt.Service,
	logger *slog.Logger,
) *http.Server {
	healthAddr := cfg.HealthAddr
	if healthAddr == "" {
		healthAddr = ":9090"
	}

	health := observability.NewHealthChecker()
	metrics := observability.NewMetrics()

	// Register health check for gateway route service.
	if gwRouteSvc != nil {
		health.Register(observability.HealthCheck{
			Name: "route",
			Check: func(ctx context.Context) error {
				status, err := gwRouteSvc.GetStatus(ctx)
				if err != nil {
					return err
				}
				if !status.Running {
					return fmt.Errorf("gateway route service not running")
				}
				return nil
			},
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", health.Handler())
	mux.Handle("/readyz", health.ReadyzHandler())
	mux.Handle("/livez", observability.LivezHandler())
	mux.Handle("/metrics", metrics.Handler())

	srv := &http.Server{
		Addr:         healthAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("gateway health server listening", slog.String("addr", healthAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", slog.String("error", err.Error()))
		}
	}()

	return srv
}

// gatewayRouteReconcileLoop periodically polls the controller for desired route state
// and reconciles the gateway route forwarding service to match. Runs every 30 seconds.
func gatewayRouteReconcileLoop(ctx context.Context, svc gatewayrt.Service, controllerAddr string, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: Poll GET /api/v1/gateways/{id}/config/routes from controller
			// and call svc.Reconcile(ctx, config).
			logger.Debug("gateway route reconciliation tick",
				slog.String("controller", controllerAddr),
			)
		}
	}
}
