package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmcleod/edgefabric/internal/bgp"
	"github.com/jmcleod/edgefabric/internal/cdnserver"
	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/dnsserver"
	"github.com/jmcleod/edgefabric/internal/observability"
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

	// Initialize DNS service if enabled.
	var dnsSvc dnsserver.Service
	if cfg.Node.DNS.Enabled {
		dnsSvc = initDNSService(cfg.Node.DNS, logger)
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
				go dnsReconcileLoop(ctx, dnsSvc, cfg.Node.ControllerAddr, logger)
			}
		}
	}

	// Initialize CDN service if enabled.
	var cdnSvc cdnserver.Service
	if cfg.Node.CDN.Enabled {
		cdnSvc = initCDNService(cfg.Node.CDN, logger)
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
				go cdnReconcileLoop(ctx, cdnSvc, cfg.Node.ControllerAddr, logger)
			}
		}
	}

	// TODO: Connect to controller over WireGuard.

	<-ctx.Done()
	logger.Info("shutting down node")

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

// initDNSService creates the appropriate DNS service based on config mode.
func initDNSService(cfg config.DNSConfig, logger *slog.Logger) dnsserver.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	switch mode {
	case "miekg":
		logger.Info("using miekg/dns authoritative DNS service")
		return dnsserver.NewMiekgService()
	case "noop":
		logger.Info("using noop DNS service (demo mode)")
		return dnsserver.NewNoopService()
	default:
		logger.Error("unknown DNS mode, falling back to noop", slog.String("mode", mode))
		return dnsserver.NewNoopService()
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

// dnsReconcileLoop periodically polls the controller for desired DNS state
// and reconciles the local DNS service to match. Runs every 30 seconds.
func dnsReconcileLoop(ctx context.Context, svc dnsserver.Service, controllerAddr string, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: Poll GET /api/v1/nodes/{id}/config/dns from controller
			// and call svc.Reconcile(ctx, config).
			// For now, just log that reconciliation would happen.
			logger.Debug("DNS reconciliation tick",
				slog.String("controller", controllerAddr),
			)
		}
	}
}

// initCDNService creates the appropriate CDN service based on config mode.
func initCDNService(cfg config.CDNConfig, logger *slog.Logger) cdnserver.Service {
	mode := cfg.Mode
	if mode == "" {
		mode = "noop"
	}

	switch mode {
	case "proxy":
		logger.Info("using reverse proxy CDN service")
		return cdnserver.NewProxyService(logger)
	case "noop":
		logger.Info("using noop CDN service (demo mode)")
		return cdnserver.NewNoopService()
	default:
		logger.Error("unknown CDN mode, falling back to noop", slog.String("mode", mode))
		return cdnserver.NewNoopService()
	}
}

// cdnReconcileLoop periodically polls the controller for desired CDN state
// and reconciles the local CDN service to match. Runs every 30 seconds.
func cdnReconcileLoop(ctx context.Context, svc cdnserver.Service, controllerAddr string, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: Poll GET /api/v1/nodes/{id}/config/cdn from controller
			// and call svc.Reconcile(ctx, config).
			// For now, just log that reconciliation would happen.
			logger.Debug("CDN reconciliation tick",
				slog.String("controller", controllerAddr),
			)
		}
	}
}
