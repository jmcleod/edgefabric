package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Ensure DefaultProvisioner implements Service at compile time.
var _ Service = (*DefaultProvisioner)(nil)

// WireGuardConfigGenerator generates WireGuard configuration for nodes.
// Defined here as a narrow interface to avoid circular imports with the
// networking package, which imports provisioning for key generation.
type WireGuardConfigGenerator interface {
	GenerateNodeConfig(ctx context.Context, nodeID domain.ID) (string, error)
}

// DefaultProvisioner implements the Service interface.
type DefaultProvisioner struct {
	nodes         storage.NodeStore
	jobs          storage.ProvisioningJobStore
	tokens        storage.EnrollmentTokenStore
	peers         storage.WireGuardPeerStore
	sshKeys       storage.SSHKeyStore
	sshClient     ssh.Client
	secrets       *secrets.Store
	wgConfig      config.WireGuardHub
	extURL        string // Controller external URL for node config.
	binaryPath    string // Local path to the edgefabric binary for SCP upload.
	wgConfigGen   WireGuardConfigGenerator // Optional: for WireGuard config sync.
}

// NewProvisioner creates a new DefaultProvisioner.
func NewProvisioner(
	nodes storage.NodeStore,
	jobs storage.ProvisioningJobStore,
	tokens storage.EnrollmentTokenStore,
	peers storage.WireGuardPeerStore,
	sshKeys storage.SSHKeyStore,
	sshClient ssh.Client,
	secrets *secrets.Store,
	wgConfig config.WireGuardHub,
	externalURL string,
) *DefaultProvisioner {
	return &DefaultProvisioner{
		nodes:     nodes,
		jobs:      jobs,
		tokens:    tokens,
		peers:     peers,
		sshKeys:   sshKeys,
		sshClient: sshClient,
		secrets:   secrets,
		wgConfig:  wgConfig,
		extURL:    externalURL,
	}
}

// SetBinaryPath sets the local path to the edgefabric binary for SCP upload
// during enrollment and upgrade operations.
func (p *DefaultProvisioner) SetBinaryPath(path string) {
	p.binaryPath = path
}

// SetWireGuardConfigGenerator sets the WireGuard config generator for config sync.
// This is called after construction to avoid circular dependencies between
// the provisioning and networking packages.
func (p *DefaultProvisioner) SetWireGuardConfigGenerator(gen WireGuardConfigGenerator) {
	p.wgConfigGen = gen
}

// --- Lifecycle actions ---

func (p *DefaultProvisioner) EnrollNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionEnroll)
}

func (p *DefaultProvisioner) StartNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionStart)
}

func (p *DefaultProvisioner) StopNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionStop)
}

func (p *DefaultProvisioner) RestartNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionRestart)
}

func (p *DefaultProvisioner) UpgradeNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionUpgrade)
}

func (p *DefaultProvisioner) ReprovisionNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionReprovision)
}

func (p *DefaultProvisioner) DecommissionNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error) {
	return p.startAction(ctx, nodeID, initiatedBy, domain.ProvisionActionDecommission)
}

// startAction validates the transition, rejects concurrent jobs, creates a job,
// and runs the pipeline.
func (p *DefaultProvisioner) startAction(ctx context.Context, nodeID, initiatedBy domain.ID, action domain.ProvisioningAction) (*domain.ProvisioningJob, error) {
	// Load node.
	node, err := p.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	// Validate state transition.
	if !domain.IsValidTransition(action, node.Status) {
		return nil, fmt.Errorf("%w: cannot %s node in %s state", storage.ErrConflict, action, node.Status)
	}

	// Reject concurrent jobs.
	_, err = p.jobs.GetActiveProvisioningJob(ctx, nodeID)
	if err == nil {
		return nil, fmt.Errorf("%w: node already has an active provisioning job", storage.ErrConflict)
	}
	if err != storage.ErrNotFound {
		return nil, fmt.Errorf("check active job: %w", err)
	}

	// Create job.
	pipeline := PipelineFor(action)
	firstStep := domain.ProvisioningStep("")
	if len(pipeline) > 0 {
		firstStep = pipeline[0]
	}

	job := &domain.ProvisioningJob{
		ID:          domain.NewID(),
		NodeID:      nodeID,
		TenantID:    node.TenantID,
		Action:      action,
		Status:      domain.ProvisionStatusPending,
		CurrentStep: firstStep,
		InitiatedBy: initiatedBy,
	}
	if err := p.jobs.CreateProvisioningJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	// Run pipeline in background goroutine.
	go p.runPipeline(context.Background(), job, node, pipeline)

	return job, nil
}

// runPipeline executes each step in sequence, recording results.
func (p *DefaultProvisioner) runPipeline(ctx context.Context, job *domain.ProvisioningJob, node *domain.Node, steps []domain.ProvisioningStep) {
	now := time.Now().UTC()
	job.Status = domain.ProvisionStatusRunning
	job.StartedAt = &now
	_ = p.jobs.UpdateProvisioningJob(ctx, job)

	// Update node status for enroll action.
	if job.Action == domain.ProvisionActionEnroll {
		node.Status = domain.NodeStatusEnrolling
		_ = p.nodes.UpdateNode(ctx, node)
	}

	var results []domain.StepResult

	for _, step := range steps {
		job.CurrentStep = step
		stepsJSON, _ := json.Marshal(results)
		job.Steps = stepsJSON
		_ = p.jobs.UpdateProvisioningJob(ctx, job)

		stepStart := time.Now().UTC()
		output, err := p.executeStep(ctx, step, job, node)
		duration := time.Since(stepStart).Milliseconds()

		result := domain.StepResult{
			Step:       step,
			StartedAt:  stepStart,
			DurationMs: duration,
			Output:     output,
		}

		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			results = append(results, result)

			// On upgrade failure past the backup step, attempt automatic rollback.
			if job.Action == domain.ProvisionActionUpgrade && p.shouldRollback(steps, step) {
				rbStart := time.Now().UTC()
				rbOutput, rbErr := p.stepRollback(ctx, node)
				rbResult := domain.StepResult{
					Step:       domain.StepRollback,
					StartedAt:  rbStart,
					DurationMs: time.Since(rbStart).Milliseconds(),
					Output:     rbOutput,
				}
				if rbErr != nil {
					rbResult.Status = "failed"
					rbResult.Error = rbErr.Error()
				} else {
					rbResult.Status = "success"
				}
				results = append(results, rbResult)
			}

			// Record failure and exit.
			completedAt := time.Now().UTC()
			job.Status = domain.ProvisionStatusFailed
			job.Error = fmt.Sprintf("step %s failed: %s", step, err.Error())
			job.CompletedAt = &completedAt
			stepsJSON, _ = json.Marshal(results)
			job.Steps = stepsJSON
			_ = p.jobs.UpdateProvisioningJob(ctx, job)

			// Set node to error state on failure.
			if job.Action == domain.ProvisionActionEnroll {
				node.Status = domain.NodeStatusError
				_ = p.nodes.UpdateNode(ctx, node)
			}
			return
		}

		result.Status = "success"
		results = append(results, result)
	}

	// All steps completed.
	completedAt := time.Now().UTC()
	job.Status = domain.ProvisionStatusCompleted
	job.CompletedAt = &completedAt
	stepsJSON, _ := json.Marshal(results)
	job.Steps = stepsJSON
	_ = p.jobs.UpdateProvisioningJob(ctx, job)

	// Update node status based on action.
	p.updateNodeStatusAfterCompletion(ctx, node, job.Action)
}

// updateNodeStatusAfterCompletion sets node status after successful pipeline completion.
func (p *DefaultProvisioner) updateNodeStatusAfterCompletion(ctx context.Context, node *domain.Node, action domain.ProvisioningAction) {
	switch action {
	case domain.ProvisionActionEnroll, domain.ProvisionActionStart, domain.ProvisionActionRestart,
		domain.ProvisionActionUpgrade, domain.ProvisionActionReprovision:
		node.Status = domain.NodeStatusOnline
	case domain.ProvisionActionStop:
		node.Status = domain.NodeStatusOffline
	case domain.ProvisionActionDecommission:
		node.Status = domain.NodeStatusDecommissioned
	}
	_ = p.nodes.UpdateNode(ctx, node)
}

// shouldRollback returns true if the failed step occurred after a backup was
// taken, meaning a rollback is meaningful.
func (p *DefaultProvisioner) shouldRollback(steps []domain.ProvisioningStep, failedStep domain.ProvisioningStep) bool {
	backupSeen := false
	for _, s := range steps {
		if s == domain.StepBackupBinary {
			backupSeen = true
		}
		if s == failedStep {
			return backupSeen
		}
	}
	return false
}

// --- Job queries ---

func (p *DefaultProvisioner) GetJob(ctx context.Context, id domain.ID) (*domain.ProvisioningJob, error) {
	return p.jobs.GetProvisioningJob(ctx, id)
}

func (p *DefaultProvisioner) ListJobs(ctx context.Context, nodeID *domain.ID, params storage.ListParams) ([]*domain.ProvisioningJob, int, error) {
	return p.jobs.ListProvisioningJobs(ctx, nodeID, params)
}
