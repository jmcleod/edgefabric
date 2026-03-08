package ssh_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/ssh"
)

func TestMockClientConnect(t *testing.T) {
	client := ssh.NewMockClient()

	target := ssh.Target{
		Host: "192.168.1.1",
		Port: 22,
		User: "root",
	}

	session, err := client.Connect(target)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	if len(client.Targets) != 1 {
		t.Errorf("expected 1 target recorded, got %d", len(client.Targets))
	}
	if client.Targets[0].Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", client.Targets[0].Host)
	}
}

func TestMockClientConnectError(t *testing.T) {
	client := ssh.NewMockClient()
	client.ConnectError = fmt.Errorf("connection refused")

	_, err := client.Connect(ssh.Target{Host: "10.0.0.1"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("expected 'connection refused', got %v", err)
	}
}

func TestMockSessionRun(t *testing.T) {
	session := ssh.NewMockSession()

	// Default: empty output, no error.
	out, err := session.Run("echo hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}

	// With configured output.
	session.RunOutputs["uname -a"] = "Linux edge-node 5.15"
	out, err = session.Run("uname -a")
	if err != nil {
		t.Fatalf("run uname: %v", err)
	}
	if out != "Linux edge-node 5.15" {
		t.Errorf("expected uname output, got %q", out)
	}

	// With configured error.
	session.RunErrors["bad-command"] = fmt.Errorf("command not found")
	_, err = session.Run("bad-command")
	if err == nil {
		t.Fatal("expected error for bad-command")
	}

	// Verify all commands recorded.
	cmds := session.GetCommands()
	if len(cmds) != 3 {
		t.Errorf("expected 3 commands recorded, got %d", len(cmds))
	}
}

func TestMockSessionRunFunc(t *testing.T) {
	session := ssh.NewMockSession()

	session.RunFunc = func(cmd string) (string, error) {
		if cmd == "systemctl status edgefabric" {
			return "active (running)", nil
		}
		return "", fmt.Errorf("unknown command: %s", cmd)
	}

	out, err := session.Run("systemctl status edgefabric")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "active (running)" {
		t.Errorf("expected 'active (running)', got %q", out)
	}

	_, err = session.Run("other")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestMockSessionUpload(t *testing.T) {
	session := ssh.NewMockSession()

	data := "#!/bin/bash\necho hello"
	r := strings.NewReader(data)
	err := session.Upload(r, "/usr/local/bin/edgefabric", int64(len(data)), 0o755)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	uploads := session.GetUploads()
	if len(uploads) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(uploads))
	}
	if uploads[0].RemotePath != "/usr/local/bin/edgefabric" {
		t.Errorf("expected path /usr/local/bin/edgefabric, got %s", uploads[0].RemotePath)
	}
	if string(uploads[0].Data) != data {
		t.Errorf("expected upload data %q, got %q", data, string(uploads[0].Data))
	}
	if uploads[0].Mode != 0o755 {
		t.Errorf("expected mode 0755, got %o", uploads[0].Mode)
	}
}

func TestMockSessionClose(t *testing.T) {
	session := ssh.NewMockSession()

	if session.Closed {
		t.Error("expected not closed initially")
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !session.Closed {
		t.Error("expected closed after Close()")
	}
}
