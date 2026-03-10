package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// WebhookConfig defines settings for a single webhook endpoint.
type WebhookConfig struct {
	URL    string `yaml:"url"`
	Secret string `yaml:"secret,omitempty"` // HMAC-SHA256 signing secret.
}

// WebhookHandler sends events to HTTP webhook endpoints as JSON payloads.
// It includes HMAC-SHA256 signature headers when a secret is configured,
// and retries with exponential backoff on failure.
type WebhookHandler struct {
	webhooks []WebhookConfig
	client   *http.Client
	logger   *slog.Logger
	retries  int
}

// WebhookOption configures a WebhookHandler.
type WebhookOption func(*WebhookHandler)

// WithWebhookRetries sets the number of retry attempts (default 2).
func WithWebhookRetries(n int) WebhookOption {
	return func(h *WebhookHandler) {
		h.retries = n
	}
}

// WithWebhookClient sets a custom HTTP client (useful for testing).
func WithWebhookClient(c *http.Client) WebhookOption {
	return func(h *WebhookHandler) {
		h.client = c
	}
}

// NewWebhookHandler creates a handler that POSTs event payloads to the
// configured webhook URLs.
func NewWebhookHandler(logger *slog.Logger, webhooks []WebhookConfig, opts ...WebhookOption) Handler {
	h := &WebhookHandler{
		webhooks: webhooks,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:  logger,
		retries: 2,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h.Handle
}

// webhookPayload is the JSON structure sent to webhook endpoints.
type webhookPayload struct {
	Event     EventType         `json:"event"`
	Timestamp time.Time         `json:"timestamp"`
	Severity  Severity          `json:"severity"`
	Resource  string            `json:"resource"`
	Details   map[string]string `json:"details,omitempty"`
}

// Handle delivers the event to all configured webhook endpoints.
func (h *WebhookHandler) Handle(ctx context.Context, event Event) error {
	payload := webhookPayload{
		Event:     event.Type,
		Timestamp: event.Timestamp,
		Severity:  event.Severity,
		Resource:  event.Resource,
		Details:   event.Details,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	var lastErr error
	for _, wh := range h.webhooks {
		if err := h.send(ctx, wh, body); err != nil {
			h.logger.Error("webhook delivery failed",
				slog.String("url", wh.URL),
				slog.String("event", string(event.Type)),
				slog.String("error", err.Error()),
			)
			lastErr = err
		}
	}
	return lastErr
}

func (h *WebhookHandler) send(ctx context.Context, wh WebhookConfig, body []byte) error {
	var lastErr error
	for attempt := 0; attempt <= h.retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "EdgeFabric-Webhook/1.0")

		if wh.Secret != "" {
			sig := signPayload(body, []byte(wh.Secret))
			req.Header.Set("X-EdgeFabric-Signature", "sha256="+sig)
		}

		resp, err := h.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("POST %s: %w", wh.URL, err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("POST %s: status %d", wh.URL, resp.StatusCode)
	}
	return lastErr
}

// signPayload computes an HMAC-SHA256 signature of the payload.
func signPayload(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
