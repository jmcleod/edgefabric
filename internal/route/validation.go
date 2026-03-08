package route

import (
	"fmt"
	"net"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// validateRoute validates a route's fields.
func validateRoute(r *domain.Route) error {
	// Name validation.
	if r.Name == "" {
		return fmt.Errorf("route name is required")
	}
	if len(r.Name) > 255 {
		return fmt.Errorf("route name too long (max 255 characters)")
	}

	// Protocol validation.
	switch r.Protocol {
	case domain.RouteProtocolTCP, domain.RouteProtocolUDP,
		domain.RouteProtocolICMP, domain.RouteProtocolAll:
	default:
		return fmt.Errorf("invalid protocol: %q (must be tcp, udp, icmp, or all)", r.Protocol)
	}

	// Entry IP validation.
	if net.ParseIP(r.EntryIP) == nil {
		return fmt.Errorf("invalid entry_ip: %q", r.EntryIP)
	}

	// Entry port validation (required for tcp/udp/all, forbidden for icmp).
	if r.Protocol == domain.RouteProtocolICMP {
		if r.EntryPort != nil {
			return fmt.Errorf("entry_port must not be set for icmp protocol")
		}
	} else {
		if r.EntryPort == nil {
			return fmt.Errorf("entry_port is required for %s protocol", r.Protocol)
		}
		if *r.EntryPort < 1 || *r.EntryPort > 65535 {
			return fmt.Errorf("entry_port must be 1-65535, got %d", *r.EntryPort)
		}
	}

	// Destination IP validation.
	if net.ParseIP(r.DestinationIP) == nil {
		return fmt.Errorf("invalid destination_ip: %q", r.DestinationIP)
	}

	// Destination port validation (same rules as entry port).
	if r.Protocol == domain.RouteProtocolICMP {
		if r.DestinationPort != nil {
			return fmt.Errorf("destination_port must not be set for icmp protocol")
		}
	} else {
		if r.DestinationPort == nil {
			return fmt.Errorf("destination_port is required for %s protocol", r.Protocol)
		}
		if *r.DestinationPort < 1 || *r.DestinationPort > 65535 {
			return fmt.Errorf("destination_port must be 1-65535, got %d", *r.DestinationPort)
		}
	}

	return nil
}
