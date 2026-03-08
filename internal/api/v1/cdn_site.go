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

// CDNSiteHandler handles CDN site CRUD endpoints.
type CDNSiteHandler struct {
	svc        cdn.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewCDNSiteHandler creates a new CDN site handler.
func NewCDNSiteHandler(svc cdn.Service, authorizer rbac.Authorizer, audit audit.Logger) *CDNSiteHandler {
	return &CDNSiteHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts CDN site routes on the mux.
func (h *CDNSiteHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceCDNSite, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceCDNSite, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceCDNSite, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceCDNSite, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceCDNSite, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/tenants/{tenantId}/cdn/sites", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/tenants/{tenantId}/cdn/sites", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/cdn/sites/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/cdn/sites/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/cdn/sites/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
	mux.Handle("POST /api/v1/cdn/sites/{id}/purge", middleware.Chain(http.HandlerFunc(h.PurgeCache), authMW, requireUpdate))
}

// Create handles POST /api/v1/tenants/{tenantId}/cdn/sites.
func (h *CDNSiteHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req cdn.CreateSiteRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.TenantID = tenantID

	site, err := h.svc.CreateSite(r.Context(), req)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "cdn_site",
		Details:  map[string]string{"site_id": site.ID.String(), "site_name": site.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, site)
}

// Get handles GET /api/v1/cdn/sites/{id}.
func (h *CDNSiteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	site, err := h.svc.GetSite(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn site not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get cdn site")
		return
	}

	apiutil.JSON(w, http.StatusOK, site)
}

// List handles GET /api/v1/tenants/{tenantId}/cdn/sites.
func (h *CDNSiteHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	sites, total, err := h.svc.ListSites(r.Context(), tenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list cdn sites")
		return
	}

	apiutil.ListJSON(w, sites, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/cdn/sites/{id}.
func (h *CDNSiteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req cdn.UpdateSiteRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	site, err := h.svc.UpdateSite(r.Context(), id, req)
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
		Action:   "update",
		Resource: "cdn_site",
		Details:  map[string]string{"site_id": site.ID.String(), "site_name": site.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, site)
}

// Delete handles DELETE /api/v1/cdn/sites/{id}.
func (h *CDNSiteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteSite(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn site not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete cdn site")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "cdn_site",
		Details:  map[string]string{"site_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// PurgeCache handles POST /api/v1/cdn/sites/{id}/purge.
func (h *CDNSiteHandler) PurgeCache(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.PurgeSiteCache(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "cdn site not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to purge cache")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "purge_cache",
		Resource: "cdn_site",
		Details:  map[string]string{"site_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
