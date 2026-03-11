package dns

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService is the controller-side DNS management service.
type DefaultService struct {
	zones      storage.DNSZoneStore
	records    storage.DNSRecordStore
	nodeGroups storage.NodeGroupStore
	nodes      storage.NodeStore
}

// NewService creates a new DNS service.
func NewService(
	zones storage.DNSZoneStore,
	records storage.DNSRecordStore,
	nodeGroups storage.NodeGroupStore,
	nodes storage.NodeStore,
) *DefaultService {
	return &DefaultService{
		zones:      zones,
		records:    records,
		nodeGroups: nodeGroups,
		nodes:      nodes,
	}
}

// --- Zone CRUD ---

func (s *DefaultService) CreateZone(ctx context.Context, req CreateZoneRequest) (*domain.DNSZone, error) {
	if err := validateZoneName(req.Name); err != nil {
		return nil, fmt.Errorf("invalid zone: %w", err)
	}

	if err := validateTransferAllowedIPs(req.TransferAllowedIPs); err != nil {
		return nil, fmt.Errorf("invalid zone: %w", err)
	}

	zone := &domain.DNSZone{
		ID:                 domain.NewID(),
		TenantID:           req.TenantID,
		Name:               req.Name,
		TTL:                req.TTL,
		NodeGroupID:        req.NodeGroupID,
		TransferAllowedIPs: req.TransferAllowedIPs,
	}

	if err := s.zones.CreateDNSZone(ctx, zone); err != nil {
		return nil, fmt.Errorf("create zone: %w", err)
	}
	return zone, nil
}

func (s *DefaultService) GetZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error) {
	return s.zones.GetDNSZone(ctx, id)
}

func (s *DefaultService) ListZones(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.DNSZone, int, error) {
	return s.zones.ListDNSZones(ctx, tenantID, params)
}

func (s *DefaultService) UpdateZone(ctx context.Context, id domain.ID, req UpdateZoneRequest) (*domain.DNSZone, error) {
	zone, err := s.zones.GetDNSZone(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if err := validateZoneName(*req.Name); err != nil {
			return nil, fmt.Errorf("invalid zone name: %w", err)
		}
		zone.Name = *req.Name
	}
	if req.Status != nil {
		zone.Status = *req.Status
	}
	if req.TTL != nil {
		zone.TTL = *req.TTL
	}
	if req.ClearNodeGroup {
		zone.NodeGroupID = nil
	} else if req.NodeGroupID != nil {
		zone.NodeGroupID = req.NodeGroupID
	}
	if req.TransferAllowedIPs != nil {
		if err := validateTransferAllowedIPs(*req.TransferAllowedIPs); err != nil {
			return nil, fmt.Errorf("invalid transfer allowed IPs: %w", err)
		}
		zone.TransferAllowedIPs = *req.TransferAllowedIPs
	}

	if err := s.zones.UpdateDNSZone(ctx, zone); err != nil {
		return nil, fmt.Errorf("update zone: %w", err)
	}
	return zone, nil
}

func (s *DefaultService) DeleteZone(ctx context.Context, id domain.ID) error {
	return s.zones.DeleteDNSZone(ctx, id)
}

// --- Record CRUD ---

func (s *DefaultService) CreateRecord(ctx context.Context, req CreateRecordRequest) (*domain.DNSRecord, error) {
	// Verify zone exists.
	_, err := s.zones.GetDNSZone(ctx, req.ZoneID)
	if err != nil {
		return nil, fmt.Errorf("zone lookup: %w", err)
	}

	record := &domain.DNSRecord{
		ID:       domain.NewID(),
		ZoneID:   req.ZoneID,
		Name:     req.Name,
		Type:     req.Type,
		Value:    req.Value,
		TTL:      req.TTL,
		Priority: req.Priority,
		Weight:   req.Weight,
		Port:     req.Port,
	}

	// Validate record type-specific rules.
	if err := validateRecord(record); err != nil {
		return nil, fmt.Errorf("invalid record: %w", err)
	}

	// Check CNAME exclusivity.
	existing, _, err := s.records.ListDNSRecords(ctx, req.ZoneID, storage.ListParams{Limit: 10000})
	if err != nil {
		return nil, fmt.Errorf("list records for cname check: %w", err)
	}
	if err := checkCNAMEExclusivity(existing, record); err != nil {
		return nil, fmt.Errorf("cname exclusivity: %w", err)
	}

	if err := s.records.CreateDNSRecord(ctx, record); err != nil {
		return nil, fmt.Errorf("create record: %w", err)
	}

	// Increment zone serial on record mutation.
	if err := s.zones.IncrementDNSZoneSerial(ctx, req.ZoneID); err != nil {
		return nil, fmt.Errorf("increment serial: %w", err)
	}

	return record, nil
}

func (s *DefaultService) GetRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error) {
	return s.records.GetDNSRecord(ctx, id)
}

func (s *DefaultService) ListRecords(ctx context.Context, zoneID domain.ID, params storage.ListParams) ([]*domain.DNSRecord, int, error) {
	return s.records.ListDNSRecords(ctx, zoneID, params)
}

func (s *DefaultService) UpdateRecord(ctx context.Context, id domain.ID, req UpdateRecordRequest) (*domain.DNSRecord, error) {
	record, err := s.records.GetDNSRecord(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		record.Name = *req.Name
	}
	if req.Type != nil {
		record.Type = *req.Type
	}
	if req.Value != nil {
		record.Value = *req.Value
	}
	if req.TTL != nil {
		record.TTL = req.TTL
	}
	if req.Priority != nil {
		record.Priority = req.Priority
	}
	if req.Weight != nil {
		record.Weight = req.Weight
	}
	if req.Port != nil {
		record.Port = req.Port
	}

	// Validate updated record.
	if err := validateRecord(record); err != nil {
		return nil, fmt.Errorf("invalid record: %w", err)
	}

	// Check CNAME exclusivity.
	existing, _, err := s.records.ListDNSRecords(ctx, record.ZoneID, storage.ListParams{Limit: 10000})
	if err != nil {
		return nil, fmt.Errorf("list records for cname check: %w", err)
	}
	if err := checkCNAMEExclusivity(existing, record); err != nil {
		return nil, fmt.Errorf("cname exclusivity: %w", err)
	}

	if err := s.records.UpdateDNSRecord(ctx, record); err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}

	// Increment zone serial on record mutation.
	if err := s.zones.IncrementDNSZoneSerial(ctx, record.ZoneID); err != nil {
		return nil, fmt.Errorf("increment serial: %w", err)
	}

	return record, nil
}

func (s *DefaultService) DeleteRecord(ctx context.Context, id domain.ID) error {
	// Get record to find zone ID for serial increment.
	record, err := s.records.GetDNSRecord(ctx, id)
	if err != nil {
		return err
	}

	if err := s.records.DeleteDNSRecord(ctx, id); err != nil {
		return fmt.Errorf("delete record: %w", err)
	}

	// Increment zone serial on record mutation.
	if err := s.zones.IncrementDNSZoneSerial(ctx, record.ZoneID); err != nil {
		return fmt.Errorf("increment serial: %w", err)
	}

	return nil
}

// --- Config sync ---

// GetNodeDNSConfig returns the complete DNS configuration for a node.
// It finds all zones assigned to node groups that the node belongs to,
// and includes all records for each zone.
func (s *DefaultService) GetNodeDNSConfig(ctx context.Context, nodeID domain.ID) (*NodeDNSConfig, error) {
	// Get the node to verify it exists.
	node, err := s.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	// Get all node groups this node belongs to.
	groups, err := s.nodeGroups.ListNodeGroups_ByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list node groups: %w", err)
	}

	// For each group, find zones assigned to it.
	// We need to query zones by tenant, then filter by node_group_id.
	config := &NodeDNSConfig{}
	seenZones := make(map[domain.ID]bool)

	for _, group := range groups {
		// List all zones for this tenant (the group's tenant).
		zones, _, err := s.zones.ListDNSZones(ctx, group.TenantID, storage.ListParams{Limit: 10000})
		if err != nil {
			return nil, fmt.Errorf("list zones for group %s: %w", group.ID, err)
		}

		for _, zone := range zones {
			// Only include zones assigned to this group and active.
			if zone.NodeGroupID == nil || *zone.NodeGroupID != group.ID {
				continue
			}
			if zone.Status != domain.DNSZoneActive {
				continue
			}
			if seenZones[zone.ID] {
				continue
			}
			seenZones[zone.ID] = true

			// Load records for this zone.
			records, _, err := s.records.ListDNSRecords(ctx, zone.ID, storage.ListParams{Limit: 10000})
			if err != nil {
				return nil, fmt.Errorf("list records for zone %s: %w", zone.Name, err)
			}

			config.Zones = append(config.Zones, ZoneWithRecords{
				Zone:    zone,
				Records: records,
			})
		}
	}

	// Also include zones assigned directly to the node's tenant if the node has a tenant.
	// (zones without a node group are not synced — they must be explicitly assigned)
	_ = node // Used above for existence check.

	return config, nil
}
