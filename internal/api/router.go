// Package api assembles the full HTTP handler for the EdgeFabric controller.
package api

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
	v1 "github.com/jmcleod/edgefabric/internal/api/v1"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/fleet"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/route"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
	"github.com/jmcleod/edgefabric/openapi"
)

// Services groups all service dependencies needed by the API router.
type Services struct {
	AuthSvc    auth.Service
	TokenSvc   *auth.TokenService
	TenantSvc  tenant.Service
	UserSvc    user.Service
	FleetSvc        fleet.Service
	NetworkingSvc   networking.Service
	DNSSvc          dns.Service
	CDNSvc          cdn.Service
	RouteSvc        route.Service
	ProvisioningSvc provisioning.Service
	Authorizer      rbac.Authorizer
	AuditLog   audit.Logger
	APIKeys    storage.APIKeyStore
	SSHKeys         storage.SSHKeyStore
	NodeStore       storage.NodeStore
	GatewayStore    storage.GatewayStore
	SchemaVersioner storage.SchemaVersioner
	Health          *observability.HealthChecker
	Metrics         *observability.Metrics
	Logger          *slog.Logger
	StaticFS        fs.FS // Embedded SPA files (web.StaticFiles)
	CORSOrigins     []string // Allowed CORS origins (empty = same-origin only).
	TLSEnabled      bool     // When true, HSTS headers are set.
	IsLeader        func() bool // HA leader status check.
}

// NewRouter assembles the full HTTP handler with all middleware and routes.
func NewRouter(svc Services) http.Handler {
	mux := http.NewServeMux()

	// Auth middleware (shared by all protected routes).
	// TenantMetrics runs inside Auth so that claims are available for per-tenant counting.
	rawAuthMW := middleware.Auth(svc.TokenSvc, svc.AuthSvc, middleware.WithMetrics(svc.Metrics))
	tenantMetricsMW := middleware.TenantMetrics(svc.Metrics)
	authMW := func(next http.Handler) http.Handler {
		return rawAuthMW(tenantMetricsMW(next))
	}

	// Rate limiter for sensitive endpoints (login, enrollment, API key generation).
	// 10 requests/second with burst of 20 per client IP.
	rateLimiter := middleware.NewRateLimiter(10, 20)

	// Register API v1 handlers.
	tenantHandler := v1.NewTenantHandler(svc.TenantSvc, svc.Authorizer, svc.AuditLog)
	tenantHandler.Register(mux, authMW)

	userHandler := v1.NewUserHandler(svc.UserSvc, svc.Authorizer, svc.AuditLog)
	userHandler.Register(mux, authMW)

	authHandler := v1.NewAuthHandler(svc.AuthSvc, svc.TokenSvc, svc.APIKeys, svc.UserSvc, svc.Authorizer, svc.AuditLog, svc.Metrics, svc.TLSEnabled)
	authHandler.Register(mux, authMW)

	auditHandler := v1.NewAuditHandler(svc.AuditLog, svc.Authorizer)
	auditHandler.Register(mux, authMW)

	// Fleet management handlers (nodes, node groups, SSH keys).
	if svc.FleetSvc != nil {
		nodeHandler := v1.NewNodeHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		nodeHandler.Register(mux, authMW)

		nodeGroupHandler := v1.NewNodeGroupHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		nodeGroupHandler.Register(mux, authMW)

		statusHandler := v1.NewStatusHandler(svc.TenantSvc, svc.UserSvc, svc.FleetSvc, svc.DNSSvc, svc.CDNSvc, svc.RouteSvc, svc.SchemaVersioner, svc.Authorizer, svc.IsLeader)
		statusHandler.Register(mux, authMW)
	}

	if svc.SSHKeys != nil {
		sshKeyHandler := v1.NewSSHKeyHandler(svc.SSHKeys, svc.ProvisioningSvc, svc.Authorizer, svc.AuditLog)
		sshKeyHandler.Register(mux, authMW)
	}

	// Provisioning and enrollment handlers.
	if svc.ProvisioningSvc != nil {
		provisioningHandler := v1.NewProvisioningHandler(svc.ProvisioningSvc, svc.NodeStore, svc.Authorizer, svc.AuditLog)
		provisioningHandler.Register(mux, authMW)

		enrollmentHandler := v1.NewEnrollmentHandler(svc.ProvisioningSvc, svc.TokenSvc, svc.AuditLog)
		enrollmentHandler.Register(mux) // No auth — token-based.
	}

	// Gateway handler.
	if svc.FleetSvc != nil {
		gatewayHandler := v1.NewGatewayHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		gatewayHandler.Register(mux, authMW)
	}

	// Networking handlers (BGP sessions, IP allocations, WireGuard peers, node networking state).
	if svc.NetworkingSvc != nil {
		bgpHandler := v1.NewBGPSessionHandler(svc.NetworkingSvc, svc.Authorizer, svc.AuditLog)
		bgpHandler.Register(mux, authMW)

		ipAllocHandler := v1.NewIPAllocationHandler(svc.NetworkingSvc, svc.Authorizer, svc.AuditLog)
		ipAllocHandler.Register(mux, authMW)

		wgHandler := v1.NewWireGuardHandler(svc.NetworkingSvc, svc.Authorizer)
		wgHandler.Register(mux, authMW)

		nodeNetHandler := v1.NewNodeNetworkingHandler(svc.NetworkingSvc, svc.Authorizer)
		nodeNetHandler.Register(mux, authMW)

		nodeConfigHandler := v1.NewNodeConfigHandler(svc.NetworkingSvc, svc.DNSSvc, svc.CDNSvc, svc.RouteSvc, svc.NodeStore, svc.Authorizer)
		nodeConfigHandler.Register(mux, authMW)
	}

	// DNS handlers (zones, records).
	if svc.DNSSvc != nil {
		dnsZoneHandler := v1.NewDNSZoneHandler(svc.DNSSvc, svc.Authorizer, svc.AuditLog)
		dnsZoneHandler.Register(mux, authMW)

		dnsRecordHandler := v1.NewDNSRecordHandler(svc.DNSSvc, svc.Authorizer, svc.AuditLog)
		dnsRecordHandler.Register(mux, authMW)
	}

	// CDN handlers (sites, origins).
	if svc.CDNSvc != nil {
		cdnSiteHandler := v1.NewCDNSiteHandler(svc.CDNSvc, svc.Authorizer, svc.AuditLog)
		cdnSiteHandler.Register(mux, authMW)

		cdnOriginHandler := v1.NewCDNOriginHandler(svc.CDNSvc, svc.Authorizer, svc.AuditLog)
		cdnOriginHandler.Register(mux, authMW)
	}

	// Route handlers (CRUD + config sync).
	if svc.RouteSvc != nil {
		routeHandler := v1.NewRouteHandler(svc.RouteSvc, svc.Authorizer, svc.AuditLog)
		routeHandler.Register(mux, authMW)

		gatewayConfigHandler := v1.NewGatewayConfigHandler(svc.RouteSvc, svc.GatewayStore, svc.Authorizer)
		gatewayConfigHandler.Register(mux, authMW)
	}

	// Tenant usage metrics handler.
	tenantUsageHandler := v1.NewTenantUsageHandler(svc.Metrics, svc.Authorizer)
	tenantUsageHandler.Register(mux, authMW)

	// OpenAPI spec (unauthenticated).
	mux.HandleFunc("GET /api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapi.V1Spec)
	})

	// Infrastructure endpoints (unauthenticated).
	mux.Handle("/healthz", svc.Health.Handler())
	mux.Handle("/readyz", svc.Health.ReadyzHandler())
	mux.Handle("/livez", observability.LivezHandler())
	mux.Handle("/metrics", svc.Metrics.Handler())

	// SPA static files (catch-all for non-API routes).
	if svc.StaticFS != nil {
		mux.Handle("/", SPAHandler(svc.StaticFS))
	}

	// CORS middleware (only active when origins are configured).
	corsCfg := middleware.DefaultCORSConfig()
	corsCfg.AllowedOrigins = svc.CORSOrigins

	// Apply global middleware: CORS → security headers → recover → request ID → rate limit → metrics → logging.
	handler := middleware.Chain(mux,
		middleware.CORS(corsCfg),
		middleware.SecurityHeaders(svc.TLSEnabled),
		middleware.Recover(svc.Logger),
		middleware.RequestID(),
		rateLimiter.Middleware(
			"/api/v1/auth/login",
			"/api/v1/auth/totp/verify",
			"/api/v1/api-keys",
			"/api/v1/enroll",
		),
		middleware.Metrics(svc.Metrics),
		middleware.Logging(svc.Logger),
	)

	return handler
}
