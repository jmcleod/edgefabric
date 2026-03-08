package v1

import (
	"errors"
	"net/http"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/audit"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/rbac"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DNSZoneHandler handles DNS zone CRUD endpoints.
type DNSZoneHandler struct {
	svc        dns.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewDNSZoneHandler creates a new DNS zone handler.
func NewDNSZoneHandler(svc dns.Service, authorizer rbac.Authorizer, audit audit.Logger) *DNSZoneHandler {
	return &DNSZoneHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts DNS zone routes on the mux.
func (h *DNSZoneHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceDNSZone, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceDNSZone, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceDNSZone, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceDNSZone, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceDNSZone, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/tenants/{tenantId}/dns/zones", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/tenants/{tenantId}/dns/zones", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/dns/zones/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/dns/zones/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/dns/zones/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/tenants/{tenantId}/dns/zones.
func (h *DNSZoneHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req dns.CreateZoneRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.TenantID = tenantID

	zone, err := h.svc.CreateZone(r.Context(), req)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "dns_zone",
		Details:  map[string]string{"zone_id": zone.ID.String(), "zone_name": zone.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, zone)
}

// Get handles GET /api/v1/dns/zones/{id}.
func (h *DNSZoneHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	zone, err := h.svc.GetZone(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns zone not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get dns zone")
		return
	}

	apiutil.JSON(w, http.StatusOK, zone)
}

// List handles GET /api/v1/tenants/{tenantId}/dns/zones.
func (h *DNSZoneHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := apiutil.ParseID(r, "tenantId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	zones, total, err := h.svc.ListZones(r.Context(), tenantID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list dns zones")
		return
	}

	apiutil.ListJSON(w, zones, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/dns/zones/{id}.
func (h *DNSZoneHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req dns.UpdateZoneRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	zone, err := h.svc.UpdateZone(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns zone not found")
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
		Resource: "dns_zone",
		Details:  map[string]string{"zone_id": zone.ID.String(), "zone_name": zone.Name},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, zone)
}

// Delete handles DELETE /api/v1/dns/zones/{id}.
func (h *DNSZoneHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteZone(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns zone not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete dns zone")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "dns_zone",
		Details:  map[string]string{"zone_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
