package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// CDNOriginHandler handles CDN origin CRUD endpoints.
type CDNOriginHandler struct {
	svc        cdn.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewCDNOriginHandler creates a new CDN origin handler.
func NewCDNOriginHandler(svc cdn.Service, authorizer rbac.Authorizer, audit audit.Logger) *CDNOriginHandler {
	return &CDNOriginHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts CDN origin routes on the mux.
func (h *CDNOriginHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceCDNOrigin, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceCDNOrigin, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceCDNOrigin, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceCDNOrigin, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceCDNOrigin, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/cdn/sites/{siteId}/origins", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/cdn/sites/{siteId}/origins", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/cdn/origins/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/cdn/origins/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/cdn/origins/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/cdn/sites/{siteId}/origins.
func (h *CDNOriginHandler) Create(w http.ResponseWriter, r *http.Request) {
	siteID, err := apiutil.ParseID(r, "siteId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req cdn.CreateOriginRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.SiteID = siteID

	origin, err := h.svc.CreateOrigin(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn site not found")
			return
		}
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "cdn_origin",
		Details:  map[string]string{"origin_id": origin.ID.String(), "site_id": siteID.String(), "address": origin.Address},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, origin)
}

// Get handles GET /api/v1/cdn/origins/{id}.
func (h *CDNOriginHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	origin, err := h.svc.GetOrigin(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn origin not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get cdn origin")
		return
	}

	apiutil.JSON(w, http.StatusOK, origin)
}

// List handles GET /api/v1/cdn/sites/{siteId}/origins.
func (h *CDNOriginHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID, err := apiutil.ParseID(r, "siteId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	origins, total, err := h.svc.ListOrigins(r.Context(), siteID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list cdn origins")
		return
	}

	apiutil.ListJSON(w, origins, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/cdn/origins/{id}.
func (h *CDNOriginHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req cdn.UpdateOriginRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	origin, err := h.svc.UpdateOrigin(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn origin not found")
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
		Resource: "cdn_origin",
		Details:  map[string]string{"origin_id": origin.ID.String(), "address": origin.Address},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, origin)
}

// Delete handles DELETE /api/v1/cdn/origins/{id}.
func (h *CDNOriginHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteOrigin(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn origin not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete cdn origin")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "cdn_origin",
		Details:  map[string]string{"origin_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
