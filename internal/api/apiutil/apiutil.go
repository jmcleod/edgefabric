// Package apiutil provides shared helpers for API handlers.
package apiutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// maxBodySize is the maximum request body size (1MB).
const maxBodySize = 1 << 20

// Response is the standard JSON envelope for single-object responses.
type Response struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// ListResponse is the standard JSON envelope for paginated list responses.
type ListResponse struct {
	Data   any `json:"data"`
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// Error is the standard error payload.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Data: data})
}

// ListJSON writes a paginated list response.
func ListJSON(w http.ResponseWriter, data any, total, offset, limit int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ListResponse{
		Data:   data,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Error: &Error{Code: code, Message: message},
	})
}

// Decode reads and decodes a JSON request body into dst.
// It enforces a maximum body size to prevent abuse.
func Decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

// PathParam extracts a path parameter by name (Go 1.22+ routing).
func PathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

// ParseID parses a UUID path parameter.
func ParseID(r *http.Request, name string) (domain.ID, error) {
	raw := r.PathValue(name)
	if raw == "" {
		return domain.ZeroID, fmt.Errorf("missing path parameter: %s", name)
	}
	id, err := domain.ParseID(raw)
	if err != nil {
		return domain.ZeroID, fmt.Errorf("invalid UUID for %s: %w", name, err)
	}
	return id, nil
}

// ParseListParams extracts pagination parameters from query string.
// Defaults: offset=0, limit=50. Max limit=200.
func ParseListParams(r *http.Request) storage.ListParams {
	params := storage.ListParams{
		Offset: 0,
		Limit:  storage.DefaultLimit,
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			params.Offset = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			params.Limit = n
		}
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	return params
}
