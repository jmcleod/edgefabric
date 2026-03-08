package domain

import (
	"encoding/json"
	"time"
)

// BGPSessionStatus represents the state of a BGP peering session.
type BGPSessionStatus string

const (
	BGPSessionConfigured  BGPSessionStatus = "configured"
	BGPSessionEstablished BGPSessionStatus = "established"
	BGPSessionIdle        BGPSessionStatus = "idle"
	BGPSessionError       BGPSessionStatus = "error"
)

// BGPSession represents a BGP peering session on a node.
type BGPSession struct {
	ID                ID               `json:"id" db:"id"`
	NodeID            ID               `json:"node_id" db:"node_id"`
	PeerASN           uint32           `json:"peer_asn" db:"peer_asn"`
	PeerAddress       string           `json:"peer_address" db:"peer_address"`
	LocalASN          uint32           `json:"local_asn" db:"local_asn"`
	Status            BGPSessionStatus `json:"status" db:"status"`
	AnnouncedPrefixes []string         `json:"announced_prefixes" db:"-"`
	ImportPolicy      string           `json:"import_policy,omitempty" db:"import_policy"`
	ExportPolicy      string           `json:"export_policy,omitempty" db:"export_policy"`
	Metadata          json.RawMessage  `json:"metadata,omitempty" db:"metadata"`
	CreatedAt         time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at" db:"updated_at"`
}
