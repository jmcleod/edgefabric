// Package ssh provides an abstraction for SSH operations used during node provisioning.
//
// The Client interface allows the provisioning service to connect to remote hosts,
// execute commands, and upload files via SCP — all without coupling to a specific
// SSH library. A real implementation (using golang.org/x/crypto/ssh) and a mock
// implementation (for testing) are provided.
package ssh

import "io"

// Target describes a remote SSH host.
type Target struct {
	Host       string // IP or hostname.
	Port       int    // SSH port (default 22).
	User       string // SSH user (typically "root").
	PrivateKey []byte // PEM-encoded private key (decrypted).
	HostKey    []byte // Known host key for verification (empty = TOFU).
}

// Client connects to remote hosts and creates sessions.
type Client interface {
	// Connect establishes an SSH connection to the target and returns a Session.
	Connect(target Target) (Session, error)
}

// Session represents an active SSH connection with command execution
// and file upload capabilities.
type Session interface {
	// Run executes a command on the remote host and returns combined stdout+stderr.
	Run(cmd string) (output string, err error)

	// Upload transfers a file to the remote host at the specified path.
	// The file contents are read from r, with the given size and permissions.
	Upload(r io.Reader, remotePath string, size int64, mode uint32) error

	// Close releases all resources associated with the session.
	Close() error
}
