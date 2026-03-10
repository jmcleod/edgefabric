// Package nodestate manages the local state file for a node agent.
// After enrollment, the node persists its identity (node ID, API token,
// WireGuard IP) so it can reconnect to the controller on restart.
package nodestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const stateFileName = "node-state.json"

// State holds the node's persisted identity.
type State struct {
	NodeID      string `json:"node_id"`
	APIToken    string `json:"api_token"`
	WireGuardIP string `json:"wireguard_ip"`
}

// Load reads the state file from dataDir. Returns nil if the file does not exist.
func Load(dataDir string) (*State, error) {
	path := filepath.Join(dataDir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &s, nil
}

// Save writes the state file to dataDir, creating the directory if needed.
func Save(dataDir string, s *State) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := filepath.Join(dataDir, stateFileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}
