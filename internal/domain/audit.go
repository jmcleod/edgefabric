package domain

import (
	"encoding/json"
	"time"
)

// AuditEvent is an immutable audit log entry.
type AuditEvent struct {
	ID        ID              `json:"id" db:"id"`
	TenantID  *ID             `json:"tenant_id,omitempty" db:"tenant_id"`
	UserID    *ID             `json:"user_id,omitempty" db:"user_id"`
	APIKeyID  *ID             `json:"api_key_id,omitempty" db:"api_key_id"`
	Action    string          `json:"action" db:"action"`
	Resource  string          `json:"resource" db:"resource"`
	Details   json.RawMessage `json:"details,omitempty" db:"details"`
	SourceIP  string          `json:"source_ip" db:"source_ip"`
	Timestamp time.Time       `json:"timestamp" db:"timestamp"`
}
