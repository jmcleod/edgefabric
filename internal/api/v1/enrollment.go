package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// EnrollmentHandler handles the unauthenticated enrollment endpoint.
// The node agent calls this with its bootstrap token after SSH-push deployment.
type EnrollmentHandler struct {
	svc      provisioning.Service
	tokenSvc *auth.TokenService
	audit    audit.Logger
}

// NewEnrollmentHandler creates a new enrollment handler.
func NewEnrollmentHandler(svc provisioning.Service, tokenSvc *auth.TokenService, auditLog audit.Logger) *EnrollmentHandler {
	return &EnrollmentHandler{svc: svc, tokenSvc: tokenSvc, audit: auditLog}
}

// Register mounts enrollment routes on the mux (no auth middleware).
func (h *EnrollmentHandler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/enroll", http.HandlerFunc(h.Enroll))
}

// enrollRequest is the request body for the enrollment endpoint.
type enrollRequest struct {
	Token string `json:"token"`
}

// enrollResponse is returned to the node agent after enrollment.
type enrollResponse struct {
	Status      string `json:"status"`
	NodeID      string `json:"node_id"`
	APIToken    string `json:"api_token"`
	WireGuardIP string `json:"wireguard_ip"`
}

// Enroll handles POST /api/v1/enroll.
// This endpoint is unauthenticated — the enrollment token serves as auth.
func (h *EnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if req.Token == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "token is required")
		return
	}

	result, err := h.svc.CompleteEnrollment(r.Context(), req.Token)
	if err != nil {
		// Audit failed enrollment. Don't include the full token to avoid log pollution.
		h.audit.Log(r.Context(), audit.Event{
			Action:   "enrollment_failed",
			Resource: "node",
			Details:  map[string]string{"error": err.Error()},
			SourceIP: r.RemoteAddr,
		})
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "invalid enrollment token")
			return
		}
		if errors.Is(err, storage.ErrConflict) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "enrollment failed")
		return
	}

	// Issue a long-lived API token for the node agent to use when polling
	// configuration endpoints. Uses the node ID as the subject with readonly
	// role so it can read its own config but nothing else.
	apiToken, err := h.tokenSvc.Issue(auth.Claims{
		UserID:   result.NodeID,
		TenantID: result.TenantID,
		Role:     domain.RoleReadOnly,
	})
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to issue api token")
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		Action:   "enrollment_completed",
		Resource: "node",
		Details:  map[string]string{"node_id": result.NodeID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, enrollResponse{
		Status:      "enrolled",
		NodeID:      result.NodeID.String(),
		APIToken:    apiToken,
		WireGuardIP: result.WireGuardIP,
	})
}
