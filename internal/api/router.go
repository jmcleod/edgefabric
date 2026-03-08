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
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/tenant"
	"github.com/jmcleod/edgefabric/internal/user"
)

// Services groups all service dependencies needed by the API router.
type Services struct {
	AuthSvc    auth.Service
	TokenSvc   *auth.TokenService
	TenantSvc  tenant.Service
	UserSvc    user.Service
	Authorizer rbac.Authorizer
	AuditLog   audit.Logger
	APIKeys    storage.APIKeyStore
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

	// Infrastructure endpoints (unauthenticated).
	mux.Handle("/healthz", svc.Health.Handler())
	mux.Handle("/metrics", svc.Metrics.Handler())

	// SPA static files (catch-all for non-API routes).
	if svc.StaticFS != nil {
		mux.Handle("/", SPAHandler(svc.StaticFS))
	}

	// Apply global middleware: recover → metrics → logging.
	// Order: outermost (recover) catches panics from all inner layers.
	handler := middleware.Chain(mux,
		middleware.Recover(svc.Logger),
		middleware.Metrics(svc.Metrics),
		middleware.Logging(svc.Logger),
	)

	return handler
}
