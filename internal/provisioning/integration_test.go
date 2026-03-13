package provisioning_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// TestFullEnrollmentFlow tests the complete enrollment lifecycle:
// create node → trigger enroll → mock SSH succeeds → complete enrollment
// → verify WG peer created → verify node online.
func TestFullEnrollmentFlow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)
	tenantID := *node.TenantID

	// Step 1: Generate enrollment token.
	token, err := env.provisioner.GenerateEnrollmentToken(ctx, tenantID, node.ID)
	if err != nil {
		t.Fatalf("generate enrollment token: %v", err)
	}
	if token.Token == "" {
		t.Fatal("expected non-empty enrollment token")
	}
	if token.TargetType != domain.EnrollmentTargetNode {
		t.Errorf("expected target type node, got %s", token.TargetType)
	}

	// Step 2: Validate the token.
	validated, err := env.provisioner.ValidateEnrollmentToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if validated.ID != token.ID {
		t.Errorf("token ID mismatch: %s vs %s", token.ID, validated.ID)
	}

	// Step 3: Configure mock SSH to succeed.
	env.sshSession.RunFunc = func(cmd string) (string, error) {
		return "edgefabric-ssh-ok Linux edge1 5.15\nactive", nil
	}

	// Step 4: Trigger enroll via provisioning service.
	job, err := env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("enroll node: %v", err)
	}
	if job.Action != domain.ProvisionActionEnroll {
		t.Errorf("expected enroll action, got %s", job.Action)
	}

	// Step 5: Wait for async pipeline to complete.
	completedJob := waitForJobDone(t, env, job.ID, 5*time.Second)
	if completedJob.Status != domain.ProvisionStatusCompleted {
		t.Fatalf("expected job completed, got %s (error: %s)", completedJob.Status, completedJob.Error)
	}
	if completedJob.StartedAt == nil {
		t.Error("expected started_at to be set")
	}
	if completedJob.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	// Step 7: Verify node is online with WireGuard IP.
	enrolledNode, err := env.store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if enrolledNode.Status != domain.NodeStatusOnline {
		t.Errorf("expected node online, got %s", enrolledNode.Status)
	}
	if enrolledNode.WireGuardIP == "" {
		t.Error("expected WireGuard IP to be assigned")
	}

	// Step 8: Verify WireGuard peer was created.
	peer, err := env.store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, node.ID)
	if err != nil {
		t.Fatalf("get WG peer: %v", err)
	}
	if peer.PublicKey == "" {
		t.Error("expected WG public key")
	}
	if peer.Endpoint == "" {
		t.Error("expected WG endpoint (node public IP)")
	}

	// Step 9: Complete enrollment using the token we generated earlier.
	_, err = env.provisioner.CompleteEnrollment(ctx, token.Token)
	if err != nil {
		t.Fatalf("complete enrollment: %v", err)
	}

	// Step 10: Token reuse should fail.
	_, err = env.provisioner.CompleteEnrollment(ctx, token.Token)
	if err == nil {
		t.Error("expected error for reused enrollment token")
	}

	// Step 11: Verify job history is queryable.
	jobs, total, err := env.provisioner.ListJobs(ctx, &node.ID, storage.ListParams{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if total == 0 {
		t.Error("expected at least one job in history")
	}
	if len(jobs) == 0 {
		t.Error("expected at least one job returned")
	}
}

// TestDecommissionFlow verifies the decommission lifecycle:
// enroll → decommission → verify cleanup.
func TestDecommissionFlow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)

	// Configure mock SSH to succeed.
	env.sshSession.RunFunc = func(cmd string) (string, error) {
		return "edgefabric-ssh-ok\nactive", nil
	}

	// Enroll the node first.
	job, err := env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	updated := waitForJobDone(t, env, job.ID, 5*time.Second)
	if updated.Status != domain.ProvisionStatusCompleted {
		t.Fatalf("enroll job not completed: %s (error: %s)", updated.Status, updated.Error)
	}

	// Verify node is online.
	onlineNode, _ := env.store.GetNode(ctx, node.ID)
	if onlineNode.Status != domain.NodeStatusOnline {
		t.Fatalf("expected online, got %s", onlineNode.Status)
	}

	// Now decommission.
	decommJob, err := env.provisioner.DecommissionNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("decommission: %v", err)
	}

	updatedDecomm := waitForJobDone(t, env, decommJob.ID, 5*time.Second)
	if updatedDecomm.Status != domain.ProvisionStatusCompleted {
		t.Fatalf("decommission job not completed: %s (error: %s)", updatedDecomm.Status, updatedDecomm.Error)
	}

	// Verify node is decommissioned.
	decomNode, _ := env.store.GetNode(ctx, node.ID)
	if decomNode.Status != domain.NodeStatusDecommissioned {
		t.Errorf("expected decommissioned, got %s", decomNode.Status)
	}
}
