package sqlite

// Stub implementations for store interfaces not yet needed (Milestone 3+).
// These satisfy the storage.Store interface contract at compile time.
// Each will be replaced with a real implementation in its respective milestone.

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// Gateway, WireGuardPeer, and EnrollmentToken stores are implemented in
// gateway.go, wireguard_peer.go, and enrollment_token.go respectively.

// IPAllocationStore and BGPSessionStore are implemented in
// ip_allocation.go and bgp_session.go respectively.

// DNSZoneStore and DNSRecordStore are implemented in
// dns_zone.go and dns_record.go respectively.

// --- CDNSiteStore stubs (Milestone 7) ---

func (s *SQLiteStore) CreateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	return fmt.Errorf("CDNSiteStore: not implemented")
}
func (s *SQLiteStore) GetCDNSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error) {
	return nil, fmt.Errorf("CDNSiteStore: not implemented")
}
func (s *SQLiteStore) ListCDNSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error) {
	return nil, 0, fmt.Errorf("CDNSiteStore: not implemented")
}
func (s *SQLiteStore) UpdateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	return fmt.Errorf("CDNSiteStore: not implemented")
}
func (s *SQLiteStore) DeleteCDNSite(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("CDNSiteStore: not implemented")
}

// --- CDNOriginStore stubs (Milestone 7) ---

func (s *SQLiteStore) CreateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	return fmt.Errorf("CDNOriginStore: not implemented")
}
func (s *SQLiteStore) GetCDNOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error) {
	return nil, fmt.Errorf("CDNOriginStore: not implemented")
}
func (s *SQLiteStore) ListCDNOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error) {
	return nil, 0, fmt.Errorf("CDNOriginStore: not implemented")
}
func (s *SQLiteStore) UpdateCDNOrigin(ctx context.Context, o *domain.CDNOrigin) error {
	return fmt.Errorf("CDNOriginStore: not implemented")
}
func (s *SQLiteStore) DeleteCDNOrigin(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("CDNOriginStore: not implemented")
}

// --- RouteStore stubs (Milestone 8) ---

func (s *SQLiteStore) CreateRoute(ctx context.Context, r *domain.Route) error {
	return fmt.Errorf("RouteStore: not implemented")
}
func (s *SQLiteStore) GetRoute(ctx context.Context, id domain.ID) (*domain.Route, error) {
	return nil, fmt.Errorf("RouteStore: not implemented")
}
func (s *SQLiteStore) ListRoutes(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.Route, int, error) {
	return nil, 0, fmt.Errorf("RouteStore: not implemented")
}
func (s *SQLiteStore) UpdateRoute(ctx context.Context, r *domain.Route) error {
	return fmt.Errorf("RouteStore: not implemented")
}
func (s *SQLiteStore) DeleteRoute(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("RouteStore: not implemented")
}

// --- TLSCertificateStore stubs (Milestone 7) ---

func (s *SQLiteStore) CreateTLSCertificate(ctx context.Context, c *domain.TLSCertificate) error {
	return fmt.Errorf("TLSCertificateStore: not implemented")
}
func (s *SQLiteStore) GetTLSCertificate(ctx context.Context, id domain.ID) (*domain.TLSCertificate, error) {
	return nil, fmt.Errorf("TLSCertificateStore: not implemented")
}
func (s *SQLiteStore) ListTLSCertificates(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.TLSCertificate, int, error) {
	return nil, 0, fmt.Errorf("TLSCertificateStore: not implemented")
}
func (s *SQLiteStore) DeleteTLSCertificate(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("TLSCertificateStore: not implemented")
}

// ProvisioningJobStore and UpdateSSHKey are implemented in
// provisioning_job.go and sshkey.go respectively.
