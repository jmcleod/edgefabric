// Package app contains the application wiring for each binary mode.
package app

import (
	"context"
	"crypto/tls"
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
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/ha"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/storage/postgres"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
	"github.com/jmcleod/edgefabric/web"
	"golang.org/x/crypto/acme/autocert"
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

	// Initialize storage based on configured driver.
	store, err := initStore(cfg.Controller.Storage)
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

	// Use a separate token signing key if configured; fall back to encryption key.
	signingKey := cfg.Controller.Secrets.TokenSigningKey
	if signingKey == "" {
		signingKey = cfg.Controller.Secrets.EncryptionKey
		logger.Warn("token_signing_key not set, using encryption_key (set separate key for production)")
	}
	tokenSvc := auth.NewTokenService(
		[]byte(signingKey),
		24*time.Hour,
	)
	tenantSvc := tenant.NewService(store)
	userSvc := user.NewService(store, authSvc)

	// Initialize event bus for system event broadcasting.
	eventBus := events.NewBus(logger)

	// Register notification handlers from config.
	registerNotificationHandlers(eventBus, cfg.Controller.Notifications, logger)

	fleetSvc := fleet.NewService(store, store, store, store, fleet.WithEventBus(eventBus))
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

	// Set up leader election with gauge updater in onElected/onDemoted callbacks.
	var gaugeCancel context.CancelFunc
	startGauges := func() {
		var gaugeCtx context.Context
		gaugeCtx, gaugeCancel = context.WithCancel(context.Background())
		metrics.LeaderStatus.Set(1)
		observability.StartGaugeUpdater(gaugeCtx, metrics, 15*time.Second, func(m *observability.Metrics) {
			_, nodeCount, err := fleetSvc.ListNodes(context.Background(), nil, storage.ListParams{Limit: 1})
			if err == nil {
				m.ActiveNodes.Set(float64(nodeCount))
			}
			_, tCount, err := tenantSvc.List(context.Background(), storage.ListParams{Limit: 1})
			if err == nil {
				m.ActiveTenants.Set(float64(tCount))
			}
			_, gatewayTotal, err := fleetSvc.ListGateways(context.Background(), nil, storage.ListParams{Limit: 1})
			if err == nil {
				m.ActiveGateways.Set(float64(gatewayTotal))
			}
		})
		logger.Info("leader: started background gauge updater")
	}
	stopGauges := func() {
		metrics.LeaderStatus.Set(0)
		if gaugeCancel != nil {
			gaugeCancel()
			gaugeCancel = nil
		}
		logger.Info("leader: stopped background gauge updater")
	}

	// Create leader elector based on storage driver.
	var elector ha.LeaderElector
	if cfg.Controller.Storage.Driver == "postgres" {
		pgStore := store.(*postgres.PostgresStore)
		var opts []ha.Option
		if cfg.Controller.LeaderElection.Interval != "" {
			if d, err := time.ParseDuration(cfg.Controller.LeaderElection.Interval); err == nil {
				opts = append(opts, ha.WithElectionInterval(d))
			}
		}
		elector = ha.NewPostgresLeaderElector(pgStore.DB(), logger, eventBus, startGauges, stopGauges, opts...)
	} else {
		elector = ha.NewNoopLeaderElector(startGauges, stopGauges)
	}

	// Start leader election in the background.
	electionCtx, electionCancel := context.WithCancel(context.Background())
	defer electionCancel()
	go func() {
		if err := elector.Start(electionCtx); err != nil {
			logger.Error("leader election error", slog.String("error", err.Error()))
		}
	}()

	// Assemble API router.
	tlsEnabled := cfg.Controller.TLS.Enabled
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
		NodeStore:       store,
		GatewayStore:    store,
		SchemaVersioner: store,
		Health:          health,
		Metrics:         metrics,
		Logger:          logger,
		StaticFS:        web.StaticFiles,
		CORSOrigins:     cfg.Controller.CORS.AllowedOrigins,
		TLSEnabled:      tlsEnabled,
		IsLeader:        elector.IsLeader,
	})

	srv := &http.Server{
		Addr:         cfg.Controller.ListenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Configure TLS if enabled.
	if tlsEnabled {
		tlsCfg, err := buildTLSConfig(cfg.Controller.TLS, logger)
		if err != nil {
			return fmt.Errorf("configure TLS: %w", err)
		}
		srv.TLSConfig = tlsCfg
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if tlsEnabled {
			// Start HTTP→HTTPS redirect on :80 if TLS is enabled.
			go startHTTPRedirect(cfg.Controller.ListenAddr, logger)

			logger.Info("HTTPS server listening", slog.String("addr", cfg.Controller.ListenAddr))
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		} else {
			logger.Info("HTTP server listening", slog.String("addr", cfg.Controller.ListenAddr))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
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

// buildTLSConfig constructs a *tls.Config from the controller's TLS settings.
// It supports both manual cert/key files and automatic Let's Encrypt via autocert.
func buildTLSConfig(tlsCfg config.TLSConfig, logger *slog.Logger) (*tls.Config, error) {
	if tlsCfg.AutoCert {
		logger.Info("TLS: using Let's Encrypt autocert")
		m := &autocert.Manager{
			Prompt: autocert.AcceptTOS,
			Cache:  autocert.DirCache(".edgefabric-certs"),
		}
		return m.TLSConfig(), nil
	}

	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		logger.Info("TLS: using manual cert/key files",
			slog.String("cert", tlsCfg.CertFile),
			slog.String("key", tlsCfg.KeyFile),
		)
		cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS cert/key: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}

	return nil, fmt.Errorf("TLS enabled but no cert_file/key_file or auto_cert configured")
}

// startHTTPRedirect starts an HTTP server on port 80 that redirects all
// requests to HTTPS. This runs in the background and logs errors.
func startHTTPRedirect(httpsAddr string, logger *slog.Logger) {
	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	httpSrv := &http.Server{
		Addr:         ":80",
		Handler:      redirectHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	logger.Info("HTTP→HTTPS redirect server listening", slog.String("addr", ":80"))
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP redirect server error", slog.String("error", err.Error()))
	}
}

// storeWithVersion combines Store and SchemaVersioner so either driver can be
// passed to the rest of the application.
type storeWithVersion interface {
	storage.Store
	storage.SchemaVersioner
}

// initStore creates the appropriate storage backend based on the configured driver.
func initStore(cfg config.StorageConfig) (storeWithVersion, error) {
	switch cfg.Driver {
	case "sqlite":
		return sqlite.New(cfg.DSN)
	case "postgres":
		return postgres.New(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %q", cfg.Driver)
	}
}

// allEventTypes lists every known event type for subscribing notification handlers.
var allEventTypes = []events.EventType{
	events.NodeStatusChanged,
	events.GatewayStatusChanged,
	events.ProvisioningFailed,
	events.CertificateExpiring,
	events.HealthCheckFailed,
	events.OverlayPeerUnreachable,
	events.OverlayPeerRecovered,
	events.BGPSessionDown,
	events.BGPSessionEstablished,
	events.RouteHealthCheckFailed,
	events.RouteHealthCheckRecovered,
	events.LeaderElected,
	events.LeaderLost,
}

// registerNotificationHandlers wires webhook and Slack handlers onto the event
// bus based on the controller's notification configuration.
func registerNotificationHandlers(bus *events.Bus, cfg config.NotificationsConfig, logger *slog.Logger) {
	if len(cfg.Webhooks) > 0 {
		var whConfigs []events.WebhookConfig
		for _, w := range cfg.Webhooks {
			whConfigs = append(whConfigs, events.WebhookConfig{
				URL:    w.URL,
				Secret: w.Secret,
			})
		}
		handler := events.NewWebhookHandler(logger, whConfigs)
		for _, et := range allEventTypes {
			bus.Subscribe(et, handler)
		}
		logger.Info("webhook notification handler registered",
			slog.Int("endpoints", len(cfg.Webhooks)),
		)
	}

	if cfg.Slack.WebhookURL != "" {
		handler := events.NewSlackHandler(logger, events.SlackConfig{
			WebhookURL: cfg.Slack.WebhookURL,
			Channel:    cfg.Slack.Channel,
		})
		for _, et := range allEventTypes {
			bus.Subscribe(et, handler)
		}
		logger.Info("slack notification handler registered")
	}

	if cfg.Email.SMTPHost != "" && len(cfg.Email.Recipients) > 0 {
		handler := events.NewEmailHandler(logger, events.EmailConfig{
			SMTPHost:   cfg.Email.SMTPHost,
			SMTPPort:   cfg.Email.SMTPPort,
			Username:   cfg.Email.Username,
			Password:   cfg.Email.Password,
			FromAddr:   cfg.Email.FromAddr,
			Recipients: cfg.Email.Recipients,
			UseTLS:     cfg.Email.UseTLS,
		})
		for _, et := range allEventTypes {
			bus.Subscribe(et, handler)
		}
		logger.Info("email notification handler registered",
			slog.String("from", cfg.Email.FromAddr),
			slog.Int("recipients", len(cfg.Email.Recipients)),
		)
	}
}
