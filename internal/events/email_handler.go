package events

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// EmailConfig defines settings for email notifications.
type EmailConfig struct {
	SMTPHost   string   `yaml:"smtp_host"`
	SMTPPort   int      `yaml:"smtp_port"`
	Username   string   `yaml:"username,omitempty"`
	Password   string   `yaml:"password,omitempty"`
	FromAddr   string   `yaml:"from_addr"`
	Recipients []string `yaml:"recipients"`
	UseTLS     bool     `yaml:"use_tls,omitempty"`
}

// SMTPDialer abstracts SMTP sending for testability.
type SMTPDialer interface {
	SendMail(addr, from string, to []string, msg []byte) error
}

// defaultSMTPDialer uses the standard library smtp.SendMail.
type defaultSMTPDialer struct {
	username string
	password string
	useTLS   bool
}

func (d *defaultSMTPDialer) SendMail(addr, from string, to []string, msg []byte) error {
	var auth smtp.Auth
	if d.username != "" {
		host := strings.Split(addr, ":")[0]
		auth = smtp.PlainAuth("", d.username, d.password, host)
	}

	if d.useTLS {
		host := strings.Split(addr, ":")[0]
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := client.Mail(from); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		for _, r := range to {
			if err := client.Rcpt(r); err != nil {
				return fmt.Errorf("smtp rcpt %s: %w", r, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := w.Write(msg); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return w.Close()
	}

	return smtp.SendMail(addr, auth, from, to, msg)
}

// EmailHandler sends event notifications via SMTP email.
type EmailHandler struct {
	config  EmailConfig
	logger  *slog.Logger
	retries int
	dialer  SMTPDialer
}

// EmailOption configures an EmailHandler.
type EmailOption func(*EmailHandler)

// WithEmailRetries sets the number of retry attempts (default 2).
func WithEmailRetries(n int) EmailOption {
	return func(h *EmailHandler) {
		h.retries = n
	}
}

// WithEmailDialer sets a custom SMTP dialer (useful for testing).
func WithEmailDialer(d SMTPDialer) EmailOption {
	return func(h *EmailHandler) {
		h.dialer = d
	}
}

// NewEmailHandler creates a handler that sends event notifications via email.
func NewEmailHandler(logger *slog.Logger, config EmailConfig, opts ...EmailOption) Handler {
	h := &EmailHandler{
		config:  config,
		logger:  logger,
		retries: 2,
		dialer: &defaultSMTPDialer{
			username: config.Username,
			password: config.Password,
			useTLS:   config.UseTLS,
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h.Handle
}

// Handle delivers the event notification via email.
func (h *EmailHandler) Handle(ctx context.Context, event Event) error {
	subject := h.formatSubject(event)
	body := h.formatHTML(event)
	msg := h.buildMIME(subject, body)

	addr := fmt.Sprintf("%s:%d", h.config.SMTPHost, h.config.SMTPPort)

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

		err := h.dialer.SendMail(addr, h.config.FromAddr, h.config.Recipients, msg)
		if err == nil {
			return nil
		}
		lastErr = err
		h.logger.Warn("email delivery attempt failed",
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()),
		)
	}
	return fmt.Errorf("email delivery failed after %d attempts: %w", h.retries+1, lastErr)
}

func (h *EmailHandler) formatSubject(event Event) string {
	return fmt.Sprintf("[EdgeFabric] [%s] %s - %s",
		strings.ToUpper(string(event.Severity)),
		readableEventType(event.Type),
		event.Resource,
	)
}

func (h *EmailHandler) buildMIME(subject, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", h.config.FromAddr))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(h.config.Recipients, ", ")))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

func (h *EmailHandler) formatHTML(event Event) string {
	severityColor := "#2196F3" // info blue
	switch event.Severity {
	case SeverityCritical:
		severityColor = "#F44336" // red
	case SeverityWarning:
		severityColor = "#FF9800" // orange
	}

	var detailRows strings.Builder
	for k, v := range event.Details {
		detailRows.WriteString(fmt.Sprintf(
			`<tr><td style="padding:8px 12px;font-weight:bold;border-bottom:1px solid #eee;">%s</td><td style="padding:8px 12px;border-bottom:1px solid #eee;">%s</td></tr>`,
			k, v,
		))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background:#f5f5f5;">
<table width="100%%" cellpadding="0" cellspacing="0" style="max-width:600px;margin:20px auto;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 4px rgba(0,0,0,0.1);">
  <tr><td style="background:%s;height:6px;"></td></tr>
  <tr><td style="padding:24px;">
    <h2 style="margin:0 0 16px;color:#333;">%s</h2>
    <table width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #eee;border-radius:4px;">
      <tr><td style="padding:8px 12px;font-weight:bold;border-bottom:1px solid #eee;">Event</td><td style="padding:8px 12px;border-bottom:1px solid #eee;">%s</td></tr>
      <tr><td style="padding:8px 12px;font-weight:bold;border-bottom:1px solid #eee;">Severity</td><td style="padding:8px 12px;border-bottom:1px solid #eee;">%s</td></tr>
      <tr><td style="padding:8px 12px;font-weight:bold;border-bottom:1px solid #eee;">Resource</td><td style="padding:8px 12px;border-bottom:1px solid #eee;">%s</td></tr>
      <tr><td style="padding:8px 12px;font-weight:bold;border-bottom:1px solid #eee;">Time</td><td style="padding:8px 12px;border-bottom:1px solid #eee;">%s</td></tr>
      %s
    </table>
  </td></tr>
  <tr><td style="padding:16px 24px;background:#f9f9f9;color:#999;font-size:12px;text-align:center;">
    EdgeFabric Alert Notification
  </td></tr>
</table>
</body>
</html>`,
		severityColor,
		readableEventType(event.Type),
		string(event.Type),
		string(event.Severity),
		event.Resource,
		event.Timestamp.Format(time.RFC3339),
		detailRows.String(),
	)
}
