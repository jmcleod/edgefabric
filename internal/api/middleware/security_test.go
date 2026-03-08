package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeaders()(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	expected := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":        "DENY",
		"Content-Security-Policy": "default-src 'self'",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Xss-Protection":       "0",
	}

	for header, want := range expected {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("header %s = %q, want %q", header, got, want)
		}
	}
}

func TestMaxBodySize_AcceptsSmallBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if string(body) != "hello" {
			t.Errorf("expected body 'hello', got %q", string(body))
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := MaxBodySize(1024)(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", strings.NewReader("hello"))
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMaxBodySize_RejectsOversizedBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected error reading oversized body, got nil")
			return
		}
		// MaxBytesReader returns a specific error when limit is exceeded.
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	})

	handler := MaxBodySize(5)(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", strings.NewReader("this body is way too long"))
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}
