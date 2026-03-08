package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestProvisioningJobCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)
	userID := domain.NewID()

	job := &domain.ProvisioningJob{
		ID:          domain.NewID(),
		NodeID:      nodeID,
		TenantID:    &tenantID,
		Action:      domain.ProvisionActionEnroll,
		CurrentStep: domain.StepValidateSSH,
		InitiatedBy: userID,
	}

	// Create.
	if err := store.CreateProvisioningJob(ctx, job); err != nil {
		t.Fatalf("create provisioning job: %v", err)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if job.Status != domain.ProvisionStatusPending {
		t.Errorf("expected default status pending, got %s", job.Status)
	}

	// Get.
	got, err := store.GetProvisioningJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get provisioning job: %v", err)
	}
	if got.Action != domain.ProvisionActionEnroll {
		t.Errorf("expected action enroll, got %s", got.Action)
	}
	if got.NodeID != nodeID {
		t.Errorf("expected node ID %s, got %s", nodeID, got.NodeID)
	}
	if got.TenantID == nil || *got.TenantID != tenantID {
		t.Error("expected tenant ID to match")
	}

	// Update — mark as running with steps.
	now := time.Now().UTC()
	got.Status = domain.ProvisionStatusRunning
	got.StartedAt = &now
	got.CurrentStep = domain.StepUploadBinary
	stepResults := []domain.StepResult{
		{Step: domain.StepValidateSSH, Status: "success", StartedAt: now, DurationMs: 150},
	}
	stepsJSON, _ := json.Marshal(stepResults)
	got.Steps = stepsJSON
	if err := store.UpdateProvisioningJob(ctx, got); err != nil {
		t.Fatalf("update provisioning job: %v", err)
	}

	updated, err := store.GetProvisioningJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Status != domain.ProvisionStatusRunning {
		t.Errorf("expected running status, got %s", updated.Status)
	}
	if updated.StartedAt == nil {
		t.Error("expected started_at to be set")
	}
	if updated.CurrentStep != domain.StepUploadBinary {
		t.Errorf("expected current step upload_binary, got %s", updated.CurrentStep)
	}
	if updated.Steps == nil {
		t.Error("expected steps JSON to be set")
	}

	// List by node.
	jobs, total, err := store.ListProvisioningJobs(ctx, &nodeID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list provisioning jobs: %v", err)
	}
	if total != 1 || len(jobs) != 1 {
		t.Errorf("expected 1 job, got total=%d len=%d", total, len(jobs))
	}

	// List all.
	allJobs, allTotal, err := store.ListProvisioningJobs(ctx, nil, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list all provisioning jobs: %v", err)
	}
	if allTotal != 1 || len(allJobs) != 1 {
		t.Errorf("expected 1 job (all), got total=%d len=%d", allTotal, len(allJobs))
	}
}

func TestProvisioningJobActiveJob(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)
	userID := domain.NewID()

	// No active job initially.
	_, err := store.GetActiveProvisioningJob(ctx, nodeID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Create a pending job.
	job := &domain.ProvisioningJob{
		ID:          domain.NewID(),
		NodeID:      nodeID,
		TenantID:    &tenantID,
		Action:      domain.ProvisionActionEnroll,
		CurrentStep: domain.StepValidateSSH,
		InitiatedBy: userID,
	}
	if err := store.CreateProvisioningJob(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Should find active job.
	active, err := store.GetActiveProvisioningJob(ctx, nodeID)
	if err != nil {
		t.Fatalf("get active job: %v", err)
	}
	if active.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, active.ID)
	}

	// Complete the job — active should disappear.
	now := time.Now().UTC()
	active.Status = domain.ProvisionStatusCompleted
	active.CompletedAt = &now
	if err := store.UpdateProvisioningJob(ctx, active); err != nil {
		t.Fatalf("update job: %v", err)
	}

	_, err = store.GetActiveProvisioningJob(ctx, nodeID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after completion, got %v", err)
	}
}

func TestProvisioningJobFailedJob(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)
	userID := domain.NewID()

	job := &domain.ProvisioningJob{
		ID:          domain.NewID(),
		NodeID:      nodeID,
		TenantID:    &tenantID,
		Action:      domain.ProvisionActionEnroll,
		Status:      domain.ProvisionStatusRunning,
		CurrentStep: domain.StepUploadBinary,
		InitiatedBy: userID,
	}
	if err := store.CreateProvisioningJob(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Fail the job.
	now := time.Now().UTC()
	job.Status = domain.ProvisionStatusFailed
	job.Error = "SSH connection refused"
	job.CompletedAt = &now
	if err := store.UpdateProvisioningJob(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	got, err := store.GetProvisioningJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if got.Status != domain.ProvisionStatusFailed {
		t.Errorf("expected failed status, got %s", got.Status)
	}
	if got.Error != "SSH connection refused" {
		t.Errorf("expected error message, got %s", got.Error)
	}
}

func TestProvisioningJobNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetProvisioningJob(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
