package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	now := time.Now().UTC()
	site.CreatedAt = now
	site.UpdatedAt = now
	if site.Status == "" {
		site.Status = domain.CDNSiteActive
	}
	if site.CacheTTL == 0 {
		site.CacheTTL = 3600
	}

	var nodeGroupID *string
	if site.NodeGroupID != nil {
		s := site.NodeGroupID.String()
		nodeGroupID = &s
	}
	var tlsCertID *string
	if site.TLSCertID != nil {
		s := site.TLSCertID.String()
		tlsCertID = &s
	}
	var headerRules *string
	if site.HeaderRules != nil {
		s := string(site.HeaderRules)
		headerRules = &s
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cdn_sites (id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl, compression_enabled, rate_limit_rps, node_group_id, header_rules, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		site.ID.String(), site.TenantID.String(), site.Name,
		string(site.TLSMode), tlsCertID,
		site.CacheEnabled, site.CacheTTL, site.CompressionEnabled,
		site.RateLimitRPS, nodeGroupID, headerRules,
		string(site.Status), site.CreatedAt, site.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: cdn site already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert cdn site: %w", err)
	}

	// Insert domains into junction table.
	for _, d := range site.Domains {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO cdn_site_domains (site_id, domain) VALUES (?, ?)`,
			site.ID.String(), d,
		)
		if err != nil {
			return fmt.Errorf("insert cdn site domain %q: %w", d, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetCDNSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error) {
	site := &domain.CDNSite{}
	var headerRules sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl,
		        compression_enabled, rate_limit_rps, node_group_id, header_rules, status,
		        created_at, updated_at
		 FROM cdn_sites WHERE id = ?`, id.String(),
	).Scan(&site.ID, &site.TenantID, &site.Name,
		&site.TLSMode, &site.TLSCertID,
		&site.CacheEnabled, &site.CacheTTL, &site.CompressionEnabled,
		&site.RateLimitRPS, &site.NodeGroupID, &headerRules,
		&site.Status, &site.CreatedAt, &site.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cdn site: %w", err)
	}

	if headerRules.Valid {
		site.HeaderRules = json.RawMessage(headerRules.String)
	}

	// Load domains from junction table.
	domains, err := s.loadCDNSiteDomains(ctx, id)
	if err != nil {
		return nil, err
	}
	site.Domains = domains

	return site, nil
}

func (s *SQLiteStore) ListCDNSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdn_sites WHERE tenant_id = ?`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cdn sites: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl,
		        compression_enabled, rate_limit_rps, node_group_id, header_rules, status,
		        created_at, updated_at
		 FROM cdn_sites WHERE tenant_id = ? ORDER BY name ASC LIMIT ? OFFSET ?`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list cdn sites: %w", err)
	}
	defer rows.Close()

	var sites []*domain.CDNSite
	for rows.Next() {
		site := &domain.CDNSite{}
		var headerRules sql.NullString
		if err := rows.Scan(&site.ID, &site.TenantID, &site.Name,
			&site.TLSMode, &site.TLSCertID,
			&site.CacheEnabled, &site.CacheTTL, &site.CompressionEnabled,
			&site.RateLimitRPS, &site.NodeGroupID, &headerRules,
			&site.Status, &site.CreatedAt, &site.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan cdn site: %w", err)
		}
		if headerRules.Valid {
			site.HeaderRules = json.RawMessage(headerRules.String)
		}
		sites = append(sites, site)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Batch-load domains for each site.
	for _, site := range sites {
		domains, err := s.loadCDNSiteDomains(ctx, site.ID)
		if err != nil {
			return nil, 0, err
		}
		site.Domains = domains
	}

	return sites, total, nil
}

func (s *SQLiteStore) UpdateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	site.UpdatedAt = time.Now().UTC()

	var nodeGroupID *string
	if site.NodeGroupID != nil {
		s := site.NodeGroupID.String()
		nodeGroupID = &s
	}
	var tlsCertID *string
	if site.TLSCertID != nil {
		s := site.TLSCertID.String()
		tlsCertID = &s
	}
	var headerRules *string
	if site.HeaderRules != nil {
		s := string(site.HeaderRules)
		headerRules = &s
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE cdn_sites SET name = ?, tls_mode = ?, tls_cert_id = ?, cache_enabled = ?,
		 cache_ttl = ?, compression_enabled = ?, rate_limit_rps = ?, node_group_id = ?,
		 header_rules = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		site.Name, string(site.TLSMode), tlsCertID,
		site.CacheEnabled, site.CacheTTL, site.CompressionEnabled,
		site.RateLimitRPS, nodeGroupID, headerRules,
		string(site.Status), site.UpdatedAt, site.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update cdn site: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}

	// Re-sync domains: delete all, then re-insert.
	_, err = tx.ExecContext(ctx, `DELETE FROM cdn_site_domains WHERE site_id = ?`, site.ID.String())
	if err != nil {
		return fmt.Errorf("delete cdn site domains: %w", err)
	}
	for _, d := range site.Domains {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO cdn_site_domains (site_id, domain) VALUES (?, ?)`,
			site.ID.String(), d,
		)
		if err != nil {
			return fmt.Errorf("insert cdn site domain %q: %w", d, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) DeleteCDNSite(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cdn_sites WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete cdn site: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadCDNSiteDomains loads domains from the cdn_site_domains junction table.
func (s *SQLiteStore) loadCDNSiteDomains(ctx context.Context, siteID domain.ID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT domain FROM cdn_site_domains WHERE site_id = ? ORDER BY domain ASC`,
		siteID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("load cdn site domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan cdn site domain: %w", err)
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}
