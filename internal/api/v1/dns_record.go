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

// DNSRecordHandler handles DNS record CRUD endpoints.
type DNSRecordHandler struct {
	svc        dns.Service
	authorizer rbac.Authorizer
	audit      audit.Logger
}

// NewDNSRecordHandler creates a new DNS record handler.
func NewDNSRecordHandler(svc dns.Service, authorizer rbac.Authorizer, audit audit.Logger) *DNSRecordHandler {
	return &DNSRecordHandler{svc: svc, authorizer: authorizer, audit: audit}
}

// Register mounts DNS record routes on the mux.
func (h *DNSRecordHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceDNSRecord, middleware.TenantFromClaims())
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceDNSRecord, middleware.TenantFromClaims())
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceDNSRecord, middleware.TenantFromClaims())
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceDNSRecord, middleware.TenantFromClaims())
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceDNSRecord, middleware.TenantFromClaims())

	mux.Handle("POST /api/v1/dns/zones/{zoneId}/records", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/dns/zones/{zoneId}/records", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/dns/records/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("PUT /api/v1/dns/records/{id}", middleware.Chain(http.HandlerFunc(h.Update), authMW, requireUpdate))
	mux.Handle("DELETE /api/v1/dns/records/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
}

// Create handles POST /api/v1/dns/zones/{zoneId}/records.
func (h *DNSRecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	zoneID, err := apiutil.ParseID(r, "zoneId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req dns.CreateRecordRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.ZoneID = zoneID

	record, err := h.svc.CreateRecord(r.Context(), req)
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
		Action:   "create",
		Resource: "dns_record",
		Details:  map[string]string{"record_id": record.ID.String(), "zone_id": zoneID.String(), "name": record.Name, "type": string(record.Type)},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusCreated, record)
}

// Get handles GET /api/v1/dns/records/{id}.
func (h *DNSRecordHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	record, err := h.svc.GetRecord(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns record not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get dns record")
		return
	}

	apiutil.JSON(w, http.StatusOK, record)
}

// List handles GET /api/v1/dns/zones/{zoneId}/records.
func (h *DNSRecordHandler) List(w http.ResponseWriter, r *http.Request) {
	zoneID, err := apiutil.ParseID(r, "zoneId")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	params := apiutil.ParseListParams(r)

	records, total, err := h.svc.ListRecords(r.Context(), zoneID, params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list dns records")
		return
	}

	apiutil.ListJSON(w, records, total, params.Offset, params.Limit)
}

// Update handles PUT /api/v1/dns/records/{id}.
func (h *DNSRecordHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req dns.UpdateRecordRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	record, err := h.svc.UpdateRecord(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns record not found")
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
		Resource: "dns_record",
		Details:  map[string]string{"record_id": record.ID.String(), "name": record.Name, "type": string(record.Type)},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, record)
}

// Delete handles DELETE /api/v1/dns/records/{id}.
func (h *DNSRecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.svc.DeleteRecord(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "dns record not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete dns record")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		TenantID: claims.TenantID,
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "dns_record",
		Details:  map[string]string{"record_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}
