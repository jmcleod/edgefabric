package domain

import "time"

// DNSRecordType enumerates supported DNS record types.
type DNSRecordType string

const (
	DNSRecordTypeA     DNSRecordType = "A"
	DNSRecordTypeAAAA  DNSRecordType = "AAAA"
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
	DNSRecordTypeMX    DNSRecordType = "MX"
	DNSRecordTypeTXT   DNSRecordType = "TXT"
	DNSRecordTypeNS    DNSRecordType = "NS"
	DNSRecordTypeSRV   DNSRecordType = "SRV"
	DNSRecordTypeCAA   DNSRecordType = "CAA"
	DNSRecordTypePTR   DNSRecordType = "PTR"
)

// DNSZoneStatus represents whether a zone is serving.
type DNSZoneStatus string

const (
	DNSZoneActive   DNSZoneStatus = "active"
	DNSZoneDisabled DNSZoneStatus = "disabled"
)

// DNSZone is an authoritative DNS zone managed by a tenant.
type DNSZone struct {
	ID                 ID            `json:"id" db:"id"`
	TenantID           ID            `json:"tenant_id" db:"tenant_id"`
	Name               string        `json:"name" db:"name"`
	Status             DNSZoneStatus `json:"status" db:"status"`
	Serial             uint32        `json:"serial" db:"serial"`
	TTL                int           `json:"ttl" db:"ttl"`
	NodeGroupID        *ID           `json:"node_group_id,omitempty" db:"node_group_id"`
	TransferAllowedIPs []string      `json:"transfer_allowed_ips,omitempty"`
	CreatedAt          time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at" db:"updated_at"`
}

// DNSRecord is a single DNS record within a zone.
type DNSRecord struct {
	ID        ID            `json:"id" db:"id"`
	ZoneID    ID            `json:"zone_id" db:"zone_id"`
	Name      string        `json:"name" db:"name"`
	Type      DNSRecordType `json:"type" db:"type"`
	Value     string        `json:"value" db:"value"`
	TTL       *int          `json:"ttl,omitempty" db:"ttl"`
	Priority  *int          `json:"priority,omitempty" db:"priority"`
	Weight    *int          `json:"weight,omitempty" db:"weight"`
	Port      *int          `json:"port,omitempty" db:"port"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
}
