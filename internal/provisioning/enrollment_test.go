package provisioning_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestEnrollmentTokenGeneration(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, _ := createTestNodeForProvisioning(t, env)
	tenantID := *node.TenantID

	token, err := env.provisioner.GenerateEnrollmentToken(ctx, tenantID, node.ID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token.Token == "" {
		t.Error("expected non-empty token string")
	}
	if token.ExpiresAt.Before(time.Now()) {
		t.Error("expected token to expire in the future")
	}
	if token.UsedAt != nil {
		t.Error("expected used_at to be nil")
	}
	if token.TargetType != domain.EnrollmentTargetNode {
		t.Errorf("expected target type node, got %s", token.TargetType)
	}
}

func TestEnrollmentTokenValidation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, _ := createTestNodeForProvisioning(t, env)
	tenantID := *node.TenantID

	token, err := env.provisioner.GenerateEnrollmentToken(ctx, tenantID, node.ID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	// Valid token should pass validation.
	validated, err := env.provisioner.ValidateEnrollmentToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if validated.ID != token.ID {
		t.Errorf("expected token ID %s, got %s", token.ID, validated.ID)
	}
}

func TestEnrollmentTokenNotFound(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.provisioner.ValidateEnrollmentToken(ctx, "nonexistent-token")
	if err == nil {
		t.Fatal("expected error for nonexistent token")
	}
}

func TestCompleteEnrollment(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, _ := createTestNodeForProvisioning(t, env)
	tenantID := *node.TenantID

	token, err := env.provisioner.GenerateEnrollmentToken(ctx, tenantID, node.ID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	// Complete enrollment.
	result, err := env.provisioner.CompleteEnrollment(ctx, token.Token)
	if err != nil {
		t.Fatalf("complete enrollment: %v", err)
	}
	if result.NodeID != node.ID {
		t.Errorf("expected node ID %s, got %s", node.ID, result.NodeID)
	}

	// Verify node is online.
	updatedNode, err := env.store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updatedNode.Status != domain.NodeStatusOnline {
		t.Errorf("expected node online, got %s", updatedNode.Status)
	}
	if updatedNode.WireGuardIP == "" {
		t.Error("expected WireGuard IP assigned")
	}

	// Verify WG peer created.
	peer, err := env.store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, node.ID)
	if err != nil {
		t.Fatalf("get WG peer: %v", err)
	}
	if peer.PublicKey == "" {
		t.Error("expected WG public key")
	}

	// Verify token is used — reuse should fail.
	_, err = env.provisioner.CompleteEnrollment(ctx, token.Token)
	if err == nil {
		t.Fatal("expected error for reused token")
	}
}
