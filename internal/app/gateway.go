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
	"github.com/jmcleod/edgefabric/internal/gatewayclient"
	"github.com/jmcleod/edgefabric/internal/gatewayrt"
	"github.com/jmcleod/edgefabric/internal/gatewaystate"
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

	// Resolve gateway identity from state file.
	client, err := resolveGatewayIdentity(cfg, logger)
	if err != nil {
		logger.Warn("gateway identity not available, route polling disabled",
			slog.String("error", err.Error()),
		)
	}

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
			if client != nil {
				go gatewayRouteReconcileLoop(ctx, gwRouteSvc, client, logger)
			} else {
				logger.Warn("skipping route reconciliation — no controller client available")
			}
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

// resolveGatewayIdentity loads the gateway's persisted state (ID + token)
// and returns a controller client ready for config polling.
func resolveGatewayIdentity(cfg *config.Config, logger *slog.Logger) (*gatewayclient.Client, error) {
	state, err := gatewaystate.Load(cfg.Gateway.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load gateway state: %w", err)
	}

	if state == nil {
		return nil, fmt.Errorf("no gateway state file found in %s", cfg.Gateway.DataDir)
	}

	if state.GatewayID == "" || state.APIToken == "" {
		return nil, fmt.Errorf("gateway state incomplete (missing gateway_id or api_token)")
	}

	logger.Info("loaded gateway identity",
		slog.String("gateway_id", state.GatewayID),
	)

	return gatewayclient.New(cfg.Gateway.ControllerAddr, state.GatewayID, state.APIToken), nil
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
			slog.String("bind_ip", wireGuardIP),
		)
		return gatewayrt.NewForwarderService(wireGuardIP, logger, nil)
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
func gatewayRouteReconcileLoop(ctx context.Context, svc gatewayrt.Service, client *gatewayclient.Client, logger *slog.Logger) {
	// Immediate first reconciliation — don't wait 30s.
	rtCfg, err := client.FetchRouteConfig(ctx)
	if err != nil {
		logger.Warn("gateway route initial config fetch failed", slog.String("error", err.Error()))
	} else if err := svc.Reconcile(ctx, rtCfg); err != nil {
		logger.Warn("gateway route initial reconciliation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("gateway route initial reconciliation complete", slog.Int("routes", len(rtCfg.Routes)))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rtCfg, err := client.FetchRouteConfig(ctx)
			if err != nil {
				logger.Warn("gateway route config fetch failed", slog.String("error", err.Error()))
				continue
			}
			if err := svc.Reconcile(ctx, rtCfg); err != nil {
				logger.Warn("gateway route reconciliation failed", slog.String("error", err.Error()))
				continue
			}
			logger.Debug("gateway route reconciliation complete", slog.Int("routes", len(rtCfg.Routes)))
		}
	}
}
