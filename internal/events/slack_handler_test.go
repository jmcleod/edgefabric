package events

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSlackHandler_DeliverSuccess(t *testing.T) {
	var received slackMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewSlackHandler(testLogger(), SlackConfig{
		WebhookURL: srv.URL,
		Channel:    "#alerts",
	})

	event := Event{
		Type:      NodeStatusChanged,
		Timestamp: time.Now(),
		Severity:  SeverityCritical,
		Resource:  "node/abc",
		Details:   map[string]string{"old_status": "online", "new_status": "offline"},
	}

	err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.Channel != "#alerts" {
		t.Errorf("expected channel #alerts, got %q", received.Channel)
	}
	if len(received.Blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(received.Blocks))
	}

	// First block should be the title with severity emoji.
	titleBlock := received.Blocks[0]
	if titleBlock.Text == nil {
		t.Fatal("expected title block to have text")
	}
	if titleBlock.Text.Text == "" {
		t.Error("expected non-empty title text")
	}
}

func TestSlackHandler_NoChannel(t *testing.T) {
	var received slackMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewSlackHandler(testLogger(), SlackConfig{
		WebhookURL: srv.URL,
	})

	err := handler(context.Background(), Event{
		Type:     ProvisioningFailed,
		Severity: SeverityWarning,
		Resource: "provisioning/job-5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.Channel != "" {
		t.Errorf("expected empty channel, got %q", received.Channel)
	}
}

func TestSlackHandler_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	handler := NewSlackHandler(testLogger(), SlackConfig{
		WebhookURL: srv.URL,
	})

	err := handler(context.Background(), Event{
		Type:     HealthCheckFailed,
		Severity: SeverityCritical,
		Resource: "node/down",
	})
	if err == nil {
		t.Error("expected error when Slack returns 500")
	}
}

func TestSlackHandler_SeverityEmojis(t *testing.T) {
	tests := []struct {
		severity Severity
		emoji    string
	}{
		{SeverityInfo, ":information_source:"},
		{SeverityWarning, ":warning:"},
		{SeverityCritical, ":red_circle:"},
	}

	for _, tt := range tests {
		got := severityEmoji(tt.severity)
		if got != tt.emoji {
			t.Errorf("severityEmoji(%s) = %q, want %q", tt.severity, got, tt.emoji)
		}
	}
}

func TestReadableEventType(t *testing.T) {
	tests := []struct {
		input    EventType
		expected string
	}{
		{NodeStatusChanged, "node — status changed"},
		{ProvisioningFailed, "provisioning — failed"},
	}
	for _, tt := range tests {
		got := readableEventType(tt.input)
		if got != tt.expected {
			t.Errorf("readableEventType(%s) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
