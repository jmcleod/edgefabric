package cdn

import (
	"encoding/json"
	"fmt"
	"net"
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

	if err := validateWAFMode(site.WAFMode); err != nil {
		return err
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

	if err := validateWAFMode(site.WAFMode); err != nil {
		return err
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
// When allowPrivate is true, SSRF checks are skipped (for demo/dev environments).
func validateOrigin(o *domain.CDNOrigin, allowPrivate bool) error {
	if o.Address == "" {
		return fmt.Errorf("origin address is required")
	}

	// SSRF protection: block origins pointing at internal/private networks.
	if !allowPrivate {
		if err := validateOriginAddress(o.Address); err != nil {
			return fmt.Errorf("origin address: %w", err)
		}
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

// validateOriginAddress performs SSRF protection by rejecting origin addresses
// that resolve to private, loopback, link-local, or cloud metadata IPs.
// This prevents semi-trusted tenants from targeting internal services.
func validateOriginAddress(address string) error {
	// Strip port if present (address may be "host:port" or just "host").
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}

	// Block well-known internal hostnames.
	lower := strings.ToLower(host)
	blockedHosts := []string{
		"localhost",
		"metadata.google.internal",
		"metadata.google",
		"kubernetes.default",
		"kubernetes.default.svc",
	}
	for _, blocked := range blockedHosts {
		if lower == blocked || strings.HasSuffix(lower, "."+blocked) {
			return fmt.Errorf("origin address %q resolves to a blocked internal host", address)
		}
	}

	// Resolve to IPs and check each one.
	ips, err := net.LookupHost(host)
	if err != nil {
		// If it's a raw IP, parse it directly.
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("cannot resolve origin address %q: %v", address, err)
		}
		ips = []string{ip.String()}
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isBlockedIP(ip) {
			return fmt.Errorf("origin address %q resolves to blocked IP %s (private/internal network)", address, ipStr)
		}
	}

	return nil
}

// isBlockedIP returns true if the IP is in a private, loopback, link-local,
// or cloud metadata range that should not be used as a CDN origin.
func isBlockedIP(ip net.IP) bool {
	// Loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return true
	}

	// Link-local (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Private ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	// AWS/GCP/Azure metadata endpoints (169.254.169.254, fd00:ec2::254)
	metadataIPs := []string{
		"169.254.169.254",
		"fd00:ec2::254",
	}
	for _, mip := range metadataIPs {
		if ip.Equal(net.ParseIP(mip)) {
			return true
		}
	}

	// Unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return true
	}

	return false
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

// validateWAFMode validates the WAF mode setting.
func validateWAFMode(mode string) error {
	switch mode {
	case "detect", "block", "":
		return nil
	default:
		return fmt.Errorf("invalid waf_mode: %q (must be detect, block, or empty)", mode)
	}
}
