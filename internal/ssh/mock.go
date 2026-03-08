package ssh

import (
	"fmt"
	"io"
	"sync"
)

// Ensure MockClient implements Client at compile time.
var _ Client = (*MockClient)(nil)

// MockClient is a test double for Client that records all calls
// and returns configurable responses.
type MockClient struct {
	mu sync.Mutex

	// ConnectFunc overrides the default Connect behavior when set.
	ConnectFunc func(target Target) (Session, error)

	// ConnectError causes Connect to return this error when set.
	ConnectError error

	// Targets records all Connect calls.
	Targets []Target

	// Session is the mock session returned by Connect (when ConnectFunc is nil).
	Session *MockSession
}

// NewMockClient creates a MockClient with a default MockSession.
func NewMockClient() *MockClient {
	return &MockClient{
		Session: NewMockSession(),
	}
}

// Connect records the target and returns the mock session or error.
func (c *MockClient) Connect(target Target) (Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Targets = append(c.Targets, target)

	if c.ConnectFunc != nil {
		return c.ConnectFunc(target)
	}
	if c.ConnectError != nil {
		return nil, c.ConnectError
	}
	return c.Session, nil
}

// Ensure MockSession implements Session at compile time.
var _ Session = (*MockSession)(nil)

// MockSession is a test double for Session that records all calls.
type MockSession struct {
	mu sync.Mutex

	// Commands records all Run calls.
	Commands []string

	// Uploads records all Upload calls.
	Uploads []MockUpload

	// RunFunc overrides the default Run behavior when set.
	// It receives the command and should return output and error.
	RunFunc func(cmd string) (string, error)

	// RunOutputs maps command strings to their outputs.
	// Used when RunFunc is not set.
	RunOutputs map[string]string

	// RunErrors maps command strings to their errors.
	// Used when RunFunc is not set.
	RunErrors map[string]error

	// UploadFunc overrides the default Upload behavior when set.
	UploadFunc func(r io.Reader, remotePath string, size int64, mode uint32) error

	// UploadError causes Upload to return this error when set.
	UploadError error

	// Closed tracks whether Close was called.
	Closed bool
}

// MockUpload records the parameters of an Upload call.
type MockUpload struct {
	RemotePath string
	Size       int64
	Mode       uint32
	Data       []byte // The uploaded data (read from the reader).
}

// NewMockSession creates a MockSession with empty defaults.
func NewMockSession() *MockSession {
	return &MockSession{
		RunOutputs: make(map[string]string),
		RunErrors:  make(map[string]error),
	}
}

// Run records the command and returns the configured response.
func (s *MockSession) Run(cmd string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Commands = append(s.Commands, cmd)

	if s.RunFunc != nil {
		return s.RunFunc(cmd)
	}

	if err, ok := s.RunErrors[cmd]; ok {
		output := s.RunOutputs[cmd]
		return output, err
	}
	if output, ok := s.RunOutputs[cmd]; ok {
		return output, nil
	}
	return "", nil
}

// Upload records the upload and returns the configured response.
func (s *MockSession) Upload(r io.Reader, remotePath string, size int64, mode uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("mock read upload data: %w", err)
	}

	s.Uploads = append(s.Uploads, MockUpload{
		RemotePath: remotePath,
		Size:       size,
		Mode:       mode,
		Data:       data,
	})

	if s.UploadFunc != nil {
		return s.UploadFunc(r, remotePath, size, mode)
	}
	if s.UploadError != nil {
		return s.UploadError
	}
	return nil
}

// Close marks the session as closed.
func (s *MockSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Closed = true
	return nil
}

// GetCommands returns a copy of the commands run on the session.
func (s *MockSession) GetCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cmds := make([]string, len(s.Commands))
	copy(cmds, s.Commands)
	return cmds
}

// GetUploads returns a copy of the uploads performed on the session.
func (s *MockSession) GetUploads() []MockUpload {
	s.mu.Lock()
	defer s.mu.Unlock()
	uploads := make([]MockUpload, len(s.Uploads))
	copy(uploads, s.Uploads)
	return uploads
}
