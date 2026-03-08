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
	"github.com/jmcleod/edgefabric/internal/fleet"
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
	ProvisioningSvc provisioning.Service
	Authorizer      rbac.Authorizer
	AuditLog   audit.Logger
	APIKeys    storage.APIKeyStore
	SSHKeys    storage.SSHKeyStore
	Health     *observability.HealthChecker
	Metrics    *observability.Metrics
	Logger     *slog.Logger
	StaticFS   fs.FS // Embedded SPA files (web.StaticFiles)
}

// NewRouter assembles the full HTTP handler with all middleware and routes.
func NewRouter(svc Services) http.Handler {
	mux := http.NewServeMux()

	// Auth middleware (shared by all protected routes).
	authMW := middleware.Auth(svc.TokenSvc, svc.AuthSvc)

	// Register API v1 handlers.
	tenantHandler := v1.NewTenantHandler(svc.TenantSvc, svc.Authorizer, svc.AuditLog)
	tenantHandler.Register(mux, authMW)

	userHandler := v1.NewUserHandler(svc.UserSvc, svc.Authorizer, svc.AuditLog)
	userHandler.Register(mux, authMW)

	authHandler := v1.NewAuthHandler(svc.AuthSvc, svc.TokenSvc, svc.APIKeys, svc.Authorizer, svc.AuditLog)
	authHandler.Register(mux, authMW)

	auditHandler := v1.NewAuditHandler(svc.AuditLog, svc.Authorizer)
	auditHandler.Register(mux, authMW)

	// Fleet management handlers (nodes, node groups, SSH keys).
	if svc.FleetSvc != nil {
		nodeHandler := v1.NewNodeHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		nodeHandler.Register(mux, authMW)

		nodeGroupHandler := v1.NewNodeGroupHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		nodeGroupHandler.Register(mux, authMW)

		statusHandler := v1.NewStatusHandler(svc.TenantSvc, svc.UserSvc, svc.FleetSvc, svc.Authorizer)
		statusHandler.Register(mux, authMW)
	}

	if svc.SSHKeys != nil {
		sshKeyHandler := v1.NewSSHKeyHandler(svc.SSHKeys, svc.ProvisioningSvc, svc.Authorizer, svc.AuditLog)
		sshKeyHandler.Register(mux, authMW)
	}

	// Provisioning and enrollment handlers.
	if svc.ProvisioningSvc != nil {
		provisioningHandler := v1.NewProvisioningHandler(svc.ProvisioningSvc, svc.Authorizer, svc.AuditLog)
		provisioningHandler.Register(mux, authMW)

		enrollmentHandler := v1.NewEnrollmentHandler(svc.ProvisioningSvc)
		enrollmentHandler.Register(mux) // No auth — token-based.
	}

	// Gateway handler.
	if svc.FleetSvc != nil {
		gatewayHandler := v1.NewGatewayHandler(svc.FleetSvc, svc.Authorizer, svc.AuditLog)
		gatewayHandler.Register(mux, authMW)
	}

	// OpenAPI spec (unauthenticated).
	mux.HandleFunc("GET /api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapi.V1Spec)
	})

	// Infrastructure endpoints (unauthenticated).
	mux.Handle("/healthz", svc.Health.Handler())
	mux.Handle("/metrics", svc.Metrics.Handler())

	// SPA static files (catch-all for non-API routes).
	if svc.StaticFS != nil {
		mux.Handle("/", SPAHandler(svc.StaticFS))
	}

	// Apply global middleware: recover → request ID → metrics → logging.
	// Order: outermost (recover) catches panics from all inner layers.
	handler := middleware.Chain(mux,
		middleware.Recover(svc.Logger),
		middleware.RequestID(),
		middleware.Metrics(svc.Metrics),
		middleware.Logging(svc.Logger),
	)

	return handler
}
