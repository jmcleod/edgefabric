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

	"github.com/jmcleod/edgefabric/internal/bgp"
	"github.com/jmcleod/edgefabric/internal/cdnserver"
	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/dnsserver"
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/nodeclient"
	"github.com/jmcleod/edgefabric/internal/nodestate"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/plugin"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/routeserver"
)

// RunNode starts the node process.
func RunNode(cfg *config.Config) error {
	logger := observability.NewLogger(cfg.DefaultLogLevel())
	slog.SetDefault(logger)

	logger.Info("starting edgefabric node",
		slog.String("controller_addr", cfg.Node.ControllerAddr),
		slog.Bool("bgp_enabled", cfg.Node.BGP.Enabled),
		slog.Bool("dns_enabled", cfg.Node.DNS.Enabled),
		slog.Bool("cdn_enabled", cfg.Node.CDN.Enabled),
		slog.Bool("route_enabled", cfg.Node.Route.Enabled),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create shared metrics and event bus for node-side monitoring.
	metrics := observability.NewMetrics()
	eventBus := events.NewBus(logger)

	// Subscribe a log handler for monitoring events on the node side.
	monitoringEvents := []events.EventType{
		events.OverlayPeerUnreachable,
		events.OverlayPeerRecovered,
		events.BGPSessionDown,
		events.BGPSessionEstablished,
		events.RouteHealthCheckFailed,
		events.RouteHealthCheckRecovered,
	}
	logHandler := events.NewLogHandler(logger)
	for _, et := range monitoringEvents {
		eventBus.Subscribe(et, logHandler)
	}

	// Load or establish node identity.
	client, err := resolveNodeIdentity(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("resolve node identity: %w", err)
	}

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
				go bgpReconcileLoop(ctx, bgpSvc, client, logger)

				// Start BGP session monitor (Milestone 11.2).
				bgpMon := bgp.NewMonitor(bgpSvc, bgp.DefaultMonitorConfig(), eventBus, metrics, logger)
				bgpMon.Start(ctx)
				defer bgpMon.Stop()
			}
		}
	}

	// Initialize DNS service if enabled.
	var dnsSvc dnsserver.Service
	if cfg.Node.DNS.Enabled {
		dnsSvc = initDNSService(cfg.Node.DNS, logger, metrics)
		if dnsSvc != nil {
			listenAddr := cfg.Node.DNS.ListenAddr
			if listenAddr == "" {
				listenAddr = ":5353"
			}

			if err := dnsSvc.Start(ctx, listenAddr); err != nil {
				logger.Error("failed to start DNS service", slog.String("error", err.Error()))
			} else {
				logger.Info("DNS service started",
					slog.String("listen_addr", listenAddr),
					slog.String("mode", cfg.Node.DNS.Mode),
				)

				// Start DNS reconciliation loop.
				go dnsReconcileLoop(ctx, dnsSvc, client, logger)
			}
		}
	}

	// Initialize CDN service if enabled.
	var cdnSvc cdnserver.Service
	if cfg.Node.CDN.Enabled {
		cdnSvc = initCDNService(cfg.Node.CDN, logger, metrics)
		if cdnSvc != nil {
			listenAddr := cfg.Node.CDN.ListenAddr
			if listenAddr == "" {
				listenAddr = ":8080"
			}

			if err := cdnSvc.Start(ctx, listenAddr); err != nil {
				logger.Error("failed to start CDN service", slog.String("error", err.Error()))
			} else {
				logger.Info("CDN service started",
					slog.String("listen_addr", listenAddr),
					slog.String("mode", cfg.Node.CDN.Mode),
				)

				// Start CDN reconciliation loop.
				go cdnReconcileLoop(ctx, cdnSvc, client, logger)
			}
		}
	}

	// Initialize route forwarding service if enabled.
	var routeSvc routeserver.Service
	var routeHealth *routeserver.RouteHealthChecker
	if cfg.Node.Route.Enabled {
		routeSvc = initRouteService(cfg.Node.Route, logger)
		if routeSvc != nil {
			if err := routeSvc.Start(ctx); err != nil {
				logger.Error("failed to start route service", slog.String("error", err.Error()))
			} else {
				logger.Info("route service started",
					slog.String("mode", cfg.Node.Route.Mode),
				)

				// Start route health checker (Milestone 11.3).
				routeHealth = routeserver.NewRouteHealthChecker(
					routeserver.DefaultRouteHealthConfig(),
					eventBus, metrics, logger,
				)
				routeHealth.Start()
				defer routeHealth.Stop()

				// Start route reconciliation loop (passes health checker for target updates).
				go routeReconcileLoop(ctx, routeSvc, client, logger, routeHealth)
			}
		}
	}

	// Start WireGuard overlay health checker if overlay IP is configured (Milestone 11.1).
	overlayTargets := buildOverlayTargets(cfg)
	if len(overlayTargets) > 0 {
		overlayHealth := networking.NewOverlayHealthChecker(
			overlayTargets,
			networking.DefaultOverlayHealthConfig(),
			eventBus, metrics, logger,
		)
		overlayHealth.Start()
		defer overlayHealth.Stop()
	}

	// Start health/metrics server for Prometheus scraping and health probes.
	healthSrv := startNodeHealthServer(cfg.Node, bgpSvc, dnsSvc, cdnSvc, routeSvc, metrics, logger)

	<-ctx.Done()
	logger.Info("shutting down node")

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

	// Graceful shutdown of route forwarding (before CDN, DNS, BGP).
	if routeSvc != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := routeSvc.Stop(shutdownCtx); err != nil {
			logger.Error("failed to stop route service", slog.String("error", err.Error()))
		} else {
			logger.Info("route service stopped")
		}
	}

	// Graceful shutdown of CDN (before DNS and BGP).
	if cdnSvc != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := cdnSvc.Stop(shutdownCtx); err != nil {
			logger.Error("failed to stop CDN service", slog.String("error", err.Error()))
		} else {
			logger.Info("CDN service stopped")
		}
	}

	// Graceful shutdown of DNS.
	if dnsSvc != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := dnsSvc.Stop(shutdownCtx); err != nil {
			logger.Error("failed to stop DNS service", slog.String("error", err.Error()))
		} else {
			logger.Info("DNS service stopped")
		}
	}

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

// resolveNodeIdentity loads persisted node state or enrolls with the controller.
// Returns a controller client ready for config polling.
func resolveNodeIdentity(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*nodeclient.Client, error) {
	// Try loading existing state.
	state, err := nodestate.Load(cfg.Node.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load node state: %w", err)
	}

	if state != nil {
		logger.Info("loaded existing node state",
			slog.String("node_id", state.NodeID),
			slog.String("wireguard_ip", state.WireGuardIP),
		)
		return nodeclient.New(cfg.Node.ControllerAddr, state.NodeID, state.APIToken), nil
	}

	// No state — enroll with the controller.
	if cfg.Node.EnrollmentToken == "" {
		return nil, fmt.Errorf("no node state and no enrollment_token configured")
	}

	logger.Info("enrolling with controller", slog.String("controller", cfg.Node.ControllerAddr))
	result, err := nodeclient.Enroll(ctx, cfg.Node.ControllerAddr, cfg.Node.EnrollmentToken)
	if err != nil {
		return nil, fmt.Errorf("enrollment failed: %w", err)
	}

	logger.Info("enrollment successful",
		slog.String("node_id", result.NodeID),
		slog.String("wireguard_ip", result.WireGuardIP),
	)

	// Persist state for future restarts.
	if err := nodestate.Save(cfg.Node.DataDir, &nodestate.State{
		NodeID:      result.NodeID,
		APIToken:    result.APIToken,
		WireGuardIP: result.WireGuardIP,
	}); err != nil {
		logger.Error("failed to save node state (continuing anyway)", slog.String("error", err.Error()))
	}

	return nodeclient.New(cfg.Node.ControllerAddr, result.NodeID, result.APIToken), nil
}

// initBGPService creates the appropriate BGP service via the plugin registry.
func initBGPService(cfg config.BGPConfig, logger *slog.Logger) bgp.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	factory, ok := plugin.Get(plugin.PluginTypeBGP, mode)
	if !ok {
		logger.Error("unknown BGP plugin, falling back to noop", slog.String("mode", mode))
		factory, _ = plugin.Get(plugin.PluginTypeBGP, "noop")
	}

	logger.Info("using BGP plugin", slog.String("mode", mode))
	return factory.(plugin.BGPFactory)()
}

// initDNSService creates the appropriate DNS service via the plugin registry.
func initDNSService(cfg config.DNSConfig, logger *slog.Logger, metrics *observability.Metrics) dnsserver.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	factory, ok := plugin.Get(plugin.PluginTypeDNS, mode)
	if !ok {
		logger.Error("unknown DNS plugin, falling back to noop", slog.String("mode", mode))
		factory, _ = plugin.Get(plugin.PluginTypeDNS, "noop")
	}

	logger.Info("using DNS plugin", slog.String("mode", mode))
	return factory.(plugin.DNSFactory)(logger, metrics, cfg.AXFREnabled)
}

// bgpReconcileLoop periodically polls the controller for desired BGP state
// and reconciles the local BGP service to match. Runs every 30 seconds.
func bgpReconcileLoop(ctx context.Context, svc bgp.Service, client *nodeclient.Client, logger *slog.Logger) {
	// Immediate first reconciliation — don't wait 30s.
	sessions, err := client.FetchBGPConfig(ctx)
	if err != nil {
		logger.Warn("BGP initial config fetch failed", slog.String("error", err.Error()))
	} else if err := svc.Reconcile(ctx, sessions); err != nil {
		logger.Warn("BGP initial reconciliation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("BGP initial reconciliation complete", slog.Int("sessions", len(sessions)))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sessions, err := client.FetchBGPConfig(ctx)
			if err != nil {
				logger.Warn("BGP config fetch failed", slog.String("error", err.Error()))
				continue
			}
			if err := svc.Reconcile(ctx, sessions); err != nil {
				logger.Warn("BGP reconciliation failed", slog.String("error", err.Error()))
			} else {
				logger.Debug("BGP reconciliation complete", slog.Int("sessions", len(sessions)))
			}
		}
	}
}

// dnsReconcileLoop periodically polls the controller for desired DNS state
// and reconciles the local DNS service to match. Runs every 30 seconds.
func dnsReconcileLoop(ctx context.Context, svc dnsserver.Service, client *nodeclient.Client, logger *slog.Logger) {
	// Immediate first reconciliation — don't wait 30s.
	dnsCfg, err := client.FetchDNSConfig(ctx)
	if err != nil {
		logger.Warn("DNS initial config fetch failed", slog.String("error", err.Error()))
	} else if err := svc.Reconcile(ctx, dnsCfg); err != nil {
		logger.Warn("DNS initial reconciliation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("DNS initial reconciliation complete", slog.Int("zones", len(dnsCfg.Zones)))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dnsCfg, err := client.FetchDNSConfig(ctx)
			if err != nil {
				logger.Warn("DNS config fetch failed", slog.String("error", err.Error()))
				continue
			}
			if err := svc.Reconcile(ctx, dnsCfg); err != nil {
				logger.Warn("DNS reconciliation failed", slog.String("error", err.Error()))
			} else {
				logger.Debug("DNS reconciliation complete", slog.Int("zones", len(dnsCfg.Zones)))
			}
		}
	}
}

// initCDNService creates the appropriate CDN service via the plugin registry.
func initCDNService(cfg config.CDNConfig, logger *slog.Logger, metrics *observability.Metrics) cdnserver.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	factory, ok := plugin.Get(plugin.PluginTypeCDN, mode)
	if !ok {
		logger.Error("unknown CDN plugin, falling back to noop", slog.String("mode", mode))
		factory, _ = plugin.Get(plugin.PluginTypeCDN, "noop")
	}

	logger.Info("using CDN plugin", slog.String("mode", mode))
	svc := factory.(plugin.CDNFactory)(logger, metrics)

	// Apply CDN-specific cache configuration for the proxy implementation.
	if mode == "proxy" && cfg.CacheDir != "" {
		if proxySvc, ok := svc.(*cdnserver.ProxyService); ok {
			maxBytes := cfg.CacheMaxBytes
			if maxBytes <= 0 {
				maxBytes = 512 * 1024 * 1024 // 512MB default
			}
			proxySvc.SetCacheConfig(cfg.CacheDir, maxBytes)
			logger.Info("CDN disk cache enabled",
				slog.String("cache_dir", cfg.CacheDir),
				slog.Int64("max_bytes", maxBytes),
			)
		}
	}

	return svc
}

// cdnReconcileLoop periodically polls the controller for desired CDN state
// and reconciles the local CDN service to match. Runs every 30 seconds.
func cdnReconcileLoop(ctx context.Context, svc cdnserver.Service, client *nodeclient.Client, logger *slog.Logger) {
	// Immediate first reconciliation — don't wait 30s.
	cdnCfg, err := client.FetchCDNConfig(ctx)
	if err != nil {
		logger.Warn("CDN initial config fetch failed", slog.String("error", err.Error()))
	} else if err := svc.Reconcile(ctx, cdnCfg); err != nil {
		logger.Warn("CDN initial reconciliation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("CDN initial reconciliation complete", slog.Int("sites", len(cdnCfg.Sites)))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cdnCfg, err := client.FetchCDNConfig(ctx)
			if err != nil {
				logger.Warn("CDN config fetch failed", slog.String("error", err.Error()))
				continue
			}
			if err := svc.Reconcile(ctx, cdnCfg); err != nil {
				logger.Warn("CDN reconciliation failed", slog.String("error", err.Error()))
			} else {
				logger.Debug("CDN reconciliation complete", slog.Int("sites", len(cdnCfg.Sites)))
			}
		}
	}
}

// initRouteService creates the appropriate route forwarding service via the plugin registry.
func initRouteService(cfg config.RouteConfig, logger *slog.Logger) routeserver.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	factory, ok := plugin.Get(plugin.PluginTypeRoute, mode)
	if !ok {
		logger.Error("unknown route plugin, falling back to noop", slog.String("mode", mode))
		factory, _ = plugin.Get(plugin.PluginTypeRoute, "noop")
	}

	logger.Info("using route plugin", slog.String("mode", mode))
	return factory.(plugin.RouteFactory)(logger, nil)
}

// routeReconcileLoop periodically polls the controller for desired route state
// and reconciles the local route forwarding service to match. Runs every 30 seconds.
// If healthChecker is non-nil, it updates the health checker's targets after each reconciliation.
func routeReconcileLoop(ctx context.Context, svc routeserver.Service, client *nodeclient.Client, logger *slog.Logger, healthChecker *routeserver.RouteHealthChecker) {
	// Immediate first reconciliation — don't wait 30s.
	rtCfg, err := client.FetchRouteConfig(ctx)
	if err != nil {
		logger.Warn("route initial config fetch failed", slog.String("error", err.Error()))
	} else if err := svc.Reconcile(ctx, rtCfg); err != nil {
		logger.Warn("route initial reconciliation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("route initial reconciliation complete", slog.Int("routes", len(rtCfg.Routes)))
		updateRouteHealthTargets(healthChecker, rtCfg)
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
				logger.Warn("route config fetch failed", slog.String("error", err.Error()))
				continue
			}
			if err := svc.Reconcile(ctx, rtCfg); err != nil {
				logger.Warn("route reconciliation failed", slog.String("error", err.Error()))
			} else {
				logger.Debug("route reconciliation complete", slog.Int("routes", len(rtCfg.Routes)))
				updateRouteHealthTargets(healthChecker, rtCfg)
			}
		}
	}
}

// updateRouteHealthTargets extracts route targets from config and feeds them to the health checker.
func updateRouteHealthTargets(hc *routeserver.RouteHealthChecker, rtCfg *route.NodeRouteConfig) {
	if hc == nil || rtCfg == nil {
		return
	}
	var targets []routeserver.RouteTarget
	for _, rwg := range rtCfg.Routes {
		port := 0
		if rwg.Route.EntryPort != nil {
			port = *rwg.Route.EntryPort
		}
		targets = append(targets, routeserver.RouteTarget{
			RouteID:     rwg.Route.ID,
			RouteName:   rwg.Route.Name,
			Protocol:    rwg.Route.Protocol,
			GatewayWGIP: rwg.GatewayWGIP,
			EntryPort:   port,
		})
	}
	hc.UpdateTargets(targets)
}

// buildOverlayTargets constructs overlay health check targets from config.
// Returns an empty slice if WireGuard overlay is not configured (e.g. Docker demo mode).
func buildOverlayTargets(cfg *config.Config) []networking.OverlayTarget {
	// The controller overlay IP defaults to the WireGuard hub address.
	controllerIP := cfg.Node.Monitoring.ControllerOverlayIP
	if controllerIP == "" {
		return nil // No overlay to monitor.
	}
	return []networking.OverlayTarget{
		{Name: "controller", IP: controllerIP},
	}
}

// startNodeHealthServer creates and starts an HTTP server for health checks
// and Prometheus metrics on the node. Returns nil if the server fails to start.
func startNodeHealthServer(
	cfg config.NodeConfig,
	bgpSvc bgp.Service,
	dnsSvc dnsserver.Service,
	cdnSvc cdnserver.Service,
	routeSvc routeserver.Service,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *http.Server {
	healthAddr := cfg.HealthAddr
	if healthAddr == "" {
		healthAddr = ":9090"
	}

	health := observability.NewHealthChecker()

	// Register health checks for enabled services.
	if bgpSvc != nil {
		health.Register(observability.HealthCheck{
			Name: "bgp",
			Check: func(ctx context.Context) error {
				_, err := bgpSvc.GetStatus(ctx)
				return err
			},
		})
	}
	if dnsSvc != nil {
		health.Register(observability.HealthCheck{
			Name: "dns",
			Check: func(ctx context.Context) error {
				status, err := dnsSvc.GetStatus(ctx)
				if err != nil {
					return err
				}
				if !status.Listening {
					return fmt.Errorf("DNS service not listening")
				}
				return nil
			},
		})
	}
	if cdnSvc != nil {
		health.Register(observability.HealthCheck{
			Name: "cdn",
			Check: func(ctx context.Context) error {
				status, err := cdnSvc.GetStatus(ctx)
				if err != nil {
					return err
				}
				if !status.Listening {
					return fmt.Errorf("CDN service not listening")
				}
				return nil
			},
		})
	}
	if routeSvc != nil {
		health.Register(observability.HealthCheck{
			Name: "route",
			Check: func(ctx context.Context) error {
				status, err := routeSvc.GetStatus(ctx)
				if err != nil {
					return err
				}
				if !status.Running {
					return fmt.Errorf("route service not running")
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
		logger.Info("node health server listening", slog.String("addr", healthAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", slog.String("error", err.Error()))
		}
	}()

	return srv
}
