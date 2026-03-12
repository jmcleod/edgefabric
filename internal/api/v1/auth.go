package v1

import (
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/observability"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/user"
)

// AuthHandler handles authentication and API key endpoints.
type AuthHandler struct {
	authSvc    auth.Service
	tokenSvc   *auth.TokenService
	apiKeys    storage.APIKeyStore
	userSvc    user.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
	metrics    *observability.Metrics
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authSvc auth.Service, tokenSvc *auth.TokenService, apiKeys storage.APIKeyStore, userSvc user.Service, authorizer rbac.Authorizer, audit audit.Logger, metrics *observability.Metrics) *AuthHandler {
	return &AuthHandler{
		authSvc:    authSvc,
		tokenSvc:   tokenSvc,
		apiKeys:    apiKeys,
		userSvc:    userSvc,
		authorizer: authorizer,
		audit:      audit,
		metrics:    metrics,
	}
}

// Register mounts auth routes on the mux.
func (h *AuthHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	// Public routes (no auth required).
	mux.Handle("POST /api/v1/auth/login", http.HandlerFunc(h.Login))

	// Protected routes.
	mux.Handle("GET /api/v1/auth/me", middleware.Chain(http.HandlerFunc(h.Me), authMW))
	mux.Handle("POST /api/v1/auth/totp/verify", middleware.Chain(http.HandlerFunc(h.VerifyTOTP), authMW))
	mux.Handle("POST /api/v1/auth/totp/enroll", middleware.Chain(http.HandlerFunc(h.EnrollTOTP), authMW))
	mux.Handle("POST /api/v1/auth/totp/confirm", middleware.Chain(http.HandlerFunc(h.ConfirmTOTP), authMW))

	// API key management (requires auth + permission).
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceAPIKey, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceAPIKey, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceAPIKey, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/api-keys", middleware.Chain(http.HandlerFunc(h.CreateAPIKey), authMW, requireCreate))
	mux.Handle("GET /api/v1/api-keys", middleware.Chain(http.HandlerFunc(h.ListAPIKeys), authMW, requireList))
	mux.Handle("DELETE /api/v1/api-keys/{id}", middleware.Chain(http.HandlerFunc(h.DeleteAPIKey), authMW, requireDelete))
}

// loginRequest is the body of POST /api/v1/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is the response for a successful login.
type loginResponse struct {
	Token        string `json:"token"`
	TOTPRequired bool   `json:"totp_required"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	u, err := h.authSvc.AuthenticatePassword(r.Context(), req.Email, req.Password)
	if err != nil {
		// Audit failed login attempt. Don't include UserID/TenantID to avoid
		// revealing whether the account exists.
		h.audit.Log(r.Context(), audit.Event{
			Action:   "login_failed",
			Resource: "session",
			Details:  map[string]string{"email": req.Email},
			SourceIP: r.RemoteAddr,
		})
		if h.metrics != nil {
			h.metrics.AuthFailuresTotal.WithLabelValues("login").Inc()
		}
		// Don't leak whether the email exists — always "invalid credentials".
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	// If TOTP is enabled, the client must complete a second factor.
	if u.TOTPEnabled {
		// Issue a restricted MFA-pending token. The auth middleware will block
		// this token from accessing any endpoint except TOTP verification.
		token, err := h.tokenSvc.Issue(auth.Claims{
			UserID:     u.ID,
			TenantID:   u.TenantID,
			Role:       u.Role,
			MFAPending: true,
		})
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue token")
			return
		}
		apiutil.JSON(w, http.StatusOK, loginResponse{Token: token, TOTPRequired: true})
		return
	}

	// No TOTP — issue a full session token.
	token, err := h.tokenSvc.Issue(auth.Claims{
		UserID:   u.ID,
		TenantID: u.TenantID,
		Role:     u.Role,
	})
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue token")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: u.TenantID,
		UserID:   &u.ID,
		Action:   "login",
		Resource: "session",
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, loginResponse{Token: token, TOTPRequired: false})
}

// Me handles GET /api/v1/auth/me — returns the current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	u, err := h.userSvc.Get(r.Context(), claims.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch user profile")
		return
	}

	apiutil.JSON(w, http.StatusOK, u)
}

// totpVerifyRequest is the body of POST /api/v1/auth/totp/verify.
type totpVerifyRequest struct {
	Code string `json:"code"`
}

// VerifyTOTP handles POST /api/v1/auth/totp/verify.
// Expects the Bearer token from a TOTP-required login response.
func (h *AuthHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req totpVerifyRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.authSvc.AuthenticateTOTP(r.Context(), claims.UserID, req.Code); err != nil {
		h.audit.Log(r.Context(), audit.Event{
			TenantID: claims.TenantID,
			UserID:   &claims.UserID,
			Action:   "totp_verify_failed",
			Resource: "session",
			SourceIP: r.RemoteAddr,
		})
		if h.metrics != nil {
			h.metrics.AuthFailuresTotal.WithLabelValues("totp").Inc()
		}
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid TOTP code")
		return
	}

	// Issue a fresh full session token with MFA completed.
	token, err := h.tokenSvc.Issue(auth.Claims{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Role:     claims.Role,
	})
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue token")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "totp_verify",
		Resource: "session",
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, map[string]string{"token": token})
}

// totpEnrollResponse is the response for TOTP enrollment.
type totpEnrollResponse struct {
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
}

// EnrollTOTP handles POST /api/v1/auth/totp/enroll.
func (h *AuthHandler) EnrollTOTP(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	secret, uri, err := h.authSvc.EnrollTOTP(r.Context(), claims.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to enroll TOTP")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "totp_enroll",
		Resource: "user",
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, totpEnrollResponse{Secret: secret, ProvisioningURI: uri})
}

// totpConfirmRequest is the body of POST /api/v1/auth/totp/confirm.
type totpConfirmRequest struct {
	Code string `json:"code"`
}

// ConfirmTOTP handles POST /api/v1/auth/totp/confirm.
func (h *AuthHandler) ConfirmTOTP(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req totpConfirmRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.authSvc.ConfirmTOTP(r.Context(), claims.UserID, req.Code); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid TOTP code")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "totp_enable",
		Resource: "user",
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, map[string]string{"status": "totp_enabled"})
}

// createAPIKeyRequest is the body of POST /api/v1/api-keys.
type createAPIKeyRequest struct {
	Name string      `json:"name"`
	Role domain.Role `json:"role"`
}

// createAPIKeyResponse includes the raw key (shown once to the user).
type createAPIKeyResponse struct {
	RawKey string         `json:"raw_key"`
	APIKey *domain.APIKey `json:"api_key"`
}

// CreateAPIKey handles POST /api/v1/api-keys.
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.TenantID == nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "API keys require a tenant-scoped user")
		return
	}

	var req createAPIKeyRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if req.Name == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}

	rawKey, apiKey, err := h.authSvc.GenerateAPIKey(r.Context(), *claims.TenantID, claims.UserID, req.Name, req.Role)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create API key")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "api_key",
		Details:  map[string]string{"api_key_id": apiKey.ID.String(), "name": apiKey.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, createAPIKeyResponse{RawKey: rawKey, APIKey: apiKey})
}

// ListAPIKeys handles GET /api/v1/api-keys.
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.TenantID == nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "API keys require a tenant-scoped user")
		return
	}

	params := apiutil.ParseListParams(r)

	keys, total, err := h.apiKeys.ListAPIKeys(r.Context(), *claims.TenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list API keys")
		return
	}

	apiutil.ListJSON(w, keys, total, params.Offset, params.Limit)
}

// DeleteAPIKey handles DELETE /api/v1/api-keys/{id}.
func (h *AuthHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.apiKeys.DeleteAPIKey(r.Context(), id); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete API key")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "api_key",
		Details:  map[string]string{"api_key_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
