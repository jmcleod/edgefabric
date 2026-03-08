package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/ssh"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// executeStep dispatches a provisioning step to its implementation.
func (p *DefaultProvisioner) executeStep(ctx context.Context, step domain.ProvisioningStep, job *domain.ProvisioningJob, node *domain.Node) (string, error) {
	switch step {
	case domain.StepValidateSSH:
		return p.stepValidateSSH(ctx, node)
	case domain.StepUploadBinary:
		return p.stepUploadBinary(ctx, node)
	case domain.StepWriteConfig:
		return p.stepWriteConfig(ctx, job, node)
	case domain.StepInstallSystemd:
		return p.stepInstallSystemd(ctx, node)
	case domain.StepStartService:
		return p.stepStartService(ctx, node)
	case domain.StepGenerateWGKeys:
		return p.stepGenerateWGKeys(ctx, node)
	case domain.StepConfigureWireGuard:
		return p.stepConfigureWireGuard(ctx, node)
	case domain.StepWaitEnrollment:
		return p.stepWaitEnrollment(ctx, node)
	case domain.StepVerifyOnline:
		return p.stepVerifyOnline(ctx, node)
	case domain.StepStopService:
		return p.stepStopService(ctx, node)
	case domain.StepSendCommand:
		return p.stepSendCommand(ctx, job, node)
	case domain.StepCleanup:
		return p.stepCleanup(ctx, node)
	default:
		return "", fmt.Errorf("unknown step: %s", step)
	}
}

// connectToNode establishes an SSH session to the node.
func (p *DefaultProvisioner) connectToNode(ctx context.Context, node *domain.Node) (ssh.Session, error) {
	// Load SSH key.
	if node.SSHKeyID == nil {
		return nil, fmt.Errorf("node has no SSH key assigned")
	}

	key, err := p.sshKeys.GetSSHKey(ctx, *node.SSHKeyID)
	if err != nil {
		return nil, fmt.Errorf("get SSH key: %w", err)
	}

	// Decrypt private key.
	privateKeyPEM, err := p.secrets.Decrypt(key.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt SSH private key: %w", err)
	}

	target := ssh.Target{
		Host:       node.PublicIP,
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: []byte(privateKeyPEM),
	}

	session, err := p.sshClient.Connect(target)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	return session, nil
}

func (p *DefaultProvisioner) stepValidateSSH(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.Run("echo edgefabric-ssh-ok && uname -a")
	if err != nil {
		return output, fmt.Errorf("SSH validation failed: %w", err)
	}
	if !strings.Contains(output, "edgefabric-ssh-ok") {
		return output, fmt.Errorf("SSH validation: unexpected output")
	}
	return output, nil
}

func (p *DefaultProvisioner) stepUploadBinary(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	// Upload the edgefabric binary.
	// In production, this would read from an embedded binary or artifact store.
	// For now, we upload a placeholder and record the intent.
	output, err := session.Run("mkdir -p /usr/local/bin && echo binary-upload-ready")
	if err != nil {
		return output, fmt.Errorf("prepare binary upload: %w", err)
	}
	return "binary upload prepared (placeholder)", nil
}

func (p *DefaultProvisioner) stepWriteConfig(ctx context.Context, job *domain.ProvisioningJob, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	// Generate enrollment token for the node.
	tenantID := domain.ID{}
	if node.TenantID != nil {
		tenantID = *node.TenantID
	}
	token, err := p.GenerateEnrollmentToken(ctx, tenantID, node.ID)
	if err != nil {
		return "", fmt.Errorf("generate enrollment token: %w", err)
	}

	cfg := GenerateNodeConfig(p.extURL, token.Token, "/var/lib/edgefabric")

	// Write config to the node.
	cmd := fmt.Sprintf("mkdir -p /etc/edgefabric && cat > /etc/edgefabric/config.yaml << 'EFCONFIG'\n%s\nEFCONFIG", cfg)
	output, err := session.Run(cmd)
	if err != nil {
		return output, fmt.Errorf("write config: %w", err)
	}
	return "config written to /etc/edgefabric/config.yaml", nil
}

func (p *DefaultProvisioner) stepInstallSystemd(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	unit := GenerateSystemdUnit()
	cmd := fmt.Sprintf("cat > /etc/systemd/system/edgefabric.service << 'EFUNIT'\n%s\nEFUNIT\nsystemctl daemon-reload && systemctl enable edgefabric", unit)
	output, err := session.Run(cmd)
	if err != nil {
		return output, fmt.Errorf("install systemd unit: %w", err)
	}
	return "systemd unit installed and enabled", nil
}

func (p *DefaultProvisioner) stepStartService(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.Run("systemctl start edgefabric")
	if err != nil {
		return output, fmt.Errorf("start service: %w", err)
	}
	return "edgefabric service started", nil
}

func (p *DefaultProvisioner) stepStopService(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.Run("systemctl stop edgefabric")
	if err != nil {
		return output, fmt.Errorf("stop service: %w", err)
	}
	return "edgefabric service stopped", nil
}

func (p *DefaultProvisioner) stepGenerateWGKeys(ctx context.Context, node *domain.Node) (string, error) {
	// Generate key pair.
	kp, err := GenerateWireGuardKeyPair()
	if err != nil {
		return "", fmt.Errorf("generate WG key pair: %w", err)
	}

	psk, err := GeneratePresharedKey()
	if err != nil {
		return "", fmt.Errorf("generate preshared key: %w", err)
	}

	// Encrypt private key for storage.
	encPriv, err := p.secrets.Encrypt(kp.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("encrypt WG private key: %w", err)
	}

	encPSK, err := p.secrets.Encrypt(psk)
	if err != nil {
		return "", fmt.Errorf("encrypt preshared key: %w", err)
	}

	// Allocate overlay IP.
	peers, _, err := p.peers.ListWireGuardPeers(ctx, storage.ListParams{Limit: 10000})
	if err != nil {
		return "", fmt.Errorf("list WG peers: %w", err)
	}
	nodes, _, err := p.nodes.ListNodes(ctx, nil, storage.ListParams{Limit: 10000})
	if err != nil {
		return "", fmt.Errorf("list nodes: %w", err)
	}

	overlayIP, err := AllocateOverlayIP(p.wgConfig.Subnet, p.wgConfig.Address, peers, nodes)
	if err != nil {
		return "", fmt.Errorf("allocate overlay IP: %w", err)
	}

	// Create WireGuard peer record.
	peer := &domain.WireGuardPeer{
		ID:           domain.NewID(),
		OwnerType:    domain.PeerOwnerNode,
		OwnerID:      node.ID,
		PublicKey:     kp.PublicKey,
		PrivateKey:    encPriv,
		PresharedKey:  encPSK,
		AllowedIPs:    []string{overlayIP + "/32"},
		Endpoint:      fmt.Sprintf("%s:%d", node.PublicIP, p.wgConfig.ListenPort),
	}
	if err := p.peers.CreateWireGuardPeer(ctx, peer); err != nil {
		return "", fmt.Errorf("create WG peer: %w", err)
	}

	// Update node with overlay IP.
	node.WireGuardIP = overlayIP
	if err := p.nodes.UpdateNode(ctx, node); err != nil {
		return "", fmt.Errorf("update node overlay IP: %w", err)
	}

	return fmt.Sprintf("WG keys generated, overlay IP %s assigned", overlayIP), nil
}

func (p *DefaultProvisioner) stepConfigureWireGuard(ctx context.Context, node *domain.Node) (string, error) {
	if p.wgConfigGen == nil {
		return "wireguard config sync skipped (config generator not configured)", nil
	}

	// Generate wg0.conf for the node.
	wgConf, err := p.wgConfigGen.GenerateNodeConfig(ctx, node.ID)
	if err != nil {
		return "", fmt.Errorf("generate WG config: %w", err)
	}

	// Push config to node via SSH.
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	// Write wg0.conf and bring up the interface.
	cmd := fmt.Sprintf("mkdir -p /etc/wireguard && cat > /etc/wireguard/wg0.conf << 'WGCONF'\n%s\nWGCONF", wgConf)
	output, err := session.Run(cmd)
	if err != nil {
		return output, fmt.Errorf("write wg0.conf: %w", err)
	}

	// Bring up WireGuard interface (idempotent: down first if already up).
	output, err = session.Run("wg-quick down wg0 2>/dev/null; wg-quick up wg0")
	if err != nil {
		return output, fmt.Errorf("wg-quick up: %w", err)
	}

	return "wireguard config written and interface started", nil
}

func (p *DefaultProvisioner) stepWaitEnrollment(_ context.Context, _ *domain.Node) (string, error) {
	// In a full implementation, this would poll for the node agent to complete
	// enrollment by calling back to the controller. For v1, we mark it as
	// successful since the binary was started and WG keys were generated.
	return "enrollment acknowledged (v1: auto-complete)", nil
}

func (p *DefaultProvisioner) stepVerifyOnline(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.Run("systemctl is-active edgefabric")
	if err != nil {
		return output, fmt.Errorf("service not active: %w", err)
	}
	if !strings.Contains(output, "active") {
		return output, fmt.Errorf("service status: %s", output)
	}
	return "edgefabric service verified active", nil
}

func (p *DefaultProvisioner) stepSendCommand(ctx context.Context, job *domain.ProvisioningJob, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	// Determine command based on action.
	var cmd string
	switch job.Action {
	case domain.ProvisionActionStart:
		cmd = "systemctl start edgefabric"
	case domain.ProvisionActionStop, domain.ProvisionActionDecommission:
		cmd = "systemctl stop edgefabric"
	case domain.ProvisionActionRestart, domain.ProvisionActionUpgrade:
		cmd = "systemctl restart edgefabric"
	default:
		cmd = "systemctl status edgefabric"
	}

	output, err := session.Run(cmd)
	if err != nil {
		return output, fmt.Errorf("send command %q: %w", cmd, err)
	}
	return fmt.Sprintf("command %q executed", cmd), nil
}

func (p *DefaultProvisioner) stepCleanup(ctx context.Context, node *domain.Node) (string, error) {
	session, err := p.connectToNode(ctx, node)
	if err != nil {
		return "", err
	}
	defer session.Close()

	// Disable service, remove binary and config.
	cmds := []string{
		"systemctl disable edgefabric 2>/dev/null || true",
		"rm -f /etc/systemd/system/edgefabric.service",
		"systemctl daemon-reload",
		"rm -f /usr/local/bin/edgefabric",
		"rm -rf /etc/edgefabric",
		"rm -rf /var/lib/edgefabric",
	}
	output, err := session.Run(strings.Join(cmds, " && "))
	if err != nil {
		return output, fmt.Errorf("cleanup: %w", err)
	}

	// Delete WireGuard peer.
	peer, err := p.peers.GetWireGuardPeerByOwner(ctx, domain.PeerOwnerNode, node.ID)
	if err == nil {
		_ = p.peers.DeleteWireGuardPeer(ctx, peer.ID)
	}

	return "node cleaned up and decommissioned", nil
}
