// Package dns implements the controller-side DNS zone and record management
// service. It validates DNS data, manages zone serials, and provides
// configuration snapshots for node-side DNS servers to poll.
package dns

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Service defines the controller-side DNS management interface.
type Service interface {
	// Zone CRUD.
	CreateZone(ctx context.Context, req CreateZoneRequest) (*domain.DNSZone, error)
	GetZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error)
	ListZones(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.DNSZone, int, error)
	UpdateZone(ctx context.Context, id domain.ID, req UpdateZoneRequest) (*domain.DNSZone, error)
	DeleteZone(ctx context.Context, id domain.ID) error

	// Record CRUD.
	CreateRecord(ctx context.Context, req CreateRecordRequest) (*domain.DNSRecord, error)
	GetRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error)
	ListRecords(ctx context.Context, zoneID domain.ID, params storage.ListParams) ([]*domain.DNSRecord, int, error)
	UpdateRecord(ctx context.Context, id domain.ID, req UpdateRecordRequest) (*domain.DNSRecord, error)
	DeleteRecord(ctx context.Context, id domain.ID) error

	// Config sync — returns DNS config for a specific node.
	GetNodeDNSConfig(ctx context.Context, nodeID domain.ID) (*NodeDNSConfig, error)
}

// CreateZoneRequest is the input for creating a DNS zone.
type CreateZoneRequest struct {
	TenantID           domain.ID  `json:"tenant_id"`
	Name               string     `json:"name"`
	TTL                int        `json:"ttl,omitempty"`
	NodeGroupID        *domain.ID `json:"node_group_id,omitempty"`
	TransferAllowedIPs []string   `json:"transfer_allowed_ips,omitempty"`
}

// UpdateZoneRequest is the input for updating a DNS zone.
type UpdateZoneRequest struct {
	Name        *string               `json:"name,omitempty"`
	Status      *domain.DNSZoneStatus `json:"status,omitempty"`
	TTL         *int                  `json:"ttl,omitempty"`
	NodeGroupID *domain.ID            `json:"node_group_id,omitempty"`
	// ClearNodeGroup explicitly removes the node group assignment when true.
	ClearNodeGroup     bool      `json:"clear_node_group,omitempty"`
	TransferAllowedIPs *[]string `json:"transfer_allowed_ips,omitempty"`
}

// CreateRecordRequest is the input for creating a DNS record.
type CreateRecordRequest struct {
	ZoneID   domain.ID         `json:"zone_id"`
	Name     string            `json:"name"`
	Type     domain.DNSRecordType `json:"type"`
	Value    string            `json:"value"`
	TTL      *int              `json:"ttl,omitempty"`
	Priority *int              `json:"priority,omitempty"`
	Weight   *int              `json:"weight,omitempty"`
	Port     *int              `json:"port,omitempty"`
}

// UpdateRecordRequest is the input for updating a DNS record.
type UpdateRecordRequest struct {
	Name     *string              `json:"name,omitempty"`
	Type     *domain.DNSRecordType `json:"type,omitempty"`
	Value    *string              `json:"value,omitempty"`
	TTL      *int                 `json:"ttl,omitempty"`
	Priority *int                 `json:"priority,omitempty"`
	Weight   *int                 `json:"weight,omitempty"`
	Port     *int                 `json:"port,omitempty"`
}

// NodeDNSConfig is the full DNS configuration snapshot for a node.
// Nodes poll this to reconcile their local DNS server state.
type NodeDNSConfig struct {
	Zones []ZoneWithRecords `json:"zones"`
}

// ZoneWithRecords bundles a zone with all its records for sync.
type ZoneWithRecords struct {
	Zone    *domain.DNSZone    `json:"zone"`
	Records []*domain.DNSRecord `json:"records"`
}
