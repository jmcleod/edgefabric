package dnsserver

import (
	"log/slog"
	"net"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// handleAXFR processes an AXFR zone transfer request.
func (s *MiekgService) handleAXFR(w mdns.ResponseWriter, r *mdns.Msg, zd *zoneData, zoneName string, start time.Time) {
	// Check if AXFR is globally enabled.
	if !s.axfrEnabled {
		s.refuseAXFR(w, r, zoneName, zd.tenantID, "disabled", start)
		return
	}

	// ACL check: empty ACL = deny all (secure default).
	if len(zd.transferAllowedCIDRs) == 0 {
		s.refuseAXFR(w, r, zoneName, zd.tenantID, "denied", start)
		return
	}

	clientIP := extractIP(w.RemoteAddr())
	if clientIP == nil || !isTransferAllowed(zd, clientIP) {
		s.refuseAXFR(w, r, zoneName, zd.tenantID, "denied", start)
		return
	}

	// Build the transfer: SOA → all RRs → SOA.
	soa := s.buildSOA(zd)
	ch := make(chan *mdns.Envelope, 4)

	go func() {
		defer close(ch)

		// First envelope: SOA.
		ch <- &mdns.Envelope{RR: []mdns.RR{soa}}

		// All zone records in batches.
		var batch []mdns.RR
		recordCount := 0
		for _, rrs := range zd.records {
			for _, rr := range rrs {
				batch = append(batch, rr)
				recordCount++
				if len(batch) >= 100 {
					ch <- &mdns.Envelope{RR: batch}
					batch = nil
				}
			}
		}
		if len(batch) > 0 {
			ch <- &mdns.Envelope{RR: batch}
		}

		// Final envelope: SOA (bookend).
		ch <- &mdns.Envelope{RR: []mdns.RR{soa}}

		// Record metrics.
		if s.metrics != nil {
			s.metrics.AXFRTransfersTotal.WithLabelValues(zoneName, "success").Inc()
			s.metrics.AXFRRecordsTransferred.Add(float64(recordCount))
		}
	}()

	t := new(mdns.Transfer)
	if err := t.Out(w, r, ch); err != nil {
		s.logger.Warn("AXFR transfer error",
			slog.String("zone", zoneName),
			slog.String("error", err.Error()),
		)
	}

	s.recordQueryMetrics(zoneName, zd.tenantID, "AXFR", mdns.RcodeSuccess, start)
}

// refuseAXFR sends a REFUSED response for an AXFR request.
func (s *MiekgService) refuseAXFR(w mdns.ResponseWriter, r *mdns.Msg, zoneName, tenantID, reason string, start time.Time) {
	msg := new(mdns.Msg)
	msg.SetReply(r)
	msg.Rcode = mdns.RcodeRefused
	w.WriteMsg(msg)

	if s.metrics != nil {
		s.metrics.AXFRTransfersTotal.WithLabelValues(zoneName, reason).Inc()
	}
	s.recordQueryMetrics(zoneName, tenantID, "AXFR", mdns.RcodeRefused, start)
}

// isTransferAllowed checks if a client IP is within the zone's allowed CIDRs.
func isTransferAllowed(zd *zoneData, clientIP net.IP) bool {
	for _, cidr := range zd.transferAllowedCIDRs {
		if cidr.Contains(clientIP) {
			return true
		}
	}
	return false
}

// parseAllowedCIDRs converts a list of IP/CIDR strings into []*net.IPNet.
// Plain IP addresses are converted to /32 (v4) or /128 (v6) CIDRs.
func parseAllowedCIDRs(ips []string) []*net.IPNet {
	var cidrs []*net.IPNet
	for _, entry := range ips {
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil {
				cidrs = append(cidrs, cidr)
			}
		} else {
			ip := net.ParseIP(entry)
			if ip == nil {
				continue
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			cidrs = append(cidrs, &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(bits, bits),
			})
		}
	}
	return cidrs
}

// extractIP parses the IP address from a net.Addr, stripping the port.
func extractIP(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		// Try parsing as plain IP.
		return net.ParseIP(addr.String())
	}
	return net.ParseIP(host)
}
