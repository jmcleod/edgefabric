// Package storage defines the persistence abstraction for EdgeFabric.
//
// All data access goes through the Store interface. Implementations exist
// for SQLite (default, single instance) and PostgreSQL (future HA).
package storage

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// Store is the top-level persistence interface. Each domain area has its own
// sub-interface to keep things organized and allow independent testing.
type Store interface {
	TenantStore
	UserStore
	APIKeyStore
	NodeStore
	NodeGroupStore
	GatewayStore
	WireGuardPeerStore
	IPAllocationStore
	BGPSessionStore
	DNSZoneStore
	DNSRecordStore
	CDNSiteStore
	CDNOriginStore
	RouteStore
	SSHKeyStore
	TLSCertificateStore
	AuditEventStore
	EnrollmentTokenStore
	ProvisioningJobStore

	// Migrate runs database migrations to ensure the schema is current.
	Migrate(ctx context.Context) error

	// Close releases database resources.
	Close() error

	// Ping checks database connectivity.
	Ping(ctx context.Context) error
}

// ListParams provides common pagination parameters.
type ListParams struct {
	Offset int
	Limit  int
}

// DefaultLimit is the default page size.
const DefaultLimit = 50

// TenantStore manages tenant persistence.
type TenantStore interface {
	CreateTenant(ctx context.Context, t *domain.Tenant) error
	GetTenant(ctx context.Context, id domain.ID) (*domain.Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	ListTenants(ctx context.Context, params ListParams) ([]*domain.Tenant, int, error)
	UpdateTenant(ctx context.Context, t *domain.Tenant) error
	DeleteTenant(ctx context.Context, id domain.ID) error
}

// UserStore manages user persistence.
type UserStore interface {
	CreateUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, id domain.ID) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	ListUsers(ctx context.Context, tenantID *domain.ID, params ListParams) ([]*domain.User, int, error)
	UpdateUser(ctx context.Context, u *domain.User) error
	DeleteUser(ctx context.Context, id domain.ID) error
}

// APIKeyStore manages API key persistence.
type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, k *domain.APIKey) error
	GetAPIKey(ctx context.Context, id domain.ID) (*domain.APIKey, error)
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error)
	ListAPIKeys(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.APIKey, int, error)
	DeleteAPIKey(ctx context.Context, id domain.ID) error
	UpdateAPIKeyLastUsed(ctx context.Context, id domain.ID) error
}

// NodeStore manages node persistence.
type NodeStore interface {
	CreateNode(ctx context.Context, n *domain.Node) error
	GetNode(ctx context.Context, id domain.ID) (*domain.Node, error)
	ListNodes(ctx context.Context, tenantID *domain.ID, params ListParams) ([]*domain.Node, int, error)
	UpdateNode(ctx context.Context, n *domain.Node) error
	DeleteNode(ctx context.Context, id domain.ID) error
	UpdateNodeHeartbeat(ctx context.Context, id domain.ID) error
}

// NodeGroupStore manages node group persistence.
type NodeGroupStore interface {
	CreateNodeGroup(ctx context.Context, g *domain.NodeGroup) error
	GetNodeGroup(ctx context.Context, id domain.ID) (*domain.NodeGroup, error)
	ListNodeGroups(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.NodeGroup, int, error)
	UpdateNodeGroup(ctx context.Context, g *domain.NodeGroup) error
	DeleteNodeGroup(ctx context.Context, id domain.ID) error
	AddNodeToGroup(ctx context.Context, groupID, nodeID domain.ID) error
	RemoveNodeFromGroup(ctx context.Context, groupID, nodeID domain.ID) error
	ListGroupNodes(ctx context.Context, groupID domain.ID) ([]*domain.Node, error)
	ListNodeGroups_ByNode(ctx context.Context, nodeID domain.ID) ([]*domain.NodeGroup, error)
}

// GatewayStore manages gateway persistence.
type GatewayStore interface {
	CreateGateway(ctx context.Context, g *domain.Gateway) error
	GetGateway(ctx context.Context, id domain.ID) (*domain.Gateway, error)
	ListGateways(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.Gateway, int, error)
	UpdateGateway(ctx context.Context, g *domain.Gateway) error
	DeleteGateway(ctx context.Context, id domain.ID) error
	UpdateGatewayHeartbeat(ctx context.Context, id domain.ID) error
}

// WireGuardPeerStore manages WireGuard peer persistence.
type WireGuardPeerStore interface {
	CreateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error
	GetWireGuardPeer(ctx context.Context, id domain.ID) (*domain.WireGuardPeer, error)
	GetWireGuardPeerByOwner(ctx context.Context, ownerType domain.PeerOwnerType, ownerID domain.ID) (*domain.WireGuardPeer, error)
	ListWireGuardPeers(ctx context.Context, params ListParams) ([]*domain.WireGuardPeer, int, error)
	UpdateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error
	DeleteWireGuardPeer(ctx context.Context, id domain.ID) error
}

// IPAllocationStore manages IP allocation persistence.
type IPAllocationStore interface {
	CreateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error
	GetIPAllocation(ctx context.Context, id domain.ID) (*domain.IPAllocation, error)
	ListIPAllocations(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.IPAllocation, int, error)
	UpdateIPAllocation(ctx context.Context, ip *domain.IPAllocation) error
	DeleteIPAllocation(ctx context.Context, id domain.ID) error
}

// BGPSessionStore manages BGP session persistence.
type BGPSessionStore interface {
	CreateBGPSession(ctx context.Context, s *domain.BGPSession) error
	GetBGPSession(ctx context.Context, id domain.ID) (*domain.BGPSession, error)
	ListBGPSessions(ctx context.Context, nodeID domain.ID, params ListParams) ([]*domain.BGPSession, int, error)
	UpdateBGPSession(ctx context.Context, s *domain.BGPSession) error
	DeleteBGPSession(ctx context.Context, id domain.ID) error
}

// DNSZoneStore manages DNS zone persistence.
type DNSZoneStore interface {
	CreateDNSZone(ctx context.Context, z *domain.DNSZone) error
	GetDNSZone(ctx context.Context, id domain.ID) (*domain.DNSZone, error)
	ListDNSZones(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.DNSZone, int, error)
	UpdateDNSZone(ctx context.Context, z *domain.DNSZone) error
	DeleteDNSZone(ctx context.Context, id domain.ID) error
	IncrementDNSZoneSerial(ctx context.Context, id domain.ID) error
}

// DNSRecordStore manages DNS record persistence.
type DNSRecordStore interface {
	CreateDNSRecord(ctx context.Context, r *domain.DNSRecord) error
	GetDNSRecord(ctx context.Context, id domain.ID) (*domain.DNSRecord, error)
	ListDNSRecords(ctx context.Context, zoneID domain.ID, params ListParams) ([]*domain.DNSRecord, int, error)
	UpdateDNSRecord(ctx context.Context, r *domain.DNSRecord) error
	DeleteDNSRecord(ctx context.Context, id domain.ID) error
}

// CDNSiteStore manages CDN site persistence.
type CDNSiteStore interface {
	CreateCDNSite(ctx context.Context, s *domain.CDNSite) error
	GetCDNSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error)
	ListCDNSites(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.CDNSite, int, error)
	UpdateCDNSite(ctx context.Context, s *domain.CDNSite) error
	DeleteCDNSite(ctx context.Context, id domain.ID) error
}

// CDNOriginStore manages CDN origin persistence.
type CDNOriginStore interface {
	CreateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error
	GetCDNOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error)
	ListCDNOrigins(ctx context.Context, siteID domain.ID, params ListParams) ([]*domain.CDNOrigin, int, error)
	UpdateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error
	DeleteCDNOrigin(ctx context.Context, id domain.ID) error
}

// RouteStore manages route persistence.
type RouteStore interface {
	CreateRoute(ctx context.Context, r *domain.Route) error
	GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error)
	ListRoutes(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.Route, int, error)
	ListRoutesByGateway(ctx context.Context, gatewayID domain.ID) ([]*domain.Route, error)
	UpdateRoute(ctx context.Context, r *domain.Route) error
	DeleteRoute(ctx context.Context, id domain.ID) error
}

// SSHKeyStore manages SSH key persistence.
type SSHKeyStore interface {
	CreateSSHKey(ctx context.Context, k *domain.SSHKey) error
	GetSSHKey(ctx context.Context, id domain.ID) (*domain.SSHKey, error)
	ListSSHKeys(ctx context.Context, params ListParams) ([]*domain.SSHKey, int, error)
	UpdateSSHKey(ctx context.Context, k *domain.SSHKey) error
	DeleteSSHKey(ctx context.Context, id domain.ID) error
}

// TLSCertificateStore manages TLS certificate persistence.
type TLSCertificateStore interface {
	CreateTLSCertificate(ctx context.Context, c *domain.TLSCertificate) error
	GetTLSCertificate(ctx context.Context, id domain.ID) (*domain.TLSCertificate, error)
	ListTLSCertificates(ctx context.Context, tenantID domain.ID, params ListParams) ([]*domain.TLSCertificate, int, error)
	DeleteTLSCertificate(ctx context.Context, id domain.ID) error
}

// AuditEventStore manages audit event persistence.
type AuditEventStore interface {
	CreateAuditEvent(ctx context.Context, e *domain.AuditEvent) error
	ListAuditEvents(ctx context.Context, tenantID *domain.ID, params ListParams) ([]*domain.AuditEvent, int, error)
}

// EnrollmentTokenStore manages enrollment token persistence.
type EnrollmentTokenStore interface {
	CreateEnrollmentToken(ctx context.Context, t *domain.EnrollmentToken) error
	GetEnrollmentToken(ctx context.Context, token string) (*domain.EnrollmentToken, error)
	MarkEnrollmentTokenUsed(ctx context.Context, id domain.ID) error
}

// ProvisioningJobStore manages provisioning job persistence.
type ProvisioningJobStore interface {
	CreateProvisioningJob(ctx context.Context, j *domain.ProvisioningJob) error
	GetProvisioningJob(ctx context.Context, id domain.ID) (*domain.ProvisioningJob, error)
	ListProvisioningJobs(ctx context.Context, nodeID *domain.ID, params ListParams) ([]*domain.ProvisioningJob, int, error)
	UpdateProvisioningJob(ctx context.Context, j *domain.ProvisioningJob) error
	// GetActiveProvisioningJob returns the currently running job for a node, or ErrNotFound.
	GetActiveProvisioningJob(ctx context.Context, nodeID domain.ID) (*domain.ProvisioningJob, error)
}
