package cdn

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// hostnameRegex matches valid DNS hostnames (RFC 1123).
var hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.?$`)

// validateSite validates a CDN site configuration for creation.
func validateSite(site *domain.CDNSite) error {
	if site.Name == "" {
		return fmt.Errorf("site name is required")
	}
	if len(site.Name) > 255 {
		return fmt.Errorf("site name too long (max 255 characters)")
	}

	if len(site.Domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}
	for _, d := range site.Domains {
		if err := validateDomain(d); err != nil {
			return fmt.Errorf("invalid domain %q: %w", d, err)
		}
	}

	if err := validateTLSMode(site.TLSMode); err != nil {
		return err
	}

	if site.CacheTTL < 0 {
		return fmt.Errorf("cache_ttl must be >= 0, got %d", site.CacheTTL)
	}

	if site.RateLimitRPS != nil && *site.RateLimitRPS <= 0 {
		return fmt.Errorf("rate_limit_rps must be > 0 when set, got %d", *site.RateLimitRPS)
	}

	if site.HeaderRules != nil {
		if err := validateHeaderRules(site.HeaderRules); err != nil {
			return fmt.Errorf("invalid header_rules: %w", err)
		}
	}

	return nil
}

// validateSiteUpdate validates a CDN site update (only checks provided fields).
func validateSiteUpdate(site *domain.CDNSite) error {
	if site.Name == "" {
		return fmt.Errorf("site name is required")
	}
	if len(site.Name) > 255 {
		return fmt.Errorf("site name too long (max 255 characters)")
	}

	for _, d := range site.Domains {
		if err := validateDomain(d); err != nil {
			return fmt.Errorf("invalid domain %q: %w", d, err)
		}
	}

	if err := validateTLSMode(site.TLSMode); err != nil {
		return err
	}

	if site.CacheTTL < 0 {
		return fmt.Errorf("cache_ttl must be >= 0, got %d", site.CacheTTL)
	}

	if site.RateLimitRPS != nil && *site.RateLimitRPS <= 0 {
		return fmt.Errorf("rate_limit_rps must be > 0 when set, got %d", *site.RateLimitRPS)
	}

	if site.HeaderRules != nil {
		if err := validateHeaderRules(site.HeaderRules); err != nil {
			return fmt.Errorf("invalid header_rules: %w", err)
		}
	}

	return nil
}

// validateDomain validates that a domain string is a valid hostname.
func validateDomain(d string) error {
	if d == "" {
		return fmt.Errorf("domain must not be empty")
	}
	clean := strings.TrimSuffix(d, ".")
	if len(clean) > 253 {
		return fmt.Errorf("domain too long (max 253 characters)")
	}
	if !hostnameRegex.MatchString(clean) {
		return fmt.Errorf("not a valid hostname")
	}
	return nil
}

// validateTLSMode validates the TLS mode.
func validateTLSMode(mode domain.TLSMode) error {
	switch mode {
	case domain.TLSModeAuto, domain.TLSModeManual, domain.TLSModeDisabled:
		return nil
	case "":
		return fmt.Errorf("tls_mode is required")
	default:
		return fmt.Errorf("invalid tls_mode: %q (must be auto, manual, or disabled)", mode)
	}
}

// HeaderRule represents a single header manipulation rule.
type HeaderRule struct {
	Action string `json:"action"` // "add", "set", "remove"
	Header string `json:"header"`
	Value  string `json:"value,omitempty"` // Not required for "remove"
}

// validateHeaderRules validates a JSON array of header rules.
func validateHeaderRules(raw json.RawMessage) error {
	var rules []HeaderRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return fmt.Errorf("must be a valid JSON array: %w", err)
	}

	for i, rule := range rules {
		if rule.Header == "" {
			return fmt.Errorf("rule[%d]: header is required", i)
		}

		switch rule.Action {
		case "add", "set":
			// Value is required for add and set.
			if rule.Value == "" {
				return fmt.Errorf("rule[%d]: value is required for action %q", i, rule.Action)
			}
		case "remove":
			// Value is optional for remove.
		default:
			return fmt.Errorf("rule[%d]: invalid action %q (must be add, set, or remove)", i, rule.Action)
		}
	}

	return nil
}

// validateOrigin validates a CDN origin configuration for creation.
func validateOrigin(o *domain.CDNOrigin) error {
	if o.Address == "" {
		return fmt.Errorf("origin address is required")
	}

	if err := validateOriginScheme(o.Scheme); err != nil {
		return err
	}

	if o.Weight < 1 {
		return fmt.Errorf("origin weight must be >= 1, got %d", o.Weight)
	}

	if o.HealthCheckPath != "" && !strings.HasPrefix(o.HealthCheckPath, "/") {
		return fmt.Errorf("health_check_path must start with '/', got %q", o.HealthCheckPath)
	}

	if o.HealthCheckInterval != nil && *o.HealthCheckInterval < 5 {
		return fmt.Errorf("health_check_interval must be >= 5 seconds when set, got %d", *o.HealthCheckInterval)
	}

	return nil
}

// validateOriginScheme validates the origin scheme.
func validateOriginScheme(scheme domain.CDNOriginScheme) error {
	switch scheme {
	case domain.CDNOriginHTTP, domain.CDNOriginHTTPS:
		return nil
	case "":
		return fmt.Errorf("origin scheme is required")
	default:
		return fmt.Errorf("invalid origin scheme: %q (must be http or https)", scheme)
	}
}
