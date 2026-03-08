package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// EnrollmentHandler handles the unauthenticated enrollment endpoint.
// The node agent calls this with its bootstrap token after SSH-push deployment.
type EnrollmentHandler struct {
	svc provisioning.Service
}

// NewEnrollmentHandler creates a new enrollment handler.
func NewEnrollmentHandler(svc provisioning.Service) *EnrollmentHandler {
	return &EnrollmentHandler{svc: svc}
}

// Register mounts enrollment routes on the mux (no auth middleware).
func (h *EnrollmentHandler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/enroll", http.HandlerFunc(h.Enroll))
}

// enrollRequest is the request body for the enrollment endpoint.
type enrollRequest struct {
	Token string `json:"token"`
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

	err := h.svc.CompleteEnrollment(r.Context(), req.Token)
	if err != nil {
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

	apiutil.JSON(w, http.StatusOK, map[string]string{"status": "enrolled"})
}
