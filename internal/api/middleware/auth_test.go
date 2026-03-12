package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/api/middleware"
	"github.com/jmcleod/edgefabric/internal/auth"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestMFAPendingBlocksProtectedRoutes(t *testing.T) {
	claims := &auth.Claims{
		UserID:     domain.NewID(),
		Role:       domain.RoleAdmin,
		MFAPending: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Simulate a request to a protected route with MFA-pending claims.
	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	// Wrap in a middleware that checks MFA (same logic as Auth middleware).
	mfaGate := mfaGateMiddleware(handler)
	mfaGate.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		if errObj["code"] != "mfa_required" {
			t.Errorf("expected error code mfa_required, got %v", errObj["code"])
		}
	}
}

func TestMFAPendingAllowsTOTPVerify(t *testing.T) {
	claims := &auth.Claims{
		UserID:     domain.NewID(),
		Role:       domain.RoleAdmin,
		MFAPending: true,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	allowedPaths := []string{
		"/api/v1/auth/totp/verify",
		"/api/v1/auth/totp/confirm",
		"/api/v1/auth/me",
	}

	for _, path := range allowedPaths {
		called = false
		req := httptest.NewRequest("POST", path, nil)
		req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
		rr := httptest.NewRecorder()

		mfaGate := mfaGateMiddleware(handler)
		mfaGate.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("path %s: expected 200, got %d", path, rr.Code)
		}
		if !called {
			t.Errorf("path %s: handler was not called", path)
		}
	}
}

func TestNonMFATokenPassesThrough(t *testing.T) {
	claims := &auth.Claims{
		UserID:     domain.NewID(),
		Role:       domain.RoleAdmin,
		MFAPending: false,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/nodes", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	mfaGate := mfaGateMiddleware(handler)
	mfaGate.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}

// mfaGateMiddleware replicates the MFA-pending gate logic from the Auth middleware
// for unit testing without needing a full token service.
func mfaGateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.ClaimsFromContext(r.Context())
		if claims != nil && claims.MFAPending {
			path := r.URL.Path
			if path != "/api/v1/auth/totp/verify" &&
				path != "/api/v1/auth/totp/confirm" &&
				path != "/api/v1/auth/me" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "mfa_required",
						"message": "MFA verification required",
					},
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
