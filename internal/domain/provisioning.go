package domain

import (
	"encoding/json"
	"time"
)

// ProvisioningAction identifies the type of provisioning operation.
type ProvisioningAction string

const (
	ProvisionActionEnroll       ProvisioningAction = "enroll"
	ProvisionActionReprovision  ProvisioningAction = "reprovision"
	ProvisionActionUpgrade      ProvisioningAction = "upgrade"
	ProvisionActionStart        ProvisioningAction = "start"
	ProvisionActionStop         ProvisioningAction = "stop"
	ProvisionActionRestart      ProvisioningAction = "restart"
	ProvisionActionDecommission ProvisioningAction = "decommission"
)

// ProvisioningStatus tracks the overall status of a provisioning job.
type ProvisioningStatus string

const (
	ProvisionStatusPending   ProvisioningStatus = "pending"
	ProvisionStatusRunning   ProvisioningStatus = "running"
	ProvisionStatusCompleted ProvisioningStatus = "completed"
	ProvisionStatusFailed    ProvisioningStatus = "failed"
)

// ProvisioningStep identifies a single step in a provisioning pipeline.
type ProvisioningStep string

const (
	StepValidateSSH    ProvisioningStep = "validate_ssh"
	StepUploadBinary   ProvisioningStep = "upload_binary"
	StepWriteConfig    ProvisioningStep = "write_config"
	StepInstallSystemd ProvisioningStep = "install_systemd"
	StepStartService   ProvisioningStep = "start_service"
	StepGenerateWGKeys ProvisioningStep = "generate_wg_keys"
	StepWaitEnrollment ProvisioningStep = "wait_enrollment"
	StepVerifyOnline   ProvisioningStep = "verify_online"
	StepStopService    ProvisioningStep = "stop_service"
	StepCleanup        ProvisioningStep = "cleanup"
	StepSendCommand    ProvisioningStep = "send_command"
)

// ProvisioningJob tracks a provisioning operation on a node.
// Each node lifecycle action (enroll, upgrade, etc.) creates a separate job,
// enabling full history and retry.
type ProvisioningJob struct {
	ID          ID                 `json:"id" db:"id"`
	NodeID      ID                 `json:"node_id" db:"node_id"`
	TenantID    *ID                `json:"tenant_id,omitempty" db:"tenant_id"`
	Action      ProvisioningAction `json:"action" db:"action"`
	Status      ProvisioningStatus `json:"status" db:"status"`
	CurrentStep ProvisioningStep   `json:"current_step" db:"current_step"`
	Steps       json.RawMessage    `json:"steps,omitempty" db:"steps"`
	Error       string             `json:"error,omitempty" db:"error"`
	InitiatedBy ID                 `json:"initiated_by" db:"initiated_by"`
	StartedAt   *time.Time         `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time         `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
}

// StepResult records the outcome of a single provisioning step.
type StepResult struct {
	Step       ProvisioningStep `json:"step"`
	Status     string           `json:"status"` // "success", "failed", "skipped"
	Output     string           `json:"output,omitempty"`
	Error      string           `json:"error,omitempty"`
	StartedAt  time.Time        `json:"started_at"`
	DurationMs int64            `json:"duration_ms"`
}

// ValidActionStates maps each provisioning action to the set of node statuses
// from which the action can be triggered.
var ValidActionStates = map[ProvisioningAction][]NodeStatus{
	ProvisionActionEnroll:       {NodeStatusPending},
	ProvisionActionStart:        {NodeStatusOffline, NodeStatusError},
	ProvisionActionStop:         {NodeStatusOnline},
	ProvisionActionRestart:      {NodeStatusOnline},
	ProvisionActionUpgrade:      {NodeStatusOnline, NodeStatusOffline},
	ProvisionActionReprovision:  {NodeStatusOnline, NodeStatusOffline, NodeStatusError},
	ProvisionActionDecommission: {NodeStatusPending, NodeStatusOnline, NodeStatusOffline, NodeStatusError},
}

// IsValidTransition checks if a provisioning action is valid from the given node status.
func IsValidTransition(action ProvisioningAction, currentStatus NodeStatus) bool {
	validStates, ok := ValidActionStates[action]
	if !ok {
		return false
	}
	for _, s := range validStates {
		if s == currentStatus {
			return true
		}
	}
	return false
}
