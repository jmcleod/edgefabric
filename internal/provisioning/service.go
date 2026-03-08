// Package provisioning orchestrates node lifecycle operations.
//
// The provisioning service manages the full lifecycle of edge nodes: enrollment,
// start, stop, restart, upgrade, reprovision, and decommission. Each lifecycle
// action creates a ProvisioningJob that tracks step-by-step progress through
// a pipeline of operations (validate SSH, upload binary, write config, etc.).
//
// Enrollment tokens provide bootstrap authentication for new nodes. The controller
// generates a signed, time-limited token that the node agent presents during
// enrollment to prove authorization.
package provisioning

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the provisioning interface.
type Service interface {
	// Lifecycle actions — each creates a ProvisioningJob and runs the pipeline.
	EnrollNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	StartNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	StopNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	RestartNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	UpgradeNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	ReprovisionNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)
	DecommissionNode(ctx context.Context, nodeID, initiatedBy domain.ID) (*domain.ProvisioningJob, error)

	// Job queries.
	GetJob(ctx context.Context, id domain.ID) (*domain.ProvisioningJob, error)
	ListJobs(ctx context.Context, nodeID *domain.ID, params storage.ListParams) ([]*domain.ProvisioningJob, int, error)

	// Enrollment tokens.
	GenerateEnrollmentToken(ctx context.Context, tenantID, targetID domain.ID) (*domain.EnrollmentToken, error)
	ValidateEnrollmentToken(ctx context.Context, token string) (*domain.EnrollmentToken, error)
	CompleteEnrollment(ctx context.Context, token string) error

	// SSH key management.
	RotateSSHKey(ctx context.Context, keyID domain.ID) (*domain.SSHKey, error)
	DeploySSHKey(ctx context.Context, keyID domain.ID) error
}
