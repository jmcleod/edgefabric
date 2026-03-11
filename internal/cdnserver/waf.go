package cdnserver

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/jmcleod/edgefabric/internal/observability"
)

// WAFMode determines how the WAF handles matches.
type WAFMode string

const (
	WAFModeDetect WAFMode = "detect" // Log only, request passes through.
	WAFModeBlock  WAFMode = "block"  // Log and return 403 Forbidden.
)

// WAF is a Web Application Firewall that inspects HTTP requests for malicious patterns.
type WAF struct {
	mode    WAFMode
	rules   []*WAFRule
	logger  *slog.Logger
	metrics *observability.Metrics
}

// WAFMatch describes a matched WAF rule.
type WAFMatch struct {
	Rule  *WAFRule
	Field string // Which part of the request matched (path, query, header:Name).
	Value string // The matched value (truncated for logging).
}

// NewWAF creates a WAF engine with the given mode, rules, and optional metrics.
func NewWAF(mode WAFMode, rules []*WAFRule, logger *slog.Logger, metrics *observability.Metrics) *WAF {
	if logger == nil {
		logger = slog.Default()
	}
	return &WAF{
		mode:    mode,
		rules:   rules,
		logger:  logger,
		metrics: metrics,
	}
}

// Inspect checks a request against all WAF rules. Returns the first match, or nil if clean.
func (w *WAF) Inspect(r *http.Request) *WAFMatch {
	if w.metrics != nil {
		w.metrics.WAFRequestsInspected.Inc()
	}

	// Check URL path.
	if match := w.checkValue(r.URL.Path, "path"); match != nil {
		return match
	}

	// Check query parameters (decoded values catch URL-encoded payloads).
	for key, vals := range r.URL.Query() {
		for _, v := range vals {
			if match := w.checkValue(v, "query:"+key); match != nil {
				return match
			}
		}
	}
	// Also check the full raw query string (decoded) for patterns that span
	// key=value boundaries or use characters like semicolons that Go's
	// url.Query() silently drops.
	if r.URL.RawQuery != "" {
		decoded, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			decoded = r.URL.RawQuery
		}
		if match := w.checkValue(decoded, "query"); match != nil {
			return match
		}
	}

	// Check selected headers.
	headersToCheck := []string{"User-Agent", "Referer", "Cookie"}
	for _, h := range headersToCheck {
		if v := r.Header.Get(h); v != "" {
			if match := w.checkValue(v, "header:"+h); match != nil {
				return match
			}
		}
	}

	return nil
}

// checkValue runs all rules against a single value.
func (w *WAF) checkValue(value, field string) *WAFMatch {
	for _, rule := range w.rules {
		if rule.Pattern.MatchString(value) {
			return &WAFMatch{
				Rule:  rule,
				Field: field,
				Value: truncate(value, 200),
			}
		}
	}
	return nil
}

// truncate limits a string to maxLen characters for safe logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
