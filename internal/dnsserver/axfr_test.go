package dnsserver

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// startAXFRServer creates a miekg service with AXFR enabled.
func startAXFRServer(t *testing.T) (*MiekgService, string) {
	t.Helper()
	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	svc := NewMiekgService(nil, nil, true)
	if err := svc.Start(context.Background(), addr); err != nil {
		t.Fatalf("start server: %v", err)
	}

	t.Cleanup(func() {
		svc.Stop(context.Background())
	})

	return svc, addr
}

// reconcileTestZone loads a zone with records and allowed transfer IPs.
func reconcileTestZone(t *testing.T, svc *MiekgService, allowedIPs []string) {
	t.Helper()
	zone := &domain.DNSZone{
		ID:                 domain.NewID(),
		Name:               "example.com.",
		Serial:             2024010101,
		TTL:                300,
		Status:             domain.DNSZoneActive,
		TransferAllowedIPs: allowedIPs,
	}
	records := []*domain.DNSRecord{
		{ID: domain.NewID(), ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeA, Value: "1.2.3.4"},
		{ID: domain.NewID(), ZoneID: zone.ID, Name: "www", Type: domain.DNSRecordTypeA, Value: "5.6.7.8"},
		{ID: domain.NewID(), ZoneID: zone.ID, Name: "@", Type: domain.DNSRecordTypeMX, Value: "mail.example.com", Priority: intPtr(10)},
	}

	err := svc.Reconcile(context.Background(), &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{{Zone: zone, Records: records}},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestAXFRTransfer(t *testing.T) {
	svc, addr := startAXFRServer(t)
	reconcileTestZone(t, svc, []string{"127.0.0.1"})

	// Perform AXFR transfer via TCP.
	tr := new(mdns.Transfer)
	m := new(mdns.Msg)
	m.SetAxfr("example.com.")

	ch, err := tr.In(m, addr)
	if err != nil {
		t.Fatalf("AXFR transfer: %v", err)
	}

	var allRRs []mdns.RR
	for env := range ch {
		if env.Error != nil {
			t.Fatalf("AXFR envelope error: %v", env.Error)
		}
		allRRs = append(allRRs, env.RR...)
	}

	// Should have at least: SOA + records + SOA (bookend).
	if len(allRRs) < 3 {
		t.Fatalf("expected at least 3 RRs, got %d", len(allRRs))
	}

	// First RR should be SOA.
	if _, ok := allRRs[0].(*mdns.SOA); !ok {
		t.Fatalf("first RR should be SOA, got %T", allRRs[0])
	}

	// Last RR should be SOA.
	if _, ok := allRRs[len(allRRs)-1].(*mdns.SOA); !ok {
		t.Fatalf("last RR should be SOA, got %T", allRRs[len(allRRs)-1])
	}

	// Check that we got our A records.
	var aCount int
	for _, rr := range allRRs {
		if _, ok := rr.(*mdns.A); ok {
			aCount++
		}
	}
	if aCount != 2 {
		t.Fatalf("expected 2 A records, got %d", aCount)
	}
}

func TestAXFRDenied(t *testing.T) {
	svc, addr := startAXFRServer(t)
	// Allow only 10.0.0.1 — our client is 127.0.0.1.
	reconcileTestZone(t, svc, []string{"10.0.0.1"})

	c := new(mdns.Client)
	c.Net = "tcp"
	m := new(mdns.Msg)
	m.SetQuestion("example.com.", mdns.TypeAXFR)

	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if r.Rcode != mdns.RcodeRefused {
		t.Fatalf("expected REFUSED, got %s", mdns.RcodeToString[r.Rcode])
	}
}

func TestAXFRDisabled(t *testing.T) {
	// Create server with AXFR disabled.
	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	svc := NewMiekgService(nil, nil, false)
	if err := svc.Start(context.Background(), addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { svc.Stop(context.Background()) })

	reconcileTestZone(t, svc, []string{"127.0.0.1"})

	c := new(mdns.Client)
	c.Net = "tcp"
	m := new(mdns.Msg)
	m.SetQuestion("example.com.", mdns.TypeAXFR)

	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if r.Rcode != mdns.RcodeRefused {
		t.Fatalf("expected REFUSED, got %s", mdns.RcodeToString[r.Rcode])
	}
}

func TestAXFREmptyACL(t *testing.T) {
	svc, addr := startAXFRServer(t)
	// Empty ACL = deny all.
	reconcileTestZone(t, svc, nil)

	c := new(mdns.Client)
	c.Net = "tcp"
	m := new(mdns.Msg)
	m.SetQuestion("example.com.", mdns.TypeAXFR)

	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if r.Rcode != mdns.RcodeRefused {
		t.Fatalf("expected REFUSED, got %s", mdns.RcodeToString[r.Rcode])
	}
}

func TestParseAllowedCIDRs(t *testing.T) {
	cidrs := parseAllowedCIDRs([]string{"192.168.1.0/24", "10.0.0.1", "2001:db8::1"})
	if len(cidrs) != 3 {
		t.Fatalf("expected 3 CIDRs, got %d", len(cidrs))
	}

	// Test matching.
	if !cidrs[0].Contains(net.ParseIP("192.168.1.50")) {
		t.Fatal("192.168.1.50 should match 192.168.1.0/24")
	}
	if cidrs[0].Contains(net.ParseIP("192.168.2.1")) {
		t.Fatal("192.168.2.1 should not match 192.168.1.0/24")
	}
	if !cidrs[1].Contains(net.ParseIP("10.0.0.1")) {
		t.Fatal("10.0.0.1 should match itself")
	}
	if cidrs[1].Contains(net.ParseIP("10.0.0.2")) {
		t.Fatal("10.0.0.2 should not match 10.0.0.1/32")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"127.0.0.1:53", "127.0.0.1"},
		{"[::1]:53", "::1"},
	}

	for _, tt := range tests {
		addr, _ := net.ResolveTCPAddr("tcp", tt.addr)
		ip := extractIP(addr)
		if ip == nil || ip.String() != tt.expected {
			t.Errorf("extractIP(%s) = %v, want %s", tt.addr, ip, tt.expected)
		}
	}

	// nil addr.
	if ip := extractIP(nil); ip != nil {
		t.Errorf("extractIP(nil) = %v, want nil", ip)
	}
}

// Suppress unused import warning for time.
var _ = time.Now
