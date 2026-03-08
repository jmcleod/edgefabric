package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// ProvisioningHandler handles provisioning lifecycle endpoints.
type ProvisioningHandler struct {
	svc        provisioning.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewProvisioningHandler creates a new provisioning handler.
func NewProvisioningHandler(svc provisioning.Service, authorizer rbac.Authorizer, audit audit.Logger) *ProvisioningHandler {
	return &ProvisioningHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts provisioning routes on the mux.
func (h *ProvisioningHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceProvisioningJob, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceProvisioningJob, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceProvisioningJob, middleware.TenantFromClaims())

	// Lifecycle action: POST /api/v1/nodes/{id}/{action}
	mux.Handle("POST /api/v1/nodes/{id}/enroll", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/start", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/stop", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/restart", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/upgrade", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/reprovision", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))
	mux.Handle("POST /api/v1/nodes/{id}/decommission", middleware.Chain(http.HandlerFunc(h.Action), authMW, requireCreate))

	// Job queries.
	mux.Handle("GET /api/v1/nodes/{id}/jobs", middleware.Chain(http.HandlerFunc(h.ListNodeJobs), authMW, requireList))
	mux.Handle("GET /api/v1/provisioning/jobs/{id}", middleware.Chain(http.HandlerFunc(h.GetJob), authMW, requireRead))
}

// Action handles POST /api/v1/nodes/{id}/{action}.
func (h *ProvisioningHandler) Action(w http.ResponseWriter, r *http.Request) {
	nodeID, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Extract action from the URL path.
	action := extractAction(r.URL.Path)
	if action == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "unknown action")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	userID := claims.UserID

	var job *domain.ProvisioningJob

	switch domain.ProvisioningAction(action) {
	case domain.ProvisionActionEnroll:
		job, err = h.svc.EnrollNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionStart:
		job, err = h.svc.StartNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionStop:
		job, err = h.svc.StopNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionRestart:
		job, err = h.svc.RestartNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionUpgrade:
		job, err = h.svc.UpgradeNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionReprovision:
		job, err = h.svc.ReprovisionNode(r.Context(), nodeID, userID)
	case domain.ProvisionActionDecommission:
		job, err = h.svc.DecommissionNode(r.Context(), nodeID, userID)
	default:
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "unknown action: "+action)
		return
	}

	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		if errors.Is(err, storage.ErrConflict) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   action,
		Resource: "provisioning_job",
		Details:  map[string]string{"node_id": nodeID.String(), "job_id": job.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusAccepted, job)
}

// ListNodeJobs handles GET /api/v1/nodes/{id}/jobs.
func (h *ProvisioningHandler) ListNodeJobs(w http.ResponseWriter, r *http.Request) {
	nodeID, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)
	jobs, total, err := h.svc.ListJobs(r.Context(), &nodeID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list jobs")
		return
	}

	apiutil.ListJSON(w, jobs, total, params.Offset, params.Limit)
}

// GetJob handles GET /api/v1/provisioning/jobs/{id}.
func (h *ProvisioningHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	job, err := h.svc.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "job not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get job")
		return
	}

	apiutil.JSON(w, http.StatusOK, job)
}

// extractAction extracts the last path segment (the action) from a URL path.
func extractAction(path string) string {
	// Path is like /api/v1/nodes/{id}/enroll
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return ""
}
