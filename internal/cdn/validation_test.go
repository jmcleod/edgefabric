package cdn

import (
	"encoding/json"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestValidateSite_Good(t *testing.T) {
	rps := 100
	site := &domain.CDNSite{
		Name:               "my-site",
		Domains:            []string{"cdn.example.com", "www.example.com"},
		TLSMode:            domain.TLSModeAuto,
		CacheEnabled:       true,
		CacheTTL:           3600,
		CompressionEnabled: true,
		RateLimitRPS:       &rps,
		HeaderRules:        json.RawMessage(`[{"action":"set","header":"X-CDN","value":"edgefabric"}]`),
	}
	if err := validateSite(site); err != nil {
		t.Errorf("expected valid site, got error: %v", err)
	}
}

func TestValidateSite_Bad(t *testing.T) {
	tests := []struct {
		name string
		site *domain.CDNSite
	}{
		{
			name: "empty name",
			site: &domain.CDNSite{
				Name:    "",
				Domains: []string{"example.com"},
				TLSMode: domain.TLSModeAuto,
			},
		},
		{
			name: "no domains",
			site: &domain.CDNSite{
				Name:    "site",
				Domains: []string{},
				TLSMode: domain.TLSModeAuto,
			},
		},
		{
			name: "invalid domain",
			site: &domain.CDNSite{
				Name:    "site",
				Domains: []string{"not a valid domain!"},
				TLSMode: domain.TLSModeAuto,
			},
		},
		{
			name: "invalid TLS mode",
			site: &domain.CDNSite{
				Name:    "site",
				Domains: []string{"example.com"},
				TLSMode: "invalid",
			},
		},
		{
			name: "empty TLS mode",
			site: &domain.CDNSite{
				Name:    "site",
				Domains: []string{"example.com"},
				TLSMode: "",
			},
		},
		{
			name: "negative cache TTL",
			site: &domain.CDNSite{
				Name:     "site",
				Domains:  []string{"example.com"},
				TLSMode:  domain.TLSModeAuto,
				CacheTTL: -1,
			},
		},
		{
			name: "zero rate limit",
			site: func() *domain.CDNSite {
				rps := 0
				return &domain.CDNSite{
					Name:         "site",
					Domains:      []string{"example.com"},
					TLSMode:      domain.TLSModeAuto,
					RateLimitRPS: &rps,
				}
			}(),
		},
		{
			name: "negative rate limit",
			site: func() *domain.CDNSite {
				rps := -5
				return &domain.CDNSite{
					Name:         "site",
					Domains:      []string{"example.com"},
					TLSMode:      domain.TLSModeAuto,
					RateLimitRPS: &rps,
				}
			}(),
		},
		{
			name: "invalid header rules (not JSON)",
			site: &domain.CDNSite{
				Name:        "site",
				Domains:     []string{"example.com"},
				TLSMode:     domain.TLSModeAuto,
				HeaderRules: json.RawMessage(`not json`),
			},
		},
		{
			name: "invalid header rules (bad action)",
			site: &domain.CDNSite{
				Name:        "site",
				Domains:     []string{"example.com"},
				TLSMode:     domain.TLSModeAuto,
				HeaderRules: json.RawMessage(`[{"action":"delete","header":"X-Foo"}]`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSite(tc.site); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestValidateOrigin_Good(t *testing.T) {
	interval := 30
	origin := &domain.CDNOrigin{
		Address:             "origin.example.com:443",
		Scheme:              domain.CDNOriginHTTPS,
		Weight:              10,
		HealthCheckPath:     "/healthz",
		HealthCheckInterval: &interval,
	}
	if err := validateOrigin(origin); err != nil {
		t.Errorf("expected valid origin, got error: %v", err)
	}
}

func TestValidateOrigin_Bad(t *testing.T) {
	tests := []struct {
		name   string
		origin *domain.CDNOrigin
	}{
		{
			name: "empty address",
			origin: &domain.CDNOrigin{
				Address: "",
				Scheme:  domain.CDNOriginHTTPS,
				Weight:  1,
			},
		},
		{
			name: "empty scheme",
			origin: &domain.CDNOrigin{
				Address: "origin.example.com",
				Scheme:  "",
				Weight:  1,
			},
		},
		{
			name: "invalid scheme",
			origin: &domain.CDNOrigin{
				Address: "origin.example.com",
				Scheme:  "ftp",
				Weight:  1,
			},
		},
		{
			name: "zero weight",
			origin: &domain.CDNOrigin{
				Address: "origin.example.com",
				Scheme:  domain.CDNOriginHTTPS,
				Weight:  0,
			},
		},
		{
			name: "negative weight",
			origin: &domain.CDNOrigin{
				Address: "origin.example.com",
				Scheme:  domain.CDNOriginHTTPS,
				Weight:  -1,
			},
		},
		{
			name: "health check path without slash",
			origin: &domain.CDNOrigin{
				Address:         "origin.example.com",
				Scheme:          domain.CDNOriginHTTPS,
				Weight:          1,
				HealthCheckPath: "healthz",
			},
		},
		{
			name: "health check interval too low",
			origin: func() *domain.CDNOrigin {
				interval := 2
				return &domain.CDNOrigin{
					Address:             "origin.example.com",
					Scheme:              domain.CDNOriginHTTPS,
					Weight:              1,
					HealthCheckInterval: &interval,
				}
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateOrigin(tc.origin); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestValidateHeaderRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     json.RawMessage
		wantErr bool
	}{
		{
			name:    "valid set rule",
			raw:     json.RawMessage(`[{"action":"set","header":"X-CDN","value":"edgefabric"}]`),
			wantErr: false,
		},
		{
			name:    "valid add rule",
			raw:     json.RawMessage(`[{"action":"add","header":"X-Via","value":"edge"}]`),
			wantErr: false,
		},
		{
			name:    "valid remove rule",
			raw:     json.RawMessage(`[{"action":"remove","header":"Server"}]`),
			wantErr: false,
		},
		{
			name:    "multiple valid rules",
			raw:     json.RawMessage(`[{"action":"set","header":"X-CDN","value":"ef"},{"action":"remove","header":"Server"}]`),
			wantErr: false,
		},
		{
			name:    "empty array",
			raw:     json.RawMessage(`[]`),
			wantErr: false,
		},
		{
			name:    "not JSON",
			raw:     json.RawMessage(`not json`),
			wantErr: true,
		},
		{
			name:    "not an array",
			raw:     json.RawMessage(`{"action":"set"}`),
			wantErr: true,
		},
		{
			name:    "missing header",
			raw:     json.RawMessage(`[{"action":"set","value":"foo"}]`),
			wantErr: true,
		},
		{
			name:    "invalid action",
			raw:     json.RawMessage(`[{"action":"delete","header":"X-Foo"}]`),
			wantErr: true,
		},
		{
			name:    "set without value",
			raw:     json.RawMessage(`[{"action":"set","header":"X-Foo"}]`),
			wantErr: true,
		},
		{
			name:    "add without value",
			raw:     json.RawMessage(`[{"action":"add","header":"X-Foo"}]`),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHeaderRules(tc.raw)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	goodDomains := []string{
		"example.com",
		"cdn.example.com",
		"www.my-site.example.com",
		"a.b.c.d.example.com",
		"example.com.",
	}
	for _, d := range goodDomains {
		if err := validateDomain(d); err != nil {
			t.Errorf("expected domain %q to be valid, got error: %v", d, err)
		}
	}

	badDomains := []string{
		"",
		"-invalid.com",
		"invalid-.com",
		"not a domain",
		"has space.com",
	}
	for _, d := range badDomains {
		if err := validateDomain(d); err == nil {
			t.Errorf("expected domain %q to be invalid, got nil", d)
		}
	}
}
