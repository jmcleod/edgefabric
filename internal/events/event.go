// Package events provides an in-process event bus for broadcasting system
// events to subscribers. This is scaffolding for future alerting hooks
// (webhook, Slack, PagerDuty, email).
package events

import "time"

// EventType identifies a category of system event.
type EventType string

// Known event types.
const (
	NodeStatusChanged    EventType = "node.status_changed"
	GatewayStatusChanged EventType = "gateway.status_changed"
	ProvisioningFailed   EventType = "provisioning.failed"
	CertificateExpiring  EventType = "certificate.expiring"
	HealthCheckFailed    EventType = "health_check.failed"
)

// Severity indicates how urgent an event is.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event represents a system event that can be published on the bus.
type Event struct {
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Severity  Severity          `json:"severity"`
	Resource  string            `json:"resource"`
	Details   map[string]string `json:"details,omitempty"`
}
