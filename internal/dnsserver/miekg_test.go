package dnsserver

import (
	"context"
	"fmt"
	"net"
	"testing"

	mdns "github.com/miekg/dns"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
)

// getFreePort returns an available localhost port.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("get free port: %v", err)
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
	l.Close()
	return port
}

// startTestServer creates, starts, and returns a MiekgService on a random port.
func startTestServer(t *testing.T) (*MiekgService, string) {
	t.Helper()
	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	svc := NewMiekgService()
	if err := svc.Start(context.Background(), addr); err != nil {
		t.Fatalf("start server: %v", err)
	}

	t.Cleanup(func() {
		svc.Stop(context.Background())
	})

	return svc, addr
}

// query sends a DNS query and returns the response.
func query(t *testing.T, addr string, name string, qtype uint16) *mdns.Msg {
	t.Helper()
	c := new(mdns.Client)
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(name), qtype)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("query %s %s: %v", name, mdns.TypeToString[qtype], err)
	}
	return r
}

func intPtr(v int) *int { return &v }

func TestMiekgStartStop(t *testing.T) {
	svc, _ := startTestServer(t)
	ctx := context.Background()

	// Double start should fail.
	if err := svc.Start(ctx, "127.0.0.1:0"); err == nil {
		t.Error("expected error on double start")
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if !status.Listening {
		t.Error("expected listening")
	}
	if status.ZoneCount != 0 {
		t.Errorf("expected 0 zones, got %d", status.ZoneCount)
	}
}

func TestMiekgReconcileAndQuery(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 42,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.1"},
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.2"},
					{ID: domain.NewID(), Name: "ipv6", Type: domain.DNSRecordTypeAAAA, Value: "2001:db8::1"},
					{ID: domain.NewID(), Name: "mail", Type: domain.DNSRecordTypeMX, Value: "mx1.example.com", Priority: intPtr(10)},
					{ID: domain.NewID(), Name: "mail", Type: domain.DNSRecordTypeMX, Value: "mx2.example.com", Priority: intPtr(20)},
					{ID: domain.NewID(), Name: "alias", Type: domain.DNSRecordTypeCNAME, Value: "www.example.com"},
					{ID: domain.NewID(), Name: "@", Type: domain.DNSRecordTypeTXT, Value: "v=spf1 include:_spf.example.com ~all"},
					{ID: domain.NewID(), Name: "_sip._tcp", Type: domain.DNSRecordTypeSRV, Value: "sipserver.example.com", Priority: intPtr(10), Weight: intPtr(60), Port: intPtr(5060)},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Test A record query.
	t.Run("A_record", func(t *testing.T) {
		r := query(t, addr, "www.example.com", mdns.TypeA)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		if len(r.Answer) != 2 {
			t.Fatalf("expected 2 A records, got %d", len(r.Answer))
		}
		if !r.Authoritative {
			t.Error("expected authoritative response")
		}
		ips := make(map[string]bool)
		for _, rr := range r.Answer {
			a, ok := rr.(*mdns.A)
			if !ok {
				t.Fatalf("expected *dns.A, got %T", rr)
			}
			ips[a.A.String()] = true
		}
		if !ips["192.0.2.1"] || !ips["192.0.2.2"] {
			t.Errorf("expected 192.0.2.1 and 192.0.2.2, got %v", ips)
		}
	})

	// Test AAAA record query.
	t.Run("AAAA_record", func(t *testing.T) {
		r := query(t, addr, "ipv6.example.com", mdns.TypeAAAA)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		if len(r.Answer) != 1 {
			t.Fatalf("expected 1 AAAA record, got %d", len(r.Answer))
		}
		aaaa := r.Answer[0].(*mdns.AAAA)
		if aaaa.AAAA.String() != "2001:db8::1" {
			t.Errorf("expected 2001:db8::1, got %s", aaaa.AAAA.String())
		}
	})

	// Test MX record query.
	t.Run("MX_record", func(t *testing.T) {
		r := query(t, addr, "mail.example.com", mdns.TypeMX)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		if len(r.Answer) != 2 {
			t.Fatalf("expected 2 MX records, got %d", len(r.Answer))
		}
		for _, rr := range r.Answer {
			mx, ok := rr.(*mdns.MX)
			if !ok {
				t.Fatalf("expected *dns.MX, got %T", rr)
			}
			if mx.Preference != 10 && mx.Preference != 20 {
				t.Errorf("unexpected MX preference %d", mx.Preference)
			}
		}
	})

	// Test CNAME following.
	t.Run("CNAME_following", func(t *testing.T) {
		r := query(t, addr, "alias.example.com", mdns.TypeA)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		// Should include CNAME + target A records.
		if len(r.Answer) < 1 {
			t.Fatal("expected at least 1 answer (CNAME)")
		}
		cname, ok := r.Answer[0].(*mdns.CNAME)
		if !ok {
			t.Fatalf("expected CNAME first, got %T", r.Answer[0])
		}
		if cname.Target != "www.example.com." {
			t.Errorf("expected CNAME target www.example.com., got %s", cname.Target)
		}
		// Should also have the 2 A records from following the CNAME.
		if len(r.Answer) != 3 {
			t.Errorf("expected 3 answers (1 CNAME + 2 A), got %d", len(r.Answer))
		}
	})

	// Test TXT record query.
	t.Run("TXT_record", func(t *testing.T) {
		r := query(t, addr, "example.com", mdns.TypeTXT)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		if len(r.Answer) != 1 {
			t.Fatalf("expected 1 TXT record, got %d", len(r.Answer))
		}
		txt := r.Answer[0].(*mdns.TXT)
		if len(txt.Txt) != 1 || txt.Txt[0] != "v=spf1 include:_spf.example.com ~all" {
			t.Errorf("unexpected TXT value: %v", txt.Txt)
		}
	})

	// Test SRV record query.
	t.Run("SRV_record", func(t *testing.T) {
		r := query(t, addr, "_sip._tcp.example.com", mdns.TypeSRV)
		if r.Rcode != mdns.RcodeSuccess {
			t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
		}
		if len(r.Answer) != 1 {
			t.Fatalf("expected 1 SRV record, got %d", len(r.Answer))
		}
		srv := r.Answer[0].(*mdns.SRV)
		if srv.Priority != 10 {
			t.Errorf("expected priority 10, got %d", srv.Priority)
		}
		if srv.Weight != 60 {
			t.Errorf("expected weight 60, got %d", srv.Weight)
		}
		if srv.Port != 5060 {
			t.Errorf("expected port 5060, got %d", srv.Port)
		}
		if srv.Target != "sipserver.example.com." {
			t.Errorf("expected target sipserver.example.com., got %s", srv.Target)
		}
	})
}

func TestMiekgSOAGeneration(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 99,
					TTL:    600,
					Status: domain.DNSZoneActive,
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Query SOA.
	r := query(t, addr, "example.com", mdns.TypeSOA)
	if r.Rcode != mdns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 SOA record, got %d", len(r.Answer))
	}

	soa, ok := r.Answer[0].(*mdns.SOA)
	if !ok {
		t.Fatalf("expected *dns.SOA, got %T", r.Answer[0])
	}
	if soa.Serial != 99 {
		t.Errorf("expected serial 99, got %d", soa.Serial)
	}
	if soa.Ns != "ns1.example.com." {
		t.Errorf("expected ns1.example.com., got %s", soa.Ns)
	}
	if soa.Mbox != "hostmaster.example.com." {
		t.Errorf("expected hostmaster.example.com., got %s", soa.Mbox)
	}
	if soa.Hdr.Ttl != 600 {
		t.Errorf("expected TTL 600, got %d", soa.Hdr.Ttl)
	}
}

func TestMiekgNXDOMAIN(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.1"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Query non-existent name within served zone.
	r := query(t, addr, "nonexistent.example.com", mdns.TypeA)
	if r.Rcode != mdns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got %s", mdns.RcodeToString[r.Rcode])
	}
	if !r.Authoritative {
		t.Error("expected authoritative NXDOMAIN")
	}
	// Should have SOA in authority section.
	if len(r.Ns) == 0 {
		t.Error("expected SOA in authority section for NXDOMAIN")
	}
}

func TestMiekgRefused(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Query a zone we don't serve.
	r := query(t, addr, "notserved.org", mdns.TypeA)
	if r.Rcode != mdns.RcodeRefused {
		t.Errorf("expected REFUSED, got %s", mdns.RcodeToString[r.Rcode])
	}
}

func TestMiekgQueryCount(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.1"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Make a few queries.
	query(t, addr, "www.example.com", mdns.TypeA)
	query(t, addr, "www.example.com", mdns.TypeA)
	query(t, addr, "example.com", mdns.TypeSOA)

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.QueriesTotal < 3 {
		t.Errorf("expected at least 3 queries, got %d", status.QueriesTotal)
	}
	if status.ZoneCount != 1 {
		t.Errorf("expected 1 zone, got %d", status.ZoneCount)
	}
	if status.ZoneSerials["example.com."] != 1 {
		t.Errorf("expected serial 1, got %d", status.ZoneSerials["example.com."])
	}
}

func TestMiekgReconcileUpdatesZones(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	// First reconcile with one zone.
	config1 := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "192.0.2.1"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config1); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}

	r := query(t, addr, "www.example.com", mdns.TypeA)
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 A record, got %d", len(r.Answer))
	}

	// Second reconcile: change IP, add a zone.
	config2 := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 2,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "www", Type: domain.DNSRecordTypeA, Value: "198.51.100.1"},
				},
			},
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.org.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "web", Type: domain.DNSRecordTypeA, Value: "203.0.113.1"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config2); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}

	// Verify updated IP.
	r = query(t, addr, "www.example.com", mdns.TypeA)
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 A record, got %d", len(r.Answer))
	}
	a := r.Answer[0].(*mdns.A)
	if a.A.String() != "198.51.100.1" {
		t.Errorf("expected 198.51.100.1, got %s", a.A.String())
	}

	// Verify new zone.
	r = query(t, addr, "web.example.org", mdns.TypeA)
	if r.Rcode != mdns.RcodeSuccess {
		t.Fatalf("expected NOERROR for new zone, got %s", mdns.RcodeToString[r.Rcode])
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 A record, got %d", len(r.Answer))
	}

	// Status should show 2 zones.
	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.ZoneCount != 2 {
		t.Errorf("expected 2 zones, got %d", status.ZoneCount)
	}
}

func TestMiekgCAARecord(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "@", Type: domain.DNSRecordTypeCAA, Value: "0 issue letsencrypt.org"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	r := query(t, addr, "example.com", mdns.TypeCAA)
	if r.Rcode != mdns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 CAA record, got %d", len(r.Answer))
	}
	caa := r.Answer[0].(*mdns.CAA)
	if caa.Tag != "issue" {
		t.Errorf("expected tag 'issue', got %q", caa.Tag)
	}
	if caa.Value != "letsencrypt.org" {
		t.Errorf("expected value 'letsencrypt.org', got %q", caa.Value)
	}
}

func TestMiekgPTRRecord(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "2.0.192.in-addr.arpa.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "1", Type: domain.DNSRecordTypePTR, Value: "www.example.com"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	r := query(t, addr, "1.2.0.192.in-addr.arpa", mdns.TypePTR)
	if r.Rcode != mdns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 PTR record, got %d", len(r.Answer))
	}
	ptr := r.Answer[0].(*mdns.PTR)
	if ptr.Ptr != "www.example.com." {
		t.Errorf("expected www.example.com., got %s", ptr.Ptr)
	}
}

func TestMiekgNSRecord(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "@", Type: domain.DNSRecordTypeNS, Value: "ns1.example.com"},
					{ID: domain.NewID(), Name: "@", Type: domain.DNSRecordTypeNS, Value: "ns2.example.com"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	r := query(t, addr, "example.com", mdns.TypeNS)
	if r.Rcode != mdns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", mdns.RcodeToString[r.Rcode])
	}
	if len(r.Answer) != 2 {
		t.Fatalf("expected 2 NS records, got %d", len(r.Answer))
	}
}

func TestMiekgRecordTTLOverride(t *testing.T) {
	svc, addr := startTestServer(t)
	ctx := context.Background()

	customTTL := 60
	config := &dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:     domain.NewID(),
					Name:   "example.com.",
					Serial: 1,
					TTL:    300,
					Status: domain.DNSZoneActive,
				},
				Records: []*domain.DNSRecord{
					{ID: domain.NewID(), Name: "fast", Type: domain.DNSRecordTypeA, Value: "192.0.2.1", TTL: &customTTL},
					{ID: domain.NewID(), Name: "default", Type: domain.DNSRecordTypeA, Value: "192.0.2.2"},
				},
			},
		},
	}

	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Custom TTL record.
	r := query(t, addr, "fast.example.com", mdns.TypeA)
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 record, got %d", len(r.Answer))
	}
	if r.Answer[0].Header().Ttl != 60 {
		t.Errorf("expected TTL 60, got %d", r.Answer[0].Header().Ttl)
	}

	// Default TTL record.
	r = query(t, addr, "default.example.com", mdns.TypeA)
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 record, got %d", len(r.Answer))
	}
	if r.Answer[0].Header().Ttl != 300 {
		t.Errorf("expected TTL 300, got %d", r.Answer[0].Header().Ttl)
	}
}
