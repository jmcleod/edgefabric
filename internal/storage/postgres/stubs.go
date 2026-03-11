package postgres

// Stub implementations for store interfaces not yet needed.
// These satisfy the storage.Store interface contract at compile time.

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// --- TLSCertificateStore stubs (not yet implemented) ---

func (s *PostgresStore) CreateTLSCertificate(ctx context.Context, c *domain.TLSCertificate) error {
	return fmt.Errorf("TLSCertificateStore: not implemented")
}

func (s *PostgresStore) GetTLSCertificate(ctx context.Context, id domain.ID) (*domain.TLSCertificate, error) {
	return nil, fmt.Errorf("TLSCertificateStore: not implemented")
}

func (s *PostgresStore) ListTLSCertificates(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.TLSCertificate, int, error) {
	return nil, 0, fmt.Errorf("TLSCertificateStore: not implemented")
}

func (s *PostgresStore) DeleteTLSCertificate(ctx context.Context, id domain.ID) error {
	return fmt.Errorf("TLSCertificateStore: not implemented")
}
