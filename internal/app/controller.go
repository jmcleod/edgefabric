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
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage"
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
	fleetSvc := fleet.NewService(store, store, store, store)
	authorizer := rbac.NewAuthorizer()
	auditLog := audit.NewLogger(store, logger)

	// Seed superuser on first boot.
	if err := SeedSuperUser(context.Background(), userSvc, store, logger); err != nil {
		return fmt.Errorf("seed superuser: %w", err)
	}

	// Initialize provisioning service.
	sshClient := ssh.NewClient()
	provisioningSvc := provisioning.NewProvisioner(
		store,     // NodeStore
		store,     // ProvisioningJobStore
		store,     // EnrollmentTokenStore
		store,     // WireGuardPeerStore
		store,     // SSHKeyStore
		sshClient, // SSH client
		secretStore,
		cfg.Controller.WireGuard,
		cfg.Controller.ExternalURL,
	)

	// Initialize networking service.
	networkingSvc := networking.NewService(
		store, // NodeStore
		store, // WireGuardPeerStore
		store, // BGPSessionStore
		store, // IPAllocationStore
		secretStore,
		cfg.Controller.WireGuard,
	)

	// Initialize DNS service.
	dnsSvc := dns.NewService(
		store, // DNSZoneStore
		store, // DNSRecordStore
		store, // NodeGroupStore
		store, // NodeStore
	)

	// Initialize CDN service.
	cdnSvc := cdn.NewService(
		store, // CDNSiteStore
		store, // CDNOriginStore
		store, // NodeGroupStore
		store, // NodeStore
	)

	// Initialize route service.
	routeSvc := route.NewService(
		store, // RouteStore
		store, // GatewayStore
		store, // NodeGroupStore
		store, // NodeStore
	)

	// Connect provisioning service to networking for WG config sync.
	provisioningSvc.SetWireGuardConfigGenerator(networkingSvc)

	// Bootstrap controller's WireGuard peer (idempotent).
	if _, err := networking.BootstrapControllerPeer(
		context.Background(),
		store,
		secretStore,
		cfg.Controller.WireGuard,
		domain.ControllerPeerID,
	); err != nil {
		return fmt.Errorf("bootstrap controller wireguard peer: %w", err)
	}
	logger.Info("controller wireguard peer bootstrapped")

	// Start system gauge updater (refreshes active node/gateway/tenant counts every 15s).
	gaugeCtx, gaugeCancel := context.WithCancel(context.Background())
	defer gaugeCancel()
	observability.StartGaugeUpdater(gaugeCtx, metrics, 15*time.Second, func(m *observability.Metrics) {
		_, nodeCount, err := fleetSvc.ListNodes(context.Background(), nil, storage.ListParams{Limit: 1})
		if err == nil {
			m.ActiveNodes.Set(float64(nodeCount))
		}
		_, tCount, err := tenantSvc.List(context.Background(), storage.ListParams{Limit: 1})
		if err == nil {
			m.ActiveTenants.Set(float64(tCount))
		}
		// Sum gateway counts across all tenants.
		var gatewayTotal int
		tenants, _, err := tenantSvc.List(context.Background(), storage.ListParams{Limit: 200})
		if err == nil {
			for _, t := range tenants {
				_, gCount, err := fleetSvc.ListGateways(context.Background(), t.ID, storage.ListParams{Limit: 1})
				if err == nil {
					gatewayTotal += gCount
				}
			}
		}
		m.ActiveGateways.Set(float64(gatewayTotal))
	})

	// Assemble API router.
	handler := api.NewRouter(api.Services{
		AuthSvc:         authSvc,
		TokenSvc:        tokenSvc,
		TenantSvc:       tenantSvc,
		UserSvc:         userSvc,
		FleetSvc:        fleetSvc,
		NetworkingSvc:   networkingSvc,
		DNSSvc:          dnsSvc,
		CDNSvc:          cdnSvc,
		RouteSvc:        routeSvc,
		ProvisioningSvc: provisioningSvc,
		Authorizer:      authorizer,
		AuditLog:        auditLog,
		APIKeys:         store,
		SSHKeys:         store,
		Health:          health,
		Metrics:         metrics,
		Logger:          logger,
		StaticFS:        web.StaticFiles,
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
