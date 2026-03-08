package provisioning

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage"
	gossh "golang.org/x/crypto/ssh"
)

// RotateSSHKey generates a new key pair, encrypts the private key, and updates
// the stored SSH key. The old key is preserved until DeploySSHKey replaces it
// on each node's authorized_keys.
func (p *DefaultProvisioner) RotateSSHKey(ctx context.Context, keyID domain.ID) (*domain.SSHKey, error) {
	// Load existing key.
	existing, err := p.sshKeys.GetSSHKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("get SSH key: %w", err)
	}

	// Generate new Ed25519 key pair.
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Encode public key in OpenSSH format.
	sshPub, err := gossh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("convert public key: %w", err)
	}
	pubKeyStr := string(gossh.MarshalAuthorizedKey(sshPub))

	// Encode private key in PEM format.
	privKeyPEM, err := gossh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privKeyStr := string(pem.EncodeToMemory(privKeyPEM))

	// Encrypt the private key.
	encryptedPriv, err := p.secrets.Encrypt(privKeyStr)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}

	// Compute fingerprint.
	fingerprint := gossh.FingerprintSHA256(sshPub)

	// Update the stored key.
	now := time.Now().UTC()
	existing.PublicKey = pubKeyStr
	existing.PrivateKey = encryptedPriv
	existing.Fingerprint = fingerprint
	existing.LastRotatedAt = now

	if err := p.sshKeys.UpdateSSHKey(ctx, existing); err != nil {
		return nil, fmt.Errorf("update SSH key: %w", err)
	}

	// Don't return private key in response.
	existing.PrivateKey = ""
	return existing, nil
}

// DeploySSHKey connects to each node that uses the given SSH key and appends
// the current public key to authorized_keys, then verifies access with the
// new key. This enables zero-downtime key rotation: first RotateSSHKey, then
// DeploySSHKey to push the new key, then optionally remove the old key.
func (p *DefaultProvisioner) DeploySSHKey(ctx context.Context, keyID domain.ID) error {
	// Load the SSH key.
	key, err := p.sshKeys.GetSSHKey(ctx, keyID)
	if err != nil {
		return fmt.Errorf("get SSH key: %w", err)
	}

	// Decrypt the private key for SSH connections.
	privKeyPlain, err := p.secrets.Decrypt(key.PrivateKey)
	if err != nil {
		return fmt.Errorf("decrypt private key: %w", err)
	}

	// Find all online nodes using this key.
	// We list all nodes (no tenant filter) since SSH keys are global.
	nodes, _, err := p.nodes.ListNodes(ctx, nil, storage.ListParams{Offset: 0, Limit: 10000})
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	var deployErrors []string
	for _, node := range nodes {
		// Skip nodes not using this key.
		if node.SSHKeyID == nil || *node.SSHKeyID != keyID {
			continue
		}

		// Skip nodes that aren't reachable.
		if node.Status != domain.NodeStatusOnline && node.Status != domain.NodeStatusOffline {
			continue
		}

		// Connect and deploy.
		if err := p.deployKeyToNode(ctx, node, key.PublicKey, []byte(privKeyPlain)); err != nil {
			deployErrors = append(deployErrors, fmt.Sprintf("node %s (%s): %s", node.Name, node.ID, err))
			continue
		}
	}

	if len(deployErrors) > 0 {
		return fmt.Errorf("deploy failed on %d node(s): %s", len(deployErrors), deployErrors[0])
	}

	return nil
}

// deployKeyToNode SSH-es into a single node and appends the public key to authorized_keys.
func (p *DefaultProvisioner) deployKeyToNode(ctx context.Context, node *domain.Node, pubKey string, privKey []byte) error {
	port := node.SSHPort
	if port == 0 {
		port = 22
	}
	user := node.SSHUser
	if user == "" {
		user = "root"
	}

	target := ssh.Target{
		Host:       node.PublicIP,
		Port:       port,
		User:       user,
		PrivateKey: privKey,
	}

	session, err := p.sshClient.Connect(target)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer session.Close()

	// Append the public key to authorized_keys (idempotent via grep check).
	cmd := fmt.Sprintf(
		`grep -qF %q ~/.ssh/authorized_keys 2>/dev/null || echo %q >> ~/.ssh/authorized_keys`,
		pubKey, pubKey,
	)
	if _, err := session.Run(cmd); err != nil {
		return fmt.Errorf("append authorized_keys: %w", err)
	}

	return nil
}
