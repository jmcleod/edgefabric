package route

import (
	"strings"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
)

func intPtr(v int) *int {
	return &v
}

func TestValidateRoute(t *testing.T) {
	goodTCP := &domain.Route{
		Name: "web", Protocol: domain.RouteProtocolTCP,
		EntryIP: "198.51.100.1", EntryPort: intPtr(443),
		GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(8443),
	}
	goodUDP := &domain.Route{
		Name: "dns-fwd", Protocol: domain.RouteProtocolUDP,
		EntryIP: "198.51.100.1", EntryPort: intPtr(53),
		GatewayID: domain.NewID(), DestinationIP: "10.0.1.2", DestinationPort: intPtr(53),
	}
	goodICMP := &domain.Route{
		Name: "ping", Protocol: domain.RouteProtocolICMP,
		EntryIP: "198.51.100.1",
		GatewayID: domain.NewID(), DestinationIP: "10.0.1.3",
	}
	goodAll := &domain.Route{
		Name: "all-traffic", Protocol: domain.RouteProtocolAll,
		EntryIP: "198.51.100.1", EntryPort: intPtr(80),
		GatewayID: domain.NewID(), DestinationIP: "10.0.1.4", DestinationPort: intPtr(80),
	}

	// Good routes should pass.
	for _, r := range []*domain.Route{goodTCP, goodUDP, goodICMP, goodAll} {
		if err := validateRoute(r); err != nil {
			t.Errorf("expected %s route to be valid, got: %v", r.Protocol, err)
		}
	}

	// Bad cases.
	tests := []struct {
		name    string
		route   *domain.Route
		wantErr string
	}{
		{
			name: "empty name",
			route: &domain.Route{
				Name: "", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "route name is required",
		},
		{
			name: "name too long",
			route: &domain.Route{
				Name: strings.Repeat("a", 256), Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "route name too long",
		},
		{
			name: "bad protocol",
			route: &domain.Route{
				Name: "test", Protocol: "http",
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "invalid protocol",
		},
		{
			name: "invalid entry IP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolTCP,
				EntryIP: "not-an-ip", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "invalid entry_ip",
		},
		{
			name: "missing port for TCP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1",
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "entry_port is required for tcp",
		},
		{
			name: "port set for ICMP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolICMP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1",
			},
			wantErr: "entry_port must not be set for icmp",
		},
		{
			name: "port out of range",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(70000),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "entry_port must be 1-65535",
		},
		{
			name: "invalid destination IP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "bad-ip", DestinationPort: intPtr(80),
			},
			wantErr: "invalid destination_ip",
		},
		{
			name: "missing destination port for UDP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolUDP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(53),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1",
			},
			wantErr: "destination_port is required for udp",
		},
		{
			name: "destination port for ICMP",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolICMP,
				EntryIP: "198.51.100.1",
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(80),
			},
			wantErr: "destination_port must not be set for icmp",
		},
		{
			name: "destination port out of range",
			route: &domain.Route{
				Name: "test", Protocol: domain.RouteProtocolTCP,
				EntryIP: "198.51.100.1", EntryPort: intPtr(80),
				GatewayID: domain.NewID(), DestinationIP: "10.0.1.1", DestinationPort: intPtr(0),
			},
			wantErr: "destination_port must be 1-65535",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRoute(tc.route)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
