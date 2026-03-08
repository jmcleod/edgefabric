package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/networking"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// BGPSessionHandler handles BGP session CRUD endpoints.
type BGPSessionHandler struct {
	svc        networking.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewBGPSessionHandler creates a new BGP session handler.
func NewBGPSessionHandler(svc networking.Service, authorizer rbac.Authorizer, audit audit.Logger) *BGPSessionHandler {
	return &BGPSessionHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts BGP session routes on the mux.
func (h *BGPSessionHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceBGPSession, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceBGPSession, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceBGPSession, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceBGPSession, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceBGPSession, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/nodes/{nodeId}/bgp-sessions", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/nodes/{nodeId}/bgp-sessions", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/bgp-sessions/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/bgp-sessions/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/bgp-sessions/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/nodes/{nodeId}/bgp-sessions.
func (h *BGPSessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	nodeID, err := apiutil.ParseID(r, "nodeId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req networking.CreateBGPSessionRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.NodeID = nodeID

	sess, err := h.svc.CreateBGPSession(r.Context(), req)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "bgp_session",
		Details:  map[string]string{"session_id": sess.ID.String(), "node_id": nodeID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, sess)
}

// Get handles GET /api/v1/bgp-sessions/{id}.
func (h *BGPSessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sess, err := h.svc.GetBGPSession(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "bgp session not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get bgp session")
		return
	}

	apiutil.JSON(w, http.StatusOK, sess)
}

// List handles GET /api/v1/nodes/{nodeId}/bgp-sessions.
func (h *BGPSessionHandler) List(w http.ResponseWriter, r *http.Request) {
	nodeID, err := apiutil.ParseID(r, "nodeId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	sessions, total, err := h.svc.ListBGPSessions(r.Context(), nodeID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list bgp sessions")
		return
	}

	apiutil.ListJSON(w, sessions, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/bgp-sessions/{id}.
func (h *BGPSessionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req networking.UpdateBGPSessionRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sess, err := h.svc.UpdateBGPSession(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "bgp session not found")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "update",
		Resource: "bgp_session",
		Details:  map[string]string{"session_id": sess.ID.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, sess)
}

// Delete handles DELETE /api/v1/bgp-sessions/{id}.
func (h *BGPSessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteBGPSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "bgp session not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete bgp session")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "bgp_session",
		Details:  map[string]string{"session_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
