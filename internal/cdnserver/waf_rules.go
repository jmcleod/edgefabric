package cdnserver

import "regexp"

// WAFRuleCategory identifies the class of attack a WAF rule detects.
type WAFRuleCategory string

const (
	WAFCategorySQLi          WAFRuleCategory = "sqli"
	WAFCategoryXSS           WAFRuleCategory = "xss"
	WAFCategoryPathTraversal WAFRuleCategory = "path_traversal"
	WAFCategoryProtocol      WAFRuleCategory = "protocol"
)

// WAFRule is a compiled regex-based detection rule.
type WAFRule struct {
	ID          string
	Category    WAFRuleCategory
	Pattern     *regexp.Regexp
	Description string
}

// DefaultRules returns the built-in WAF detection rules.
func DefaultRules() []*WAFRule {
	return []*WAFRule{
		// --- SQL Injection ---
		{
			ID:          "sqli-union-select",
			Category:    WAFCategorySQLi,
			Pattern:     regexp.MustCompile(`(?i)(union\s+(all\s+)?select)`),
			Description: "SQL injection: UNION SELECT",
		},
		{
			ID:          "sqli-comment",
			Category:    WAFCategorySQLi,
			Pattern:     regexp.MustCompile(`(/\*.*?\*/|--|#)`),
			Description: "SQL injection: comment injection",
		},
		{
			ID:          "sqli-tautology",
			Category:    WAFCategorySQLi,
			Pattern:     regexp.MustCompile(`(?i)'\s*(or|and)\s+['"\d]`),
			Description: "SQL injection: tautology",
		},
		{
			ID:          "sqli-sleep",
			Category:    WAFCategorySQLi,
			Pattern:     regexp.MustCompile(`(?i)(sleep\s*\(|benchmark\s*\(|waitfor\s+delay)`),
			Description: "SQL injection: time-based blind",
		},

		// --- Cross-Site Scripting ---
		{
			ID:          "xss-script-tag",
			Category:    WAFCategoryXSS,
			Pattern:     regexp.MustCompile(`(?i)<\s*script[^>]*>`),
			Description: "XSS: script tag",
		},
		{
			ID:          "xss-event-handler",
			Category:    WAFCategoryXSS,
			Pattern:     regexp.MustCompile(`(?i)\bon\w+\s*=`),
			Description: "XSS: event handler attribute",
		},
		{
			ID:          "xss-javascript-uri",
			Category:    WAFCategoryXSS,
			Pattern:     regexp.MustCompile(`(?i)javascript\s*:`),
			Description: "XSS: javascript: URI",
		},

		// --- Path Traversal ---
		{
			ID:          "traversal-dotdot",
			Category:    WAFCategoryPathTraversal,
			Pattern:     regexp.MustCompile(`\.\./`),
			Description: "Path traversal: directory traversal",
		},
		{
			ID:          "traversal-etc-passwd",
			Category:    WAFCategoryPathTraversal,
			Pattern:     regexp.MustCompile(`(?i)/etc/passwd`),
			Description: "Path traversal: /etc/passwd access",
		},

		// --- Protocol Violations ---
		{
			ID:          "proto-null-byte",
			Category:    WAFCategoryProtocol,
			Pattern:     regexp.MustCompile(`%00|\x00`),
			Description: "Protocol violation: null byte injection",
		},
		{
			ID:          "proto-crlf",
			Category:    WAFCategoryProtocol,
			Pattern:     regexp.MustCompile(`%0[dD]%0[aA]|\r\n`),
			Description: "Protocol violation: CRLF injection",
		},
	}
}
