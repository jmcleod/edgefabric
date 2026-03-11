package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	now := time.Now().UTC()
	site.CreatedAt = now
	site.UpdatedAt = now
	if site.Status == "" {
		site.Status = domain.CDNSiteActive
	}
	if site.CacheTTL == 0 {
		site.CacheTTL = 3600
	}

	headerRules := sql.NullString{}
	if site.HeaderRules != nil {
		headerRules = sql.NullString{String: string(site.HeaderRules), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO cdn_sites (id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl,
		 compression_enabled, rate_limit_rps, node_group_id, header_rules, waf_enabled, waf_mode,
		 status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		site.ID.String(), site.TenantID.String(), site.Name, string(site.TLSMode),
		nullIDString(site.TLSCertID), site.CacheEnabled, site.CacheTTL,
		site.CompressionEnabled, nullIntPtr(site.RateLimitRPS),
		nullIDString(site.NodeGroupID), headerRules,
		site.WAFEnabled, site.WAFMode,
		string(site.Status), site.CreatedAt, site.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: cdn site already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert cdn site: %w", err)
	}

	for _, d := range site.Domains {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO cdn_site_domains (site_id, domain) VALUES ($1, $2)`,
			site.ID.String(), d,
		)
		if err != nil {
			return fmt.Errorf("insert cdn site domain: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetCDNSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error) {
	site := &domain.CDNSite{}
	var tlsCertID, nodeGroupID, headerRules sql.NullString
	var rateLimitRPS sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl,
		        compression_enabled, rate_limit_rps, node_group_id, header_rules,
		        waf_enabled, waf_mode, status, created_at, updated_at
		 FROM cdn_sites WHERE id = $1`, id.String(),
	).Scan(&site.ID, &site.TenantID, &site.Name, &site.TLSMode, &tlsCertID,
		&site.CacheEnabled, &site.CacheTTL, &site.CompressionEnabled, &rateLimitRPS,
		&nodeGroupID, &headerRules,
		&site.WAFEnabled, &site.WAFMode,
		&site.Status, &site.CreatedAt, &site.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cdn site: %w", err)
	}

	if tlsCertID.Valid {
		parsed, err := uuid.Parse(tlsCertID.String)
		if err == nil {
			site.TLSCertID = &parsed
		}
	}
	if nodeGroupID.Valid {
		parsed, err := uuid.Parse(nodeGroupID.String)
		if err == nil {
			site.NodeGroupID = &parsed
		}
	}
	if headerRules.Valid {
		site.HeaderRules = json.RawMessage(headerRules.String)
	}
	if rateLimitRPS.Valid {
		v := int(rateLimitRPS.Int64)
		site.RateLimitRPS = &v
	}

	domains, err := s.loadCDNSiteDomains(ctx, id)
	if err != nil {
		return nil, err
	}
	site.Domains = domains

	return site, nil
}

func (s *PostgresStore) ListCDNSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdn_sites WHERE tenant_id = $1`, tenantID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cdn sites: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, tls_mode, tls_cert_id, cache_enabled, cache_ttl,
		        compression_enabled, rate_limit_rps, node_group_id, header_rules,
		        waf_enabled, waf_mode, status, created_at, updated_at
		 FROM cdn_sites WHERE tenant_id = $1 ORDER BY name ASC LIMIT $2 OFFSET $3`,
		tenantID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list cdn sites: %w", err)
	}
	defer rows.Close()

	var sites []*domain.CDNSite
	for rows.Next() {
		site := &domain.CDNSite{}
		var tlsCertID, nodeGroupID, headerRules sql.NullString
		var rateLimitRPS sql.NullInt64

		if err := rows.Scan(&site.ID, &site.TenantID, &site.Name, &site.TLSMode, &tlsCertID,
			&site.CacheEnabled, &site.CacheTTL, &site.CompressionEnabled, &rateLimitRPS,
			&nodeGroupID, &headerRules,
			&site.WAFEnabled, &site.WAFMode,
			&site.Status, &site.CreatedAt, &site.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan cdn site: %w", err)
		}

		if tlsCertID.Valid {
			parsed, err := uuid.Parse(tlsCertID.String)
			if err == nil {
				site.TLSCertID = &parsed
			}
		}
		if nodeGroupID.Valid {
			parsed, err := uuid.Parse(nodeGroupID.String)
			if err == nil {
				site.NodeGroupID = &parsed
			}
		}
		if headerRules.Valid {
			site.HeaderRules = json.RawMessage(headerRules.String)
		}
		if rateLimitRPS.Valid {
			v := int(rateLimitRPS.Int64)
			site.RateLimitRPS = &v
		}

		domains, err := s.loadCDNSiteDomains(ctx, site.ID)
		if err != nil {
			return nil, 0, err
		}
		site.Domains = domains

		sites = append(sites, site)
	}
	return sites, total, rows.Err()
}

func (s *PostgresStore) UpdateCDNSite(ctx context.Context, site *domain.CDNSite) error {
	site.UpdatedAt = time.Now().UTC()

	headerRules := sql.NullString{}
	if site.HeaderRules != nil {
		headerRules = sql.NullString{String: string(site.HeaderRules), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE cdn_sites SET name = $1, tls_mode = $2, tls_cert_id = $3, cache_enabled = $4, cache_ttl = $5,
		 compression_enabled = $6, rate_limit_rps = $7, node_group_id = $8, header_rules = $9,
		 waf_enabled = $10, waf_mode = $11, status = $12, updated_at = $13
		 WHERE id = $14`,
		site.Name, string(site.TLSMode), nullIDString(site.TLSCertID),
		site.CacheEnabled, site.CacheTTL, site.CompressionEnabled,
		nullIntPtr(site.RateLimitRPS), nullIDString(site.NodeGroupID),
		headerRules, site.WAFEnabled, site.WAFMode,
		string(site.Status), site.UpdatedAt, site.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update cdn site: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}

	// Delete all existing domains and reinsert.
	_, err = tx.ExecContext(ctx, `DELETE FROM cdn_site_domains WHERE site_id = $1`, site.ID.String())
	if err != nil {
		return fmt.Errorf("delete old cdn site domains: %w", err)
	}
	for _, d := range site.Domains {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO cdn_site_domains (site_id, domain) VALUES ($1, $2)`,
			site.ID.String(), d,
		)
		if err != nil {
			return fmt.Errorf("insert cdn site domain: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) DeleteCDNSite(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cdn_sites WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete cdn site: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadCDNSiteDomains returns the domains associated with a CDN site.
func (s *PostgresStore) loadCDNSiteDomains(ctx context.Context, siteID domain.ID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT domain FROM cdn_site_domains WHERE site_id = $1`, siteID.String(),
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
