package provisioning_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcleod/edgefabric/internal/config"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/provisioning"
	"github.com/jmcleod/edgefabric/internal/secrets"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

// testEnv holds the full test provisioning environment.
type testEnv struct {
	store       *sqlite.SQLiteStore
	provisioner *provisioning.DefaultProvisioner
	sshClient   *ssh.MockClient
	sshSession  *ssh.MockSession
	secrets     *secrets.Store
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Create secrets store with a test key.
	sec, err := secrets.NewStore("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("create secrets store: %v", err)
	}

	sshClient := ssh.NewMockClient()

	wgConfig := config.WireGuardHub{
		ListenPort: 51820,
		Subnet:     "10.100.0.0/24",
		Address:    "10.100.0.1/24",
	}

	p := provisioning.NewProvisioner(
		store,   // NodeStore
		store,   // ProvisioningJobStore
		store,   // EnrollmentTokenStore
		store,   // WireGuardPeerStore
		store,   // SSHKeyStore
		sshClient,
		sec,
		wgConfig,
		"https://controller.example.com",
	)

	// Create a temporary fake binary for upload tests.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "edgefabric")
	if err := os.WriteFile(fakeBinary, []byte("fake-binary-content"), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	p.SetBinaryPath(fakeBinary)

	return &testEnv{
		store:       store,
		provisioner: p,
		sshClient:   sshClient,
		sshSession:  sshClient.Session,
		secrets:     sec,
	}
}

// createTestNodeForProvisioning creates a tenant, SSH key, and node ready for provisioning.
func createTestNodeForProvisioning(t *testing.T, env *testEnv) (*domain.Node, domain.ID) {
	t.Helper()
	ctx := context.Background()

	tenant := &domain.Tenant{
		ID:   domain.NewID(),
		Name: "test-tenant-" + domain.NewID().String()[:8],
		Slug: "test-" + domain.NewID().String()[:8],
	}
	if err := env.store.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// Create an encrypted SSH key.
	encPriv, _ := env.secrets.Encrypt("-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key-data\n-----END OPENSSH PRIVATE KEY-----")
	sshKey := &domain.SSHKey{
		ID:          domain.NewID(),
		Name:        "test-key",
		PublicKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey",
		PrivateKey:  encPriv,
		Fingerprint: "SHA256:testfp",
	}
	if err := env.store.CreateSSHKey(ctx, sshKey); err != nil {
		t.Fatalf("create ssh key: %v", err)
	}

	node := &domain.Node{
		ID:       domain.NewID(),
		TenantID: &tenant.ID,
		Name:     "edge-node-1",
		Hostname: "edge1.example.com",
		PublicIP: "203.0.113.10",
		Status:   domain.NodeStatusPending,
		SSHPort:  22,
		SSHUser:  "root",
		SSHKeyID: &sshKey.ID,
	}
	if err := env.store.CreateNode(ctx, node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	userID := domain.NewID()
	return node, userID
}

func TestEnrollNodeHappyPath(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)

	// Configure mock SSH responses.
	env.sshSession.RunFunc = func(cmd string) (string, error) {
		return "edgefabric-ssh-ok Linux edge1 5.15\nactive", nil
	}

	// Trigger enrollment.
	job, err := env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("enroll node: %v", err)
	}
	if job.Action != domain.ProvisionActionEnroll {
		t.Errorf("expected action enroll, got %s", job.Action)
	}
	if job.Status != domain.ProvisionStatusPending {
		t.Errorf("expected initial status pending, got %s", job.Status)
	}

	// Wait for pipeline to complete.
	time.Sleep(500 * time.Millisecond)

	// Check job completed.
	updated, err := env.provisioner.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.Status != domain.ProvisionStatusCompleted {
		t.Errorf("expected completed status, got %s (error: %s)", updated.Status, updated.Error)
	}

	// Verify node is online.
	updatedNode, err := env.store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updatedNode.Status != domain.NodeStatusOnline {
		t.Errorf("expected node online, got %s", updatedNode.Status)
	}

	// Verify WireGuard IP was assigned.
	if updatedNode.WireGuardIP == "" {
		t.Error("expected WireGuard IP to be assigned")
	}

	// Verify WireGuard peer was created.
	peer, err := env.store.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, node.ID)
	if err != nil {
		t.Fatalf("get WG peer: %v", err)
	}
	if peer.PublicKey == "" {
		t.Error("expected WG public key to be set")
	}

	// Verify SSH commands were called.
	cmds := env.sshSession.GetCommands()
	if len(cmds) == 0 {
		t.Error("expected SSH commands to be recorded")
	}
}

func TestEnrollNodeSSHFailure(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)

	// Make SSH connection fail.
	env.sshClient.ConnectError = fmt.Errorf("connection refused")

	job, err := env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("enroll node: %v", err)
	}

	// Wait for pipeline.
	time.Sleep(500 * time.Millisecond)

	updated, err := env.provisioner.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.Status != domain.ProvisionStatusFailed {
		t.Errorf("expected failed status, got %s", updated.Status)
	}
	if updated.Error == "" {
		t.Error("expected error message on failed job")
	}

	// Verify node went to error state.
	updatedNode, err := env.store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updatedNode.Status != domain.NodeStatusError {
		t.Errorf("expected node error state, got %s", updatedNode.Status)
	}
}

func TestConcurrentJobRejection(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)

	// Configure slow SSH responses to keep the first job running.
	env.sshSession.RunFunc = func(cmd string) (string, error) {
		time.Sleep(2 * time.Second)
		return "edgefabric-ssh-ok\nactive", nil
	}

	// Start first enrollment.
	_, err := env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err != nil {
		t.Fatalf("first enroll: %v", err)
	}

	// Wait briefly for the first job to start running.
	time.Sleep(100 * time.Millisecond)

	// Try to start a second enrollment — should be rejected.
	_, err = env.provisioner.EnrollNode(ctx, node.ID, userID)
	if err == nil {
		t.Fatal("expected error for concurrent job, got nil")
	}
}

func TestInvalidStateTransition(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, userID := createTestNodeForProvisioning(t, env)

	// Node is pending — cannot be stopped.
	_, err := env.provisioner.StopNode(ctx, node.ID, userID)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}

	// Node is pending — cannot be started.
	_, err = env.provisioner.StartNode(ctx, node.ID, userID)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
}
