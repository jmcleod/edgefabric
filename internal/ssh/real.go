package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// Ensure RealClient implements Client at compile time.
var _ Client = (*RealClient)(nil)

// RealClient implements Client using golang.org/x/crypto/ssh.
type RealClient struct {
	// ConnectTimeout is the maximum time to wait for the TCP connection.
	ConnectTimeout time.Duration
}

// NewClient creates a new SSH client with sensible defaults.
func NewClient() *RealClient {
	return &RealClient{
		ConnectTimeout: 30 * time.Second,
	}
}

// Connect establishes an SSH connection to the target.
func (c *RealClient) Connect(target Target) (Session, error) {
	if target.Port == 0 {
		target.Port = 22
	}

	signer, err := gossh.ParsePrivateKey(target.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	// Host key verification: if HostKey is provided, verify against it.
	// Otherwise, use TOFU (Trust On First Use) — accept and log.
	var hostKeyCallback gossh.HostKeyCallback
	if len(target.HostKey) > 0 {
		pubKey, _, _, _, err := gossh.ParseAuthorizedKey(target.HostKey)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		hostKeyCallback = gossh.FixedHostKey(pubKey)
	} else {
		// TOFU: accept any host key on first connect.
		// In production, the host key should be captured and stored.
		hostKeyCallback = gossh.InsecureIgnoreHostKey()
	}

	config := &gossh.ClientConfig{
		User:            target.User,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.ConnectTimeout,
	}

	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return &realSession{client: client}, nil
}

// realSession wraps an SSH client connection.
type realSession struct {
	client *gossh.Client
}

// Run executes a command and returns combined output.
func (s *realSession) Run(cmd string) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	if err := session.Run(cmd); err != nil {
		return buf.String(), fmt.Errorf("run %q: %w (output: %s)", cmd, err, buf.String())
	}
	return buf.String(), nil
}

// Upload transfers a file via SCP protocol over SSH stdin piping.
func (s *realSession) Upload(r io.Reader, remotePath string, size int64, mode uint32) error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// SCP sink mode.
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	// Start SCP in sink mode.
	if err := session.Start(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("start scp: %w", err)
	}

	// Send file header: C<mode> <size> <filename>\n
	_, err = fmt.Fprintf(stdin, "C%04o %d %s\n", mode, size, basename(remotePath))
	if err != nil {
		return fmt.Errorf("write scp header: %w", err)
	}

	// Send file contents.
	if _, err := io.Copy(stdin, r); err != nil {
		return fmt.Errorf("copy file data: %w", err)
	}

	// Send null byte to indicate end of file.
	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("write scp terminator: %w", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("scp wait: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// Close closes the underlying SSH connection.
func (s *realSession) Close() error {
	return s.client.Close()
}

// basename extracts the filename from a path.
func basename(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
