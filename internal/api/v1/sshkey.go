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

// SSHKeyHandler handles SSH key CRUD endpoints.
// SSH keys are a global resource (not tenant-scoped) — only SuperUser can create/delete.
type SSHKeyHandler struct {
	store        storage.SSHKeyStore
	provisioner  provisioning.Service
	authorizer   rbac.Authorizer
	audit        audit.Logger
}

// NewSSHKeyHandler creates a new SSH key handler.
func NewSSHKeyHandler(store storage.SSHKeyStore, provisioner provisioning.Service, authorizer rbac.Authorizer, audit audit.Logger) *SSHKeyHandler {
	return &SSHKeyHandler{store: store, provisioner: provisioner, authorizer: authorizer, audit: audit}
}

// Register mounts SSH key routes on the mux.
func (h *SSHKeyHandler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	requireCreate := middleware.RequirePermission(h.authorizer, rbac.ActionCreate, rbac.ResourceSSHKey, nil)
	requireRead := middleware.RequirePermission(h.authorizer, rbac.ActionRead, rbac.ResourceSSHKey, nil)
	requireList := middleware.RequirePermission(h.authorizer, rbac.ActionList, rbac.ResourceSSHKey, nil)
	requireUpdate := middleware.RequirePermission(h.authorizer, rbac.ActionUpdate, rbac.ResourceSSHKey, nil)
	requireDelete := middleware.RequirePermission(h.authorizer, rbac.ActionDelete, rbac.ResourceSSHKey, nil)

	mux.Handle("POST /api/v1/ssh-keys", middleware.Chain(http.HandlerFunc(h.Create), authMW, requireCreate))
	mux.Handle("GET /api/v1/ssh-keys", middleware.Chain(http.HandlerFunc(h.List), authMW, requireList))
	mux.Handle("GET /api/v1/ssh-keys/{id}", middleware.Chain(http.HandlerFunc(h.Get), authMW, requireRead))
	mux.Handle("DELETE /api/v1/ssh-keys/{id}", middleware.Chain(http.HandlerFunc(h.Delete), authMW, requireDelete))
	mux.Handle("POST /api/v1/ssh-keys/{id}/rotate", middleware.Chain(http.HandlerFunc(h.Rotate), authMW, requireUpdate))
	mux.Handle("POST /api/v1/ssh-keys/{id}/deploy", middleware.Chain(http.HandlerFunc(h.Deploy), authMW, requireUpdate))
}

// createSSHKeyRequest is the input for creating an SSH key.
type createSSHKeyRequest struct {
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"` // Must be pre-encrypted by the caller or will be encrypted at rest
}

// Create handles POST /api/v1/ssh-keys.
func (h *SSHKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSSHKeyRequest
	if err := apiutil.Decode(r, &req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if req.Name == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.PublicKey == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "public_key is required")
		return
	}
	if req.PrivateKey == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", "private_key is required")
		return
	}

	k := &domain.SSHKey{
		ID:         domain.NewID(),
		Name:       req.Name,
		PublicKey:   req.PublicKey,
		PrivateKey: req.PrivateKey,
		// Fingerprint should be computed from the public key; placeholder for v1.
		Fingerprint: "SHA256:placeholder",
	}

	if err := h.store.CreateSSHKey(r.Context(), k); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			apiutil.WriteError(w, http.StatusConflict, "conflict", "SSH key already exists")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create SSH key")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "create",
		Resource: "ssh_key",
		Details:  map[string]string{"key_id": k.ID.String(), "name": k.Name},
		SourceIP: r.RemoteAddr,
	})

	// Don't return private key in response.
	k.PrivateKey = ""
	apiutil.JSON(w, http.StatusCreated, k)
}

// Get handles GET /api/v1/ssh-keys/{id}.
func (h *SSHKeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	k, err := h.store.GetSSHKey(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "SSH key not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get SSH key")
		return
	}

	// Never expose private key in API responses.
	k.PrivateKey = ""
	apiutil.JSON(w, http.StatusOK, k)
}

// List handles GET /api/v1/ssh-keys.
func (h *SSHKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	params := apiutil.ParseListParams(r)

	keys, total, err := h.store.ListSSHKeys(r.Context(), params)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list SSH keys")
		return
	}

	// Strip private keys from response.
	for _, k := range keys {
		k.PrivateKey = ""
	}

	apiutil.ListJSON(w, keys, total, params.Offset, params.Limit)
}

// Delete handles DELETE /api/v1/ssh-keys/{id}.
func (h *SSHKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.store.DeleteSSHKey(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "SSH key not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete SSH key")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "delete",
		Resource: "ssh_key",
		Details:  map[string]string{"key_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

// Rotate handles POST /api/v1/ssh-keys/{id}/rotate.
// Generates a new key pair and updates the stored key.
func (h *SSHKeyHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if h.provisioner == nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "service_unavailable", "provisioning service not available")
		return
	}

	key, err := h.provisioner.RotateSSHKey(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "SSH key not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to rotate SSH key")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "rotate",
		Resource: "ssh_key",
		Details:  map[string]string{"key_id": id.String(), "fingerprint": key.Fingerprint},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, key)
}

// Deploy handles POST /api/v1/ssh-keys/{id}/deploy.
// Pushes the current public key to all nodes using this SSH key.
func (h *SSHKeyHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	id, err := apiutil.ParseID(r, "id")
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if h.provisioner == nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "service_unavailable", "provisioning service not available")
		return
	}

	if err := h.provisioner.DeploySSHKey(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, "not_found", "SSH key not found")
			return
		}
		apiutil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deploy SSH key")
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	h.audit.Log(r.Context(), audit.Event{
		UserID:   &claims.UserID,
		Action:   "deploy",
		Resource: "ssh_key",
		Details:  map[string]string{"key_id": id.String()},
		SourceIP: r.RemoteAddr,
	})

	apiutil.JSON(w, http.StatusOK, map[string]string{"status": "deployed"})
}
