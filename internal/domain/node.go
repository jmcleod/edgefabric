package domain

import (
	"encoding/json"
	"time"
)

// NodeStatus represents the lifecycle state of a node.
type NodeStatus string

const (
	NodeStatusPending        NodeStatus = "pending"
	NodeStatusEnrolling      NodeStatus = "enrolling"
	NodeStatusOnline         NodeStatus = "online"
	NodeStatusOffline        NodeStatus = "offline"
	NodeStatusError          NodeStatus = "error"
	NodeStatusDecommissioned NodeStatus = "decommissioned"
)

// NodeCapability represents a service a node can run.
type NodeCapability string

const (
	CapabilityBGP   NodeCapability = "bgp"
	CapabilityDNS   NodeCapability = "dns"
	CapabilityCDN   NodeCapability = "cdn"
	CapabilityRoute NodeCapability = "route"
)

// Node is an edge server managed by EdgeFabric.
type Node struct {
	ID            ID               `json:"id" db:"id"`
	TenantID      *ID              `json:"tenant_id,omitempty" db:"tenant_id"`
	Name          string           `json:"name" db:"name"`
	Hostname      string           `json:"hostname" db:"hostname"`
	PublicIP      string           `json:"public_ip" db:"public_ip"`
	WireGuardIP   string           `json:"wireguard_ip" db:"wireguard_ip"`
	Status        NodeStatus       `json:"status" db:"status"`
	Region        string           `json:"region,omitempty" db:"region"`
	Provider      string           `json:"provider,omitempty" db:"provider"`
	SSHPort       int              `json:"ssh_port" db:"ssh_port"`
	SSHUser       string           `json:"ssh_user" db:"ssh_user"`
	SSHKeyID      *ID              `json:"ssh_key_id,omitempty" db:"ssh_key_id"`
	BinaryVersion string           `json:"binary_version,omitempty" db:"binary_version"`
	LastHeartbeat *time.Time       `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	Capabilities  []NodeCapability `json:"capabilities" db:"-"`
	Metadata      json.RawMessage  `json:"metadata,omitempty" db:"metadata"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
}

// NodeGroup is a logical grouping of nodes for targeting configuration.
type NodeGroup struct {
	ID          ID        `json:"id" db:"id"`
	TenantID    ID        `json:"tenant_id" db:"tenant_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
