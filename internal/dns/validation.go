package dns

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// hostnameRegex matches valid DNS hostnames (RFC 1123).
var hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9_]([a-zA-Z0-9_-]{0,61}[a-zA-Z0-9_])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.?$`)

// validateZoneName validates that a zone name is a valid DNS zone (FQDN-like).
func validateZoneName(name string) error {
	if name == "" {
		return fmt.Errorf("zone name is required")
	}
	// Strip trailing dot for validation.
	clean := strings.TrimSuffix(name, ".")
	if len(clean) > 253 {
		return fmt.Errorf("zone name too long (max 253 characters)")
	}
	if !hostnameRegex.MatchString(clean) {
		return fmt.Errorf("invalid zone name: %q", name)
	}
	return nil
}

// validateRecord validates a DNS record based on its type.
func validateRecord(r *domain.DNSRecord) error {
	if r.Name == "" {
		return fmt.Errorf("record name is required")
	}
	if r.Value == "" {
		return fmt.Errorf("record value is required")
	}
	if r.Type == "" {
		return fmt.Errorf("record type is required")
	}

	switch r.Type {
	case domain.DNSRecordTypeA:
		return validateA(r)
	case domain.DNSRecordTypeAAAA:
		return validateAAAA(r)
	case domain.DNSRecordTypeCNAME:
		return validateCNAME(r)
	case domain.DNSRecordTypeMX:
		return validateMX(r)
	case domain.DNSRecordTypeTXT:
		return validateTXT(r)
	case domain.DNSRecordTypeNS:
		return validateNS(r)
	case domain.DNSRecordTypeSRV:
		return validateSRV(r)
	case domain.DNSRecordTypeCAA:
		return validateCAA(r)
	case domain.DNSRecordTypePTR:
		return validatePTR(r)
	default:
		return fmt.Errorf("unsupported record type: %q", r.Type)
	}
}

func validateA(r *domain.DNSRecord) error {
	ip := net.ParseIP(r.Value)
	if ip == nil {
		return fmt.Errorf("A record value must be a valid IP address: %q", r.Value)
	}
	if ip.To4() == nil {
		return fmt.Errorf("A record value must be an IPv4 address: %q", r.Value)
	}
	return nil
}

func validateAAAA(r *domain.DNSRecord) error {
	ip := net.ParseIP(r.Value)
	if ip == nil {
		return fmt.Errorf("AAAA record value must be a valid IP address: %q", r.Value)
	}
	if ip.To4() != nil {
		return fmt.Errorf("AAAA record value must be an IPv6 address, not IPv4: %q", r.Value)
	}
	return nil
}

func validateCNAME(r *domain.DNSRecord) error {
	if !isValidHostname(r.Value) {
		return fmt.Errorf("CNAME record value must be a valid hostname: %q", r.Value)
	}
	return nil
}

func validateMX(r *domain.DNSRecord) error {
	if r.Priority == nil {
		return fmt.Errorf("MX record requires priority")
	}
	if *r.Priority < 0 || *r.Priority > 65535 {
		return fmt.Errorf("MX record priority must be 0-65535, got %d", *r.Priority)
	}
	if !isValidHostname(r.Value) {
		return fmt.Errorf("MX record value must be a valid hostname: %q", r.Value)
	}
	return nil
}

func validateTXT(r *domain.DNSRecord) error {
	// TXT records just need a non-empty value, already checked above.
	return nil
}

func validateNS(r *domain.DNSRecord) error {
	if !isValidHostname(r.Value) {
		return fmt.Errorf("NS record value must be a valid hostname: %q", r.Value)
	}
	return nil
}

func validateSRV(r *domain.DNSRecord) error {
	if r.Priority == nil {
		return fmt.Errorf("SRV record requires priority")
	}
	if r.Weight == nil {
		return fmt.Errorf("SRV record requires weight")
	}
	if r.Port == nil {
		return fmt.Errorf("SRV record requires port")
	}
	if *r.Priority < 0 || *r.Priority > 65535 {
		return fmt.Errorf("SRV record priority must be 0-65535, got %d", *r.Priority)
	}
	if *r.Weight < 0 || *r.Weight > 65535 {
		return fmt.Errorf("SRV record weight must be 0-65535, got %d", *r.Weight)
	}
	if *r.Port < 0 || *r.Port > 65535 {
		return fmt.Errorf("SRV record port must be 0-65535, got %d", *r.Port)
	}
	if !isValidHostname(r.Value) {
		return fmt.Errorf("SRV record target must be a valid hostname: %q", r.Value)
	}
	return nil
}

func validateCAA(r *domain.DNSRecord) error {
	// CAA format: "flag tag value", e.g. "0 issue letsencrypt.org"
	parts := strings.SplitN(r.Value, " ", 3)
	if len(parts) < 3 {
		return fmt.Errorf("CAA record value must be in format 'flag tag value': %q", r.Value)
	}
	// Flag should be 0 or 128.
	if parts[0] != "0" && parts[0] != "128" {
		return fmt.Errorf("CAA record flag must be 0 or 128: %q", parts[0])
	}
	// Tag should be one of: issue, issuewild, iodef.
	tag := strings.ToLower(parts[1])
	if tag != "issue" && tag != "issuewild" && tag != "iodef" {
		return fmt.Errorf("CAA record tag must be issue, issuewild, or iodef: %q", parts[1])
	}
	return nil
}

func validatePTR(r *domain.DNSRecord) error {
	if !isValidHostname(r.Value) {
		return fmt.Errorf("PTR record value must be a valid hostname: %q", r.Value)
	}
	return nil
}

// isValidHostname checks if a string is a valid DNS hostname.
func isValidHostname(s string) bool {
	if s == "" {
		return false
	}
	clean := strings.TrimSuffix(s, ".")
	if len(clean) > 253 {
		return false
	}
	return hostnameRegex.MatchString(clean)
}

// validateTransferAllowedIPs validates that each entry is a valid IP or CIDR.
func validateTransferAllowedIPs(ips []string) error {
	for _, entry := range ips {
		if ip := net.ParseIP(entry); ip != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err == nil {
			continue
		}
		return fmt.Errorf("invalid IP or CIDR in transfer_allowed_ips: %q", entry)
	}
	return nil
}

// checkCNAMEExclusivity checks that a CNAME record doesn't conflict with
// existing records at the same name. CNAME records must be the only record
// at a given name.
func checkCNAMEExclusivity(existingRecords []*domain.DNSRecord, newRecord *domain.DNSRecord) error {
	// If the new record is a CNAME, ensure no other records exist at this name.
	if newRecord.Type == domain.DNSRecordTypeCNAME {
		for _, r := range existingRecords {
			if r.Name == newRecord.Name && r.ID != newRecord.ID {
				return fmt.Errorf("CNAME record conflicts with existing %s record at name %q", r.Type, newRecord.Name)
			}
		}
	}

	// If an existing CNAME exists at this name, no other records can be added.
	for _, r := range existingRecords {
		if r.Name == newRecord.Name && r.Type == domain.DNSRecordTypeCNAME && r.ID != newRecord.ID {
			return fmt.Errorf("cannot add %s record at name %q: CNAME record already exists", newRecord.Type, newRecord.Name)
		}
	}

	return nil
}
