package dnsserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// Ensure MiekgService implements Service at compile time.
var _ Service = (*MiekgService)(nil)

// recordKey is the lookup key for a DNS record set.
type recordKey struct {
	Name string // lowercase FQDN, e.g. "www.example.com."
	Type uint16 // mdns.TypeA, mdns.TypeAAAA, etc.
}

// zoneData holds the in-memory representation of a DNS zone.
type zoneData struct {
	name                 string // FQDN zone name, e.g. "example.com."
	tenantID             string // owning tenant ID for per-tenant metrics
	serial               uint32
	ttl                  uint32
	records              map[recordKey][]mdns.RR // pre-built resource records
	transferAllowedCIDRs []*net.IPNet            // AXFR ACL
}

// MiekgService is an authoritative DNS server backed by github.com/miekg/dns.
// Zone data is stored in memory and rebuilt on each Reconcile() call.
type MiekgService struct {
	mu         sync.RWMutex
	running    bool
	listenAddr string

	// zones maps lowercase zone name → zone data.
	zones map[string]*zoneData

	// sortedZones holds zone names sorted longest-first for matching.
	sortedZones []string

	udpServer *mdns.Server
	tcpServer *mdns.Server

	queriesTotal atomic.Uint64

	axfrEnabled bool

	logger  *slog.Logger
	metrics *observability.Metrics
}

// NewMiekgService creates a new miekg/dns authoritative DNS server.
func NewMiekgService(logger *slog.Logger, metrics *observability.Metrics, axfrEnabled bool) *MiekgService {
	return &MiekgService{
		zones:       make(map[string]*zoneData),
		logger:      logger,
		metrics:     metrics,
		axfrEnabled: axfrEnabled,
	}
}

func (s *MiekgService) Start(_ context.Context, listenAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("dns server already running")
	}

	// Create a dedicated mux so we don't pollute the global handler.
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.udpServer = &mdns.Server{
		Addr:    listenAddr,
		Net:     "udp",
		Handler: mux,
	}
	s.tcpServer = &mdns.Server{
		Addr:    listenAddr,
		Net:     "tcp",
		Handler: mux,
	}

	udpReady := make(chan struct{})
	tcpReady := make(chan struct{})

	s.udpServer.NotifyStartedFunc = func() { close(udpReady) }
	s.tcpServer.NotifyStartedFunc = func() { close(tcpReady) }

	errCh := make(chan error, 2)

	go func() {
		if err := s.udpServer.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("udp: %w", err)
		}
	}()

	go func() {
		if err := s.tcpServer.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("tcp: %w", err)
		}
	}()

	// Wait for both servers to be ready or fail.
	select {
	case <-udpReady:
	case err := <-errCh:
		return fmt.Errorf("start dns server: %w", err)
	}

	select {
	case <-tcpReady:
	case err := <-errCh:
		return fmt.Errorf("start dns server: %w", err)
	}

	s.running = true
	s.listenAddr = listenAddr
	return nil
}

func (s *MiekgService) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("dns server not running")
	}

	var errs []error
	if s.udpServer != nil {
		if err := s.udpServer.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown udp: %w", err))
		}
	}
	if s.tcpServer != nil {
		if err := s.tcpServer.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown tcp: %w", err))
		}
	}

	s.running = false
	s.zones = make(map[string]*zoneData)
	s.sortedZones = nil

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (s *MiekgService) Reconcile(_ context.Context, config *dns.NodeDNSConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("dns server not running")
	}

	newZones := make(map[string]*zoneData)
	var sortedNames []string

	if config != nil {
		for _, zwr := range config.Zones {
			zd := s.buildZoneData(zwr)
			newZones[strings.ToLower(zd.name)] = zd
			sortedNames = append(sortedNames, strings.ToLower(zd.name))
		}
	}

	// Sort longest-first for zone matching.
	sortByLengthDesc(sortedNames)

	s.zones = newZones
	s.sortedZones = sortedNames
	return nil
}

func (s *MiekgService) GetStatus(_ context.Context) (*ServerStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	serials := make(map[string]uint32, len(s.zones))
	for name, zd := range s.zones {
		serials[name] = zd.serial
	}

	return &ServerStatus{
		Listening:    s.running,
		ListenAddr:   s.listenAddr,
		ZoneCount:    len(s.zones),
		ZoneSerials:  serials,
		QueriesTotal: s.queriesTotal.Load(),
	}, nil
}

// handleQuery is the miekg/dns handler for all incoming queries.
func (s *MiekgService) handleQuery(w mdns.ResponseWriter, r *mdns.Msg) {
	start := time.Now()
	s.queriesTotal.Add(1)

	msg := new(mdns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true
	msg.RecursionAvailable = false

	if len(r.Question) == 0 {
		msg.Rcode = mdns.RcodeServerFailure
		w.WriteMsg(msg)
		s.recordQueryMetrics("unknown", "", "UNKNOWN", msg.Rcode, start)
		return
	}

	q := r.Question[0]
	qname := strings.ToLower(q.Name)

	s.mu.RLock()
	zd := s.findZone(qname)
	s.mu.RUnlock()

	if zd == nil {
		// Not authoritative for this zone — REFUSED.
		msg.Rcode = mdns.RcodeRefused
		msg.Authoritative = false
		w.WriteMsg(msg)
		s.recordQueryMetrics("unknown", "", mdns.TypeToString[q.Qtype], msg.Rcode, start)
		return
	}

	zoneName := zd.name

	// Handle AXFR zone transfer requests.
	if q.Qtype == mdns.TypeAXFR {
		s.handleAXFR(w, r, zd, zoneName, start)
		return
	}

	// Handle SOA queries.
	if q.Qtype == mdns.TypeSOA {
		soa := s.buildSOA(zd)
		msg.Answer = append(msg.Answer, soa)
		w.WriteMsg(msg)
		s.recordQueryMetrics(zoneName, zd.tenantID, "SOA", msg.Rcode, start)
		return
	}

	// Handle NS queries for the zone apex.
	if q.Qtype == mdns.TypeNS && qname == strings.ToLower(zd.name) {
		key := recordKey{Name: qname, Type: mdns.TypeNS}
		s.mu.RLock()
		rrs := zd.records[key]
		s.mu.RUnlock()
		if len(rrs) > 0 {
			msg.Answer = append(msg.Answer, rrs...)
		} else {
			// Auto-generate NS if none configured.
			ns := &mdns.NS{
				Hdr: mdns.RR_Header{
					Name:   zd.name,
					Rrtype: mdns.TypeNS,
					Class:  mdns.ClassINET,
					Ttl:    zd.ttl,
				},
				Ns: "ns1." + zd.name,
			}
			msg.Answer = append(msg.Answer, ns)
		}
		w.WriteMsg(msg)
		s.recordQueryMetrics(zoneName, zd.tenantID, "NS", msg.Rcode, start)
		return
	}

	// Lookup records.
	s.mu.RLock()
	key := recordKey{Name: qname, Type: q.Qtype}
	rrs := zd.records[key]

	// If no direct match, check for CNAME.
	if len(rrs) == 0 && q.Qtype != mdns.TypeCNAME {
		cnameKey := recordKey{Name: qname, Type: mdns.TypeCNAME}
		cnameRRs := zd.records[cnameKey]
		if len(cnameRRs) > 0 {
			// CNAME following: include the CNAME, then look up the target.
			msg.Answer = append(msg.Answer, cnameRRs...)
			if cname, ok := cnameRRs[0].(*mdns.CNAME); ok {
				targetKey := recordKey{Name: strings.ToLower(cname.Target), Type: q.Qtype}
				targetRRs := zd.records[targetKey]
				msg.Answer = append(msg.Answer, targetRRs...)
			}
			s.mu.RUnlock()
			w.WriteMsg(msg)
			s.recordQueryMetrics(zoneName, zd.tenantID, mdns.TypeToString[q.Qtype], msg.Rcode, start)
			return
		}
	}
	s.mu.RUnlock()

	if len(rrs) > 0 {
		msg.Answer = append(msg.Answer, rrs...)
	} else {
		// Name exists in zone but no records of requested type → NOERROR with empty answer.
		// If name doesn't exist at all → NXDOMAIN.
		s.mu.RLock()
		exists := s.nameExistsInZone(zd, qname)
		s.mu.RUnlock()
		if !exists {
			msg.Rcode = mdns.RcodeNameError // NXDOMAIN
		}
		// Add SOA in authority section for negative responses.
		soa := s.buildSOA(zd)
		msg.Ns = append(msg.Ns, soa)
	}

	w.WriteMsg(msg)
	s.recordQueryMetrics(zoneName, zd.tenantID, mdns.TypeToString[q.Qtype], msg.Rcode, start)
}

// recordQueryMetrics records per-query Prometheus metrics and structured log output.
func (s *MiekgService) recordQueryMetrics(zone, tenantID, qtype string, rcode int, start time.Time) {
	duration := time.Since(start)
	rcodeStr := mdns.RcodeToString[rcode]
	if rcodeStr == "" {
		rcodeStr = fmt.Sprintf("%d", rcode)
	}

	if s.metrics != nil {
		s.metrics.DNSQueryDuration.WithLabelValues(zone, qtype, rcodeStr).Observe(duration.Seconds())
		s.metrics.DNSQueriesByZone.WithLabelValues(zone, qtype, rcodeStr).Inc()
		if tenantID != "" {
			s.metrics.TenantDNSQueries.WithLabelValues(tenantID, zone).Inc()
		}
	}

	if s.logger != nil {
		s.logger.Debug("dns query",
			slog.String("zone", zone),
			slog.String("qtype", qtype),
			slog.String("rcode", rcodeStr),
			slog.Duration("duration", duration),
		)
	}
}

// findZone finds the most specific zone for a query name.
// Must be called with s.mu held (at least RLock).
func (s *MiekgService) findZone(qname string) *zoneData {
	for _, zoneName := range s.sortedZones {
		if qname == zoneName || strings.HasSuffix(qname, "."+zoneName) {
			return s.zones[zoneName]
		}
	}
	return nil
}

// nameExistsInZone checks if any record exists at the given name.
// Must be called with s.mu held (at least RLock).
func (s *MiekgService) nameExistsInZone(zd *zoneData, qname string) bool {
	for key := range zd.records {
		if key.Name == qname {
			return true
		}
	}
	return false
}

// buildSOA creates a synthetic SOA record for negative responses and SOA queries.
func (s *MiekgService) buildSOA(zd *zoneData) *mdns.SOA {
	return &mdns.SOA{
		Hdr: mdns.RR_Header{
			Name:   zd.name,
			Rrtype: mdns.TypeSOA,
			Class:  mdns.ClassINET,
			Ttl:    zd.ttl,
		},
		Ns:      "ns1." + zd.name,
		Mbox:    "hostmaster." + zd.name,
		Serial:  zd.serial,
		Refresh: 3600,
		Retry:   900,
		Expire:  604800,
		Minttl:  300,
	}
}

// buildZoneData converts a ZoneWithRecords into an in-memory zone for serving.
func (s *MiekgService) buildZoneData(zwr dns.ZoneWithRecords) *zoneData {
	zone := zwr.Zone
	ttl := uint32(zone.TTL)
	if ttl == 0 {
		ttl = 300 // default TTL
	}

	zd := &zoneData{
		name:                 zone.Name,
		tenantID:             zone.TenantID.String(),
		serial:               zone.Serial,
		ttl:                  ttl,
		records:              make(map[recordKey][]mdns.RR),
		transferAllowedCIDRs: parseAllowedCIDRs(zone.TransferAllowedIPs),
	}

	for _, rec := range zwr.Records {
		rrs := s.convertRecord(rec, zone.Name, ttl)
		for _, rr := range rrs {
			key := recordKey{
				Name: strings.ToLower(rr.Header().Name),
				Type: rr.Header().Rrtype,
			}
			zd.records[key] = append(zd.records[key], rr)
		}
	}

	return zd
}

// convertRecord converts a domain.DNSRecord to miekg/dns RR(s).
func (s *MiekgService) convertRecord(rec *domain.DNSRecord, zoneName string, defaultTTL uint32) []mdns.RR {
	// Build FQDN from record name + zone name.
	var fqdn string
	if rec.Name == "@" || rec.Name == "" {
		fqdn = zoneName
	} else {
		fqdn = rec.Name + "." + zoneName
	}
	fqdn = mdns.Fqdn(fqdn)

	ttl := defaultTTL
	if rec.TTL != nil && *rec.TTL > 0 {
		ttl = uint32(*rec.TTL)
	}

	hdr := mdns.RR_Header{
		Name:   fqdn,
		Class:  mdns.ClassINET,
		Ttl:    ttl,
	}

	switch rec.Type {
	case domain.DNSRecordTypeA:
		ip := net.ParseIP(rec.Value)
		if ip == nil {
			return nil
		}
		hdr.Rrtype = mdns.TypeA
		return []mdns.RR{&mdns.A{Hdr: hdr, A: ip.To4()}}

	case domain.DNSRecordTypeAAAA:
		ip := net.ParseIP(rec.Value)
		if ip == nil {
			return nil
		}
		hdr.Rrtype = mdns.TypeAAAA
		return []mdns.RR{&mdns.AAAA{Hdr: hdr, AAAA: ip.To16()}}

	case domain.DNSRecordTypeCNAME:
		hdr.Rrtype = mdns.TypeCNAME
		return []mdns.RR{&mdns.CNAME{Hdr: hdr, Target: mdns.Fqdn(rec.Value)}}

	case domain.DNSRecordTypeMX:
		priority := uint16(10)
		if rec.Priority != nil {
			priority = uint16(*rec.Priority)
		}
		hdr.Rrtype = mdns.TypeMX
		return []mdns.RR{&mdns.MX{Hdr: hdr, Preference: priority, Mx: mdns.Fqdn(rec.Value)}}

	case domain.DNSRecordTypeTXT:
		hdr.Rrtype = mdns.TypeTXT
		return []mdns.RR{&mdns.TXT{Hdr: hdr, Txt: []string{rec.Value}}}

	case domain.DNSRecordTypeNS:
		hdr.Rrtype = mdns.TypeNS
		return []mdns.RR{&mdns.NS{Hdr: hdr, Ns: mdns.Fqdn(rec.Value)}}

	case domain.DNSRecordTypeSRV:
		priority := uint16(0)
		weight := uint16(0)
		port := uint16(0)
		if rec.Priority != nil {
			priority = uint16(*rec.Priority)
		}
		if rec.Weight != nil {
			weight = uint16(*rec.Weight)
		}
		if rec.Port != nil {
			port = uint16(*rec.Port)
		}
		hdr.Rrtype = mdns.TypeSRV
		return []mdns.RR{&mdns.SRV{
			Hdr:      hdr,
			Priority: priority,
			Weight:   weight,
			Port:     port,
			Target:   mdns.Fqdn(rec.Value),
		}}

	case domain.DNSRecordTypeCAA:
		hdr.Rrtype = mdns.TypeCAA
		// CAA format: "flag tag value" e.g. "0 issue letsencrypt.org"
		parts := strings.SplitN(rec.Value, " ", 3)
		if len(parts) != 3 {
			return nil
		}
		var flag uint8
		if parts[0] == "128" {
			flag = 128
		}
		return []mdns.RR{&mdns.CAA{Hdr: hdr, Flag: flag, Tag: parts[1], Value: parts[2]}}

	case domain.DNSRecordTypePTR:
		hdr.Rrtype = mdns.TypePTR
		return []mdns.RR{&mdns.PTR{Hdr: hdr, Ptr: mdns.Fqdn(rec.Value)}}

	default:
		return nil
	}
}

// sortByLengthDesc sorts strings by length descending (longest first).
func sortByLengthDesc(s []string) {
	// Simple insertion sort — zone count is small.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j]) > len(s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
