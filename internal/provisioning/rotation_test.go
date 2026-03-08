package provisioning_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestRotateSSHKeyGeneratesNewKeyPair(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, _ = createTestNodeForProvisioning(t, env)

	// Get existing keys.
	keys, _, err := env.store.ListSSHKeys(ctx, storage.ListParams{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected at least one SSH key")
	}

	original := keys[0]
	originalPub := original.PublicKey
	originalFP := original.Fingerprint

	// Rotate.
	rotated, err := env.provisioner.RotateSSHKey(ctx, original.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Verify new key is different.
	if rotated.PublicKey == originalPub {
		t.Error("expected new public key after rotation")
	}
	if rotated.Fingerprint == originalFP {
		t.Error("expected new fingerprint after rotation")
	}
	if rotated.PrivateKey != "" {
		t.Error("expected private key to be stripped from response")
	}
	if rotated.LastRotatedAt.IsZero() {
		t.Error("expected last_rotated_at to be set")
	}

	// Verify stored key was updated.
	stored, err := env.store.GetSSHKey(ctx, original.ID)
	if err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if stored.PublicKey == originalPub {
		t.Error("expected stored public key to be updated")
	}
	if stored.PrivateKey == "" {
		t.Error("expected stored private key to be non-empty (encrypted)")
	}

	// Verify we can decrypt the new private key.
	privPlain, err := env.secrets.Decrypt(stored.PrivateKey)
	if err != nil {
		t.Fatalf("decrypt new private key: %v", err)
	}
	if privPlain == "" {
		t.Error("expected non-empty plaintext private key")
	}
}

func TestDeploySSHKeyToNodes(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, _ := createTestNodeForProvisioning(t, env)

	// Make node online so deploy will target it.
	node.Status = domain.NodeStatusOnline
	if err := env.store.UpdateNode(ctx, node); err != nil {
		t.Fatalf("update node: %v", err)
	}

	// Get the SSH key.
	keys, _, _ := env.store.ListSSHKeys(ctx, storage.ListParams{Offset: 0, Limit: 10})
	if len(keys) == 0 {
		t.Fatal("expected at least one SSH key")
	}
	keyID := keys[0].ID

	// First rotate to get a properly encrypted Ed25519 key.
	_, err := env.provisioner.RotateSSHKey(ctx, keyID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// Configure mock SSH.
	env.sshSession.RunFunc = func(cmd string) (string, error) {
		return "", nil
	}

	// Deploy.
	err = env.provisioner.DeploySSHKey(ctx, keyID)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}

	// Verify SSH was called.
	cmds := env.sshSession.GetCommands()
	if len(cmds) == 0 {
		t.Error("expected at least one SSH command for deploying key")
	}
}

func TestRotateSSHKeyNotFound(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.provisioner.RotateSSHKey(ctx, domain.NewID())
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}
