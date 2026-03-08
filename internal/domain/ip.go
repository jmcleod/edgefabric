package domain

import "time"

// IPAllocationType categorizes an IP allocation.
type IPAllocationType string

const (
	IPAllocationAnycast  IPAllocationType = "anycast"
	IPAllocationUnicast  IPAllocationType = "unicast"
)

// IPAllocationPurpose describes what the IP is used for.
type IPAllocationPurpose string

const (
	IPPurposeDNS   IPAllocationPurpose = "dns"
	IPPurposeCDN   IPAllocationPurpose = "cdn"
	IPPurposeRoute IPAllocationPurpose = "route"
)

// IPAllocationStatus represents the state of an IP allocation.
type IPAllocationStatus string

const (
	IPAllocationActive    IPAllocationStatus = "active"
	IPAllocationWithdrawn IPAllocationStatus = "withdrawn"
	IPAllocationPending   IPAllocationStatus = "pending"
)

// IPAllocation is an IP prefix assigned to a tenant.
type IPAllocation struct {
	ID        ID                 `json:"id" db:"id"`
	TenantID  ID                 `json:"tenant_id" db:"tenant_id"`
	Prefix    string             `json:"prefix" db:"prefix"`
	Type      IPAllocationType   `json:"type" db:"type"`
	Purpose   IPAllocationPurpose `json:"purpose" db:"purpose"`
	Status    IPAllocationStatus `json:"status" db:"status"`
	CreatedAt time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" db:"updated_at"`
}
