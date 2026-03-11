package sqlite

// Stub implementations for store interfaces not yet needed.
// These satisfy the storage.Store interface contract at compile time.

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

// CDNSiteStore and CDNOriginStore are implemented in
// cdn_site.go and cdn_origin.go respectively.

// RouteStore is implemented in route.go.

// --- TLSCertificateStore stubs ---

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
