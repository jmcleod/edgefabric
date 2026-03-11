package events

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mockDialer records calls and can be configured to fail.
type mockDialer struct {
	calls     atomic.Int32
	failUntil int // fail for the first N calls (0 = never fail)
	lastAddr  string
	lastFrom  string
	lastTo    []string
	lastMsg   []byte
}

func (m *mockDialer) SendMail(addr, from string, to []string, msg []byte) error {
	call := int(m.calls.Add(1))
	m.lastAddr = addr
	m.lastFrom = from
	m.lastTo = to
	m.lastMsg = msg
	if call <= m.failUntil {
		return fmt.Errorf("smtp error (attempt %d)", call)
	}
	return nil
}

func testEmailConfig() EmailConfig {
	return EmailConfig{
		SMTPHost:   "smtp.example.com",
		SMTPPort:   587,
		Username:   "user",
		Password:   "pass",
		FromAddr:   "alerts@edgefabric.io",
		Recipients: []string{"admin@example.com"},
	}
}

func testEvent() Event {
	return Event{
		Type:      NodeStatusChanged,
		Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Severity:  SeverityCritical,
		Resource:  "node-abc-123",
		Details:   map[string]string{"old_status": "online", "new_status": "offline"},
	}
}

func TestEmailHandler_DeliverSuccess(t *testing.T) {
	dialer := &mockDialer{}
	logger := slog.Default()

	handler := NewEmailHandler(logger, testEmailConfig(), WithEmailDialer(dialer))
	err := handler(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dialer.calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", dialer.calls.Load())
	}
	if dialer.lastAddr != "smtp.example.com:587" {
		t.Fatalf("unexpected addr: %s", dialer.lastAddr)
	}
	if dialer.lastFrom != "alerts@edgefabric.io" {
		t.Fatalf("unexpected from: %s", dialer.lastFrom)
	}
	if len(dialer.lastTo) != 1 || dialer.lastTo[0] != "admin@example.com" {
		t.Fatalf("unexpected to: %v", dialer.lastTo)
	}
}

func TestEmailHandler_RetryOnFailure(t *testing.T) {
	dialer := &mockDialer{failUntil: 2}
	logger := slog.Default()

	handler := NewEmailHandler(logger, testEmailConfig(), WithEmailDialer(dialer), WithEmailRetries(2))
	err := handler(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	if dialer.calls.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", dialer.calls.Load())
	}
}

func TestEmailHandler_AllRetriesFail(t *testing.T) {
	dialer := &mockDialer{failUntil: 10}
	logger := slog.Default()

	handler := NewEmailHandler(logger, testEmailConfig(), WithEmailDialer(dialer), WithEmailRetries(2))
	err := handler(context.Background(), testEvent())
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	if !strings.Contains(err.Error(), "email delivery failed after 3 attempts") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if dialer.calls.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", dialer.calls.Load())
	}
}

func TestEmailHandler_HTMLTemplate(t *testing.T) {
	dialer := &mockDialer{}
	logger := slog.Default()

	handler := NewEmailHandler(logger, testEmailConfig(), WithEmailDialer(dialer))
	err := handler(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := string(dialer.lastMsg)

	// Verify MIME headers.
	if !strings.Contains(msg, "Content-Type: text/html; charset=UTF-8") {
		t.Error("missing HTML content type header")
	}
	if !strings.Contains(msg, "MIME-Version: 1.0") {
		t.Error("missing MIME version header")
	}

	// Verify HTML body contains event data.
	if !strings.Contains(msg, "node.status_changed") {
		t.Error("HTML body missing event type")
	}
	if !strings.Contains(msg, "node-abc-123") {
		t.Error("HTML body missing resource")
	}
	if !strings.Contains(msg, "critical") {
		t.Error("HTML body missing severity")
	}
	if !strings.Contains(msg, "#F44336") {
		t.Error("HTML body missing critical severity color")
	}
	if !strings.Contains(msg, "2026-01-15T10:30:00Z") {
		t.Error("HTML body missing timestamp")
	}
}

func TestEmailHandler_SubjectFormatting(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		contains string
	}{
		{
			name: "critical",
			event: Event{
				Type:      BGPSessionDown,
				Severity:  SeverityCritical,
				Resource:  "peer-1",
				Timestamp: time.Now(),
			},
			contains: "[CRITICAL]",
		},
		{
			name: "warning",
			event: Event{
				Type:      OverlayPeerUnreachable,
				Severity:  SeverityWarning,
				Resource:  "node-1",
				Timestamp: time.Now(),
			},
			contains: "[WARNING]",
		},
		{
			name: "info",
			event: Event{
				Type:      LeaderElected,
				Severity:  SeverityInfo,
				Resource:  "controller",
				Timestamp: time.Now(),
			},
			contains: "[INFO]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer := &mockDialer{}
			handler := NewEmailHandler(slog.Default(), testEmailConfig(), WithEmailDialer(dialer))
			if err := handler(context.Background(), tt.event); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			msg := string(dialer.lastMsg)
			// Extract Subject line.
			for _, line := range strings.Split(msg, "\r\n") {
				if strings.HasPrefix(line, "Subject: ") {
					if !strings.Contains(line, tt.contains) {
						t.Errorf("subject %q missing %q", line, tt.contains)
					}
					if !strings.Contains(line, "[EdgeFabric]") {
						t.Errorf("subject %q missing [EdgeFabric] prefix", line)
					}
					return
				}
			}
			t.Error("no Subject header found")
		})
	}
}

func TestEmailHandler_WarningSeverityColor(t *testing.T) {
	dialer := &mockDialer{}
	event := Event{
		Type:      OverlayPeerUnreachable,
		Timestamp: time.Now(),
		Severity:  SeverityWarning,
		Resource:  "node-1",
	}

	handler := NewEmailHandler(slog.Default(), testEmailConfig(), WithEmailDialer(dialer))
	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := string(dialer.lastMsg)
	if !strings.Contains(msg, "#FF9800") {
		t.Error("expected warning orange color #FF9800 in HTML")
	}
}
