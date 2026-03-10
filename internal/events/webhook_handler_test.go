package events

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebhookHandler_DeliverSuccess(t *testing.T) {
	var received webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(testLogger(), []WebhookConfig{
		{URL: srv.URL},
	})

	event := Event{
		Type:      NodeStatusChanged,
		Timestamp: time.Now(),
		Severity:  SeverityWarning,
		Resource:  "node/abc",
		Details:   map[string]string{"old": "online", "new": "offline"},
	}

	err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.Event != NodeStatusChanged {
		t.Errorf("expected event type %s, got %s", NodeStatusChanged, received.Event)
	}
	if received.Resource != "node/abc" {
		t.Errorf("expected resource node/abc, got %s", received.Resource)
	}
}

func TestWebhookHandler_HMACSignature(t *testing.T) {
	secret := "test-secret-key"
	var gotSig string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-EdgeFabric-Signature")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(testLogger(), []WebhookConfig{
		{URL: srv.URL, Secret: secret},
	})

	err := handler(context.Background(), Event{
		Type:     ProvisioningFailed,
		Severity: SeverityCritical,
		Resource: "provisioning/job-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify HMAC.
	if gotSig == "" {
		t.Fatal("expected X-EdgeFabric-Signature header")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(gotBody)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != expected {
		t.Errorf("HMAC mismatch:\n  got:  %s\n  want: %s", gotSig, expected)
	}
}

func TestWebhookHandler_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(testLogger(), []WebhookConfig{
		{URL: srv.URL},
	}, WithWebhookRetries(2))

	err := handler(context.Background(), Event{
		Type:     HealthCheckFailed,
		Severity: SeverityWarning,
		Resource: "node/health",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", attempts.Load())
	}
}

func TestWebhookHandler_MultipleEndpoints(t *testing.T) {
	var count1, count2 atomic.Int32

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count1.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count2.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	handler := NewWebhookHandler(testLogger(), []WebhookConfig{
		{URL: srv1.URL},
		{URL: srv2.URL},
	})

	err := handler(context.Background(), Event{
		Type:     NodeStatusChanged,
		Severity: SeverityInfo,
		Resource: "node/xyz",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count1.Load() != 1 {
		t.Errorf("expected 1 call to endpoint 1, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("expected 1 call to endpoint 2, got %d", count2.Load())
	}
}

func TestWebhookHandler_AllEndpointsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(testLogger(), []WebhookConfig{
		{URL: srv.URL},
	}, WithWebhookRetries(0))

	err := handler(context.Background(), Event{
		Type:     HealthCheckFailed,
		Severity: SeverityCritical,
		Resource: "node/down",
	})
	if err == nil {
		t.Error("expected error when endpoint returns 503")
	}
}
