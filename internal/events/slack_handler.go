package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SlackConfig defines settings for Slack notifications.
type SlackConfig struct {
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel,omitempty"` // Optional channel override.
}

// SlackHandler sends event notifications to a Slack webhook.
type SlackHandler struct {
	config SlackConfig
	client *http.Client
	logger *slog.Logger
}

// SlackOption configures a SlackHandler.
type SlackOption func(*SlackHandler)

// WithSlackClient sets a custom HTTP client (useful for testing).
func WithSlackClient(c *http.Client) SlackOption {
	return func(h *SlackHandler) {
		h.client = c
	}
}

// NewSlackHandler creates a handler that sends event notifications to Slack
// using the Block Kit format.
func NewSlackHandler(logger *slog.Logger, config SlackConfig, opts ...SlackOption) Handler {
	h := &SlackHandler{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h.Handle
}

// Handle formats the event and sends it to Slack.
func (h *SlackHandler) Handle(ctx context.Context, event Event) error {
	msg := h.formatMessage(event)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// slackMessage is the Slack Block Kit message format.
type slackMessage struct {
	Channel string       `json:"channel,omitempty"`
	Blocks  []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type string      `json:"type"`
	Text *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (h *SlackHandler) formatMessage(event Event) slackMessage {
	emoji := severityEmoji(event.Severity)
	title := fmt.Sprintf("%s *%s*", emoji, readableEventType(event.Type))

	blocks := []slackBlock{
		{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: title},
		},
		{
			Type: "section",
			Fields: []slackText{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Severity:*\n%s", event.Severity)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Resource:*\n%s", event.Resource)},
			},
		},
	}

	// Add details as a field section if present.
	if len(event.Details) > 0 {
		var fields []slackText
		for k, v := range event.Details {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%s", k, v),
			})
		}
		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	// Timestamp context.
	blocks = append(blocks, slackBlock{
		Type: "context",
		Fields: []slackText{
			{Type: "mrkdwn", Text: fmt.Sprintf("EdgeFabric | %s", event.Timestamp.Format(time.RFC3339))},
		},
	})

	msg := slackMessage{Blocks: blocks}
	if h.config.Channel != "" {
		msg.Channel = h.config.Channel
	}
	return msg
}

func severityEmoji(s Severity) string {
	switch s {
	case SeverityCritical:
		return ":red_circle:"
	case SeverityWarning:
		return ":warning:"
	default:
		return ":information_source:"
	}
}

func readableEventType(t EventType) string {
	s := string(t)
	s = strings.ReplaceAll(s, ".", " — ")
	s = strings.ReplaceAll(s, "_", " ")
	return s
}
