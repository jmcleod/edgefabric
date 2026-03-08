package apiutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestHandleServiceError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	HandleServiceError(w, storage.ErrNotFound, "node")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "not_found" {
		t.Errorf("expected error code 'not_found', got %+v", resp.Error)
	}
	if resp.Error != nil && !strings.Contains(resp.Error.Message, "node not found") {
		t.Errorf("expected message containing 'node not found', got %q", resp.Error.Message)
	}
}

func TestHandleServiceError_AlreadyExists(t *testing.T) {
	w := httptest.NewRecorder()
	HandleServiceError(w, storage.ErrAlreadyExists, "tenant")

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "conflict" {
		t.Errorf("expected error code 'conflict', got %+v", resp.Error)
	}
	if resp.Error != nil && !strings.Contains(resp.Error.Message, "tenant already exists") {
		t.Errorf("expected message containing 'tenant already exists', got %q", resp.Error.Message)
	}
}

func TestHandleServiceError_Conflict(t *testing.T) {
	w := httptest.NewRecorder()
	HandleServiceError(w, fmt.Errorf("version mismatch: %w", storage.ErrConflict), "gateway")

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "conflict" {
		t.Errorf("expected error code 'conflict', got %+v", resp.Error)
	}
}

func TestHandleServiceError_Validation(t *testing.T) {
	w := httptest.NewRecorder()
	HandleServiceError(w, fmt.Errorf("name too long: %w", storage.ErrValidation), "user")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "validation_error" {
		t.Errorf("expected error code 'validation_error', got %+v", resp.Error)
	}
}

func TestHandleServiceError_Unknown(t *testing.T) {
	w := httptest.NewRecorder()
	HandleServiceError(w, fmt.Errorf("database timeout"), "node")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil || resp.Error.Code != "internal_error" {
		t.Errorf("expected error code 'internal_error', got %+v", resp.Error)
	}
	if resp.Error != nil && resp.Error.Message != "unexpected error" {
		t.Errorf("expected generic message 'unexpected error', got %q", resp.Error.Message)
	}
}

func TestHandleServiceError_WrappedNotFound(t *testing.T) {
	// Ensure errors.Is works through wrapping.
	wrapped := fmt.Errorf("get node by ID: %w", storage.ErrNotFound)
	w := httptest.NewRecorder()
	HandleServiceError(w, wrapped, "node")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrapped ErrNotFound, got %d", w.Code)
	}
}

func TestParseListParams_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/items", nil)
	params := ParseListParams(r)

	if params.Offset != 0 {
		t.Errorf("expected offset 0, got %d", params.Offset)
	}
	if params.Limit != 50 {
		t.Errorf("expected limit 50, got %d", params.Limit)
	}
}

func TestParseListParams_CustomValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?offset=10&limit=25", nil)
	params := ParseListParams(r)

	if params.Offset != 10 {
		t.Errorf("expected offset 10, got %d", params.Offset)
	}
	if params.Limit != 25 {
		t.Errorf("expected limit 25, got %d", params.Limit)
	}
}

func TestParseListParams_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?offset=-5", nil)
	params := ParseListParams(r)

	// Negative offset should be ignored, keeping default 0.
	if params.Offset != 0 {
		t.Errorf("expected offset 0 for negative value, got %d", params.Offset)
	}
}

func TestParseListParams_LimitClampedTo200(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?limit=500", nil)
	params := ParseListParams(r)

	if params.Limit != 200 {
		t.Errorf("expected limit clamped to 200, got %d", params.Limit)
	}
}

func TestParseListParams_ZeroLimit(t *testing.T) {
	// Zero limit should be ignored, keeping default 50.
	r := httptest.NewRequest("GET", "/items?limit=0", nil)
	params := ParseListParams(r)

	if params.Limit != 50 {
		t.Errorf("expected limit 50 for zero value, got %d", params.Limit)
	}
}

func TestParseListParams_InvalidValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?offset=abc&limit=xyz", nil)
	params := ParseListParams(r)

	if params.Offset != 0 {
		t.Errorf("expected offset 0 for invalid value, got %d", params.Offset)
	}
	if params.Limit != 50 {
		t.Errorf("expected limit 50 for invalid value, got %d", params.Limit)
	}
}

func TestDecode_ValidBody(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	r := httptest.NewRequest("POST", "/", body)

	var dst struct {
		Name string `json:"name"`
	}
	if err := Decode(r, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected name 'test', got %q", dst.Name)
	}
}

func TestDecode_OversizedBody(t *testing.T) {
	// Create a body larger than 1MB.
	bigBody := strings.NewReader(strings.Repeat("x", 2<<20))
	r := httptest.NewRequest("POST", "/", bigBody)

	var dst map[string]any
	err := Decode(r, &dst)
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
}

func TestDecode_UnknownFields(t *testing.T) {
	body := strings.NewReader(`{"name":"test","unknown_field":"value"}`)
	r := httptest.NewRequest("POST", "/", body)

	var dst struct {
		Name string `json:"name"`
	}
	err := Decode(r, &dst)
	if err == nil {
		t.Fatal("expected error for unknown fields")
	}
}
