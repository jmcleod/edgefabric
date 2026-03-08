package cdn

import (
	"context"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

// DefaultService is the controller-side CDN management service.
type DefaultService struct {
	sites      storage.CDNSiteStore
	origins    storage.CDNOriginStore
	nodeGroups storage.NodeGroupStore
	nodes      storage.NodeStore
}

// NewService creates a new CDN service.
func NewService(
	sites storage.CDNSiteStore,
	origins storage.CDNOriginStore,
	nodeGroups storage.NodeGroupStore,
	nodes storage.NodeStore,
) *DefaultService {
	return &DefaultService{
		sites:      sites,
		origins:    origins,
		nodeGroups: nodeGroups,
		nodes:      nodes,
	}
}

// --- Site CRUD ---

func (s *DefaultService) CreateSite(ctx context.Context, req CreateSiteRequest) (*domain.CDNSite, error) {
	site := &domain.CDNSite{
		ID:                 domain.NewID(),
		TenantID:           req.TenantID,
		Name:               req.Name,
		Domains:            req.Domains,
		TLSMode:            req.TLSMode,
		CacheEnabled:       req.CacheEnabled,
		CacheTTL:           req.CacheTTL,
		CompressionEnabled: req.CompressionEnabled,
		RateLimitRPS:       req.RateLimitRPS,
		NodeGroupID:        req.NodeGroupID,
		HeaderRules:        req.HeaderRules,
	}

	if err := validateSite(site); err != nil {
		return nil, fmt.Errorf("invalid site: %w", err)
	}

	if err := s.sites.CreateCDNSite(ctx, site); err != nil {
		return nil, fmt.Errorf("create site: %w", err)
	}
	return site, nil
}

func (s *DefaultService) GetSite(ctx context.Context, id domain.ID) (*domain.CDNSite, error) {
	return s.sites.GetCDNSite(ctx, id)
}

func (s *DefaultService) ListSites(ctx context.Context, tenantID domain.ID, params storage.ListParams) ([]*domain.CDNSite, int, error) {
	return s.sites.ListCDNSites(ctx, tenantID, params)
}

func (s *DefaultService) UpdateSite(ctx context.Context, id domain.ID, req UpdateSiteRequest) (*domain.CDNSite, error) {
	site, err := s.sites.GetCDNSite(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		site.Name = *req.Name
	}
	if req.Domains != nil {
		site.Domains = req.Domains
	}
	if req.TLSMode != nil {
		site.TLSMode = *req.TLSMode
	}
	if req.CacheEnabled != nil {
		site.CacheEnabled = *req.CacheEnabled
	}
	if req.CacheTTL != nil {
		site.CacheTTL = *req.CacheTTL
	}
	if req.CompressionEnabled != nil {
		site.CompressionEnabled = *req.CompressionEnabled
	}
	if req.ClearRateLimit {
		site.RateLimitRPS = nil
	} else if req.RateLimitRPS != nil {
		site.RateLimitRPS = req.RateLimitRPS
	}
	if req.ClearNodeGroup {
		site.NodeGroupID = nil
	} else if req.NodeGroupID != nil {
		site.NodeGroupID = req.NodeGroupID
	}
	if req.HeaderRules != nil {
		site.HeaderRules = req.HeaderRules
	}
	if req.Status != nil {
		site.Status = *req.Status
	}

	if err := validateSiteUpdate(site); err != nil {
		return nil, fmt.Errorf("invalid site: %w", err)
	}

	if err := s.sites.UpdateCDNSite(ctx, site); err != nil {
		return nil, fmt.Errorf("update site: %w", err)
	}
	return site, nil
}

func (s *DefaultService) DeleteSite(ctx context.Context, id domain.ID) error {
	return s.sites.DeleteCDNSite(ctx, id)
}

// --- Origin CRUD ---

func (s *DefaultService) CreateOrigin(ctx context.Context, req CreateOriginRequest) (*domain.CDNOrigin, error) {
	// Verify site exists.
	_, err := s.sites.GetCDNSite(ctx, req.SiteID)
	if err != nil {
		return nil, fmt.Errorf("site lookup: %w", err)
	}

	origin := &domain.CDNOrigin{
		ID:                  domain.NewID(),
		SiteID:              req.SiteID,
		Address:             req.Address,
		Scheme:              req.Scheme,
		Weight:              req.Weight,
		HealthCheckPath:     req.HealthCheckPath,
		HealthCheckInterval: req.HealthCheckInterval,
	}

	// Default weight to 1 if not set.
	if origin.Weight == 0 {
		origin.Weight = 1
	}

	if err := validateOrigin(origin); err != nil {
		return nil, fmt.Errorf("invalid origin: %w", err)
	}

	if err := s.origins.CreateCDNOrigin(ctx, origin); err != nil {
		return nil, fmt.Errorf("create origin: %w", err)
	}
	return origin, nil
}

func (s *DefaultService) GetOrigin(ctx context.Context, id domain.ID) (*domain.CDNOrigin, error) {
	return s.origins.GetCDNOrigin(ctx, id)
}

func (s *DefaultService) ListOrigins(ctx context.Context, siteID domain.ID, params storage.ListParams) ([]*domain.CDNOrigin, int, error) {
	return s.origins.ListCDNOrigins(ctx, siteID, params)
}

func (s *DefaultService) UpdateOrigin(ctx context.Context, id domain.ID, req UpdateOriginRequest) (*domain.CDNOrigin, error) {
	origin, err := s.origins.GetCDNOrigin(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Address != nil {
		origin.Address = *req.Address
	}
	if req.Scheme != nil {
		origin.Scheme = *req.Scheme
	}
	if req.Weight != nil {
		origin.Weight = *req.Weight
	}
	if req.HealthCheckPath != nil {
		origin.HealthCheckPath = *req.HealthCheckPath
	}
	if req.HealthCheckInterval != nil {
		origin.HealthCheckInterval = req.HealthCheckInterval
	}
	if req.Status != nil {
		origin.Status = *req.Status
	}

	if err := validateOrigin(origin); err != nil {
		return nil, fmt.Errorf("invalid origin: %w", err)
	}

	if err := s.origins.UpdateCDNOrigin(ctx, origin); err != nil {
		return nil, fmt.Errorf("update origin: %w", err)
	}
	return origin, nil
}

func (s *DefaultService) DeleteOrigin(ctx context.Context, id domain.ID) error {
	return s.origins.DeleteCDNOrigin(ctx, id)
}

// --- Cache purge ---

func (s *DefaultService) PurgeSiteCache(ctx context.Context, siteID domain.ID) error {
	// Verify site exists.
	_, err := s.sites.GetCDNSite(ctx, siteID)
	if err != nil {
		return fmt.Errorf("site lookup: %w", err)
	}
	// In v1, purge is handled node-side during reconciliation.
	// The controller acknowledges the purge request; actual cache invalidation
	// happens when nodes next poll their config.
	return nil
}

// --- Config sync ---

// GetNodeCDNConfig returns the complete CDN configuration for a node.
// It finds all sites assigned to node groups that the node belongs to,
// and includes all origins for each site.
func (s *DefaultService) GetNodeCDNConfig(ctx context.Context, nodeID domain.ID) (*NodeCDNConfig, error) {
	// Get the node to verify it exists.
	_, err := s.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	// Get all node groups this node belongs to.
	groups, err := s.nodeGroups.ListNodeGroups_ByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list node groups: %w", err)
	}

	// For each group, find sites assigned to it.
	config := &NodeCDNConfig{}
	seenSites := make(map[domain.ID]bool)

	for _, group := range groups {
		// List all sites for this tenant.
		sites, _, err := s.sites.ListCDNSites(ctx, group.TenantID, storage.ListParams{Limit: 10000})
		if err != nil {
			return nil, fmt.Errorf("list sites for group %s: %w", group.ID, err)
		}

		for _, site := range sites {
			// Only include sites assigned to this group and active.
			if site.NodeGroupID == nil || *site.NodeGroupID != group.ID {
				continue
			}
			if site.Status != domain.CDNSiteActive {
				continue
			}
			if seenSites[site.ID] {
				continue
			}
			seenSites[site.ID] = true

			// Load origins for this site.
			origins, _, err := s.origins.ListCDNOrigins(ctx, site.ID, storage.ListParams{Limit: 10000})
			if err != nil {
				return nil, fmt.Errorf("list origins for site %s: %w", site.Name, err)
			}

			config.Sites = append(config.Sites, SiteWithOrigins{
				Site:    site,
				Origins: origins,
			})
		}
	}

	return config, nil
}
