package dns

import (
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestValidateZoneName(t *testing.T) {
	tests := []struct {
		name    string
		zone    string
		wantErr bool
	}{
		{"valid simple", "example.com", false},
		{"valid with trailing dot", "example.com.", false},
		{"valid subdomain", "sub.example.com", false},
		{"valid hyphen", "my-domain.example.com", false},
		{"empty", "", true},
		{"just dot", ".", true},
		{"starts with hyphen", "-example.com", true},
		{"invalid chars", "exam ple.com", true},
		{"too long", string(make([]byte, 254)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateZoneName(tt.zone)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateZoneName(%q) = %v, wantErr %v", tt.zone, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_A(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv4", "203.0.113.10", false},
		{"valid IPv4 alt", "1.2.3.4", false},
		{"IPv6 not allowed", "2001:db8::1", true},
		{"invalid", "not-an-ip", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "www", Type: domain.DNSRecordTypeA, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(A, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_AAAA(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid IPv6", "2001:db8::1", false},
		{"valid IPv6 full", "2001:0db8:0000:0000:0000:0000:0000:0001", false},
		{"IPv4 not allowed", "203.0.113.10", true},
		{"invalid", "not-an-ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "www", Type: domain.DNSRecordTypeAAAA, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(AAAA, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_CNAME(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid hostname", "other.example.com", false},
		{"valid with trailing dot", "other.example.com.", false},
		{"invalid", "not a hostname!", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "alias", Type: domain.DNSRecordTypeCNAME, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(CNAME, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_MX(t *testing.T) {
	p10 := 10
	pNeg := -1

	tests := []struct {
		name     string
		value    string
		priority *int
		wantErr  bool
	}{
		{"valid", "mail.example.com", &p10, false},
		{"missing priority", "mail.example.com", nil, true},
		{"negative priority", "mail.example.com", &pNeg, true},
		{"invalid hostname", "not a host", &p10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "@", Type: domain.DNSRecordTypeMX, Value: tt.value, Priority: tt.priority}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(MX) = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_SRV(t *testing.T) {
	p := 10
	w := 60
	port := 5060

	tests := []struct {
		name     string
		priority *int
		weight   *int
		port     *int
		wantErr  bool
	}{
		{"valid", &p, &w, &port, false},
		{"missing priority", nil, &w, &port, true},
		{"missing weight", &p, nil, &port, true},
		{"missing port", &p, &w, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{
				Name: "_sip._tcp", Type: domain.DNSRecordTypeSRV, Value: "sip.example.com",
				Priority: tt.priority, Weight: tt.weight, Port: tt.port,
			}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(SRV) = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_TXT(t *testing.T) {
	r := &domain.DNSRecord{Name: "@", Type: domain.DNSRecordTypeTXT, Value: "v=spf1 include:example.com ~all"}
	if err := validateRecord(r); err != nil {
		t.Errorf("validateRecord(TXT) = %v, want nil", err)
	}
}

func TestValidateRecord_NS(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "ns1.example.com", false},
		{"invalid", "not a host!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "@", Type: domain.DNSRecordTypeNS, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(NS, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_CAA(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid issue", "0 issue letsencrypt.org", false},
		{"valid issuewild", "0 issuewild letsencrypt.org", false},
		{"valid iodef", "0 iodef mailto:admin@example.com", false},
		{"valid critical", "128 issue ca.example.com", false},
		{"bad flag", "5 issue ca.example.com", true},
		{"bad tag", "0 badtag ca.example.com", true},
		{"too few parts", "0 issue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "@", Type: domain.DNSRecordTypeCAA, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(CAA, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_PTR(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "host.example.com", false},
		{"invalid", "not a host!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.DNSRecord{Name: "10.113.0.203.in-addr.arpa", Type: domain.DNSRecordTypePTR, Value: tt.value}
			err := validateRecord(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRecord(PTR, %q) = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecord_UnsupportedType(t *testing.T) {
	r := &domain.DNSRecord{Name: "www", Type: "UNKNOWN", Value: "test"}
	err := validateRecord(r)
	if err == nil {
		t.Error("expected error for unsupported record type")
	}
}

func TestCheckCNAMEExclusivity(t *testing.T) {
	id1 := domain.NewID()
	id2 := domain.NewID()

	t.Run("CNAME conflicts with existing A", func(t *testing.T) {
		existing := []*domain.DNSRecord{
			{ID: id1, Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"},
		}
		newRec := &domain.DNSRecord{ID: id2, Name: "www", Type: domain.DNSRecordTypeCNAME, Value: "other.example.com"}
		if err := checkCNAMEExclusivity(existing, newRec); err == nil {
			t.Error("expected error: CNAME conflicts with existing A record")
		}
	})

	t.Run("A conflicts with existing CNAME", func(t *testing.T) {
		existing := []*domain.DNSRecord{
			{ID: id1, Name: "www", Type: domain.DNSRecordTypeCNAME, Value: "other.example.com"},
		}
		newRec := &domain.DNSRecord{ID: id2, Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"}
		if err := checkCNAMEExclusivity(existing, newRec); err == nil {
			t.Error("expected error: A record conflicts with existing CNAME")
		}
	})

	t.Run("A at different name is OK", func(t *testing.T) {
		existing := []*domain.DNSRecord{
			{ID: id1, Name: "www", Type: domain.DNSRecordTypeCNAME, Value: "other.example.com"},
		}
		newRec := &domain.DNSRecord{ID: id2, Name: "mail", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"}
		if err := checkCNAMEExclusivity(existing, newRec); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Multiple A records are OK", func(t *testing.T) {
		existing := []*domain.DNSRecord{
			{ID: id1, Name: "www", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"},
		}
		newRec := &domain.DNSRecord{ID: id2, Name: "www", Type: domain.DNSRecordTypeA, Value: "5.6.7.8"}
		if err := checkCNAMEExclusivity(existing, newRec); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Update same CNAME record is OK", func(t *testing.T) {
		existing := []*domain.DNSRecord{
			{ID: id1, Name: "www", Type: domain.DNSRecordTypeCNAME, Value: "old.example.com"},
		}
		// Same ID means it's an update to the same record.
		newRec := &domain.DNSRecord{ID: id1, Name: "www", Type: domain.DNSRecordTypeCNAME, Value: "new.example.com"}
		if err := checkCNAMEExclusivity(existing, newRec); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
