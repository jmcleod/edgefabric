package nodeclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

func envelope(t *testing.T, data any) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	out, err := json.Marshal(map[string]json.RawMessage{"data": raw})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return out
}

func TestNew(t *testing.T) {
	c := New("https://ctrl.example.com", "node-1", "tok-abc")
	if c.baseURL != "https://ctrl.example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://ctrl.example.com")
	}
	if c.nodeID != "node-1" {
		t.Errorf("nodeID = %q, want %q", c.nodeID, "node-1")
	}
	if c.apiToken != "tok-abc" {
		t.Errorf("apiToken = %q, want %q", c.apiToken, "tok-abc")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestFetchBGPConfig(t *testing.T) {
	nodeID := "node-42"
	id1 := uuid.New()
	id2 := uuid.New()
	nid := uuid.New()

	sessions := []*domain.BGPSession{
		{
			ID:                id1,
			NodeID:            nid,
			PeerASN:           65001,
			PeerAddress:       "10.0.0.1",
			LocalASN:          65000,
			Status:            domain.BGPSessionEstablished,
			AnnouncedPrefixes: []string{"192.168.1.0/24"},
		},
		{
			ID:          id2,
			NodeID:      nid,
			PeerASN:     65002,
			PeerAddress: "10.0.0.2",
			LocalASN:    65000,
			Status:      domain.BGPSessionConfigured,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes/"+nodeID+"/config/bgp" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(envelope(t, sessions))
	}))
	defer srv.Close()

	c := New(srv.URL, nodeID, "test-token")
	got, err := c.FetchBGPConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchBGPConfig: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(got))
	}
	if got[0].PeerASN != 65001 {
		t.Errorf("sessions[0].PeerASN = %d, want 65001", got[0].PeerASN)
	}
	if got[0].Status != domain.BGPSessionEstablished {
		t.Errorf("sessions[0].Status = %q, want %q", got[0].Status, domain.BGPSessionEstablished)
	}
	if got[1].PeerAddress != "10.0.0.2" {
		t.Errorf("sessions[1].PeerAddress = %q, want %q", got[1].PeerAddress, "10.0.0.2")
	}
	if len(got[0].AnnouncedPrefixes) != 1 || got[0].AnnouncedPrefixes[0] != "192.168.1.0/24" {
		t.Errorf("sessions[0].AnnouncedPrefixes = %v, want [192.168.1.0/24]", got[0].AnnouncedPrefixes)
	}
}

func TestFetchDNSConfig(t *testing.T) {
	nodeID := "node-dns"
	zoneID := uuid.New()
	tenantID := uuid.New()
	recID := uuid.New()

	cfg := dns.NodeDNSConfig{
		Zones: []dns.ZoneWithRecords{
			{
				Zone: &domain.DNSZone{
					ID:       zoneID,
					TenantID: tenantID,
					Name:     "example.com",
					Status:   "active",
					Serial:   2024010101,
					TTL:      3600,
				},
				Records: []*domain.DNSRecord{
					{
						ID:     recID,
						ZoneID: zoneID,
						Name:   "www",
						Type:   "A",
						Value:  "10.0.0.1",
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes/"+nodeID+"/config/dns" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(envelope(t, cfg))
	}))
	defer srv.Close()

	c := New(srv.URL, nodeID, "dns-token")
	got, err := c.FetchDNSConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchDNSConfig: %v", err)
	}
	if len(got.Zones) != 1 {
		t.Fatalf("len(zones) = %d, want 1", len(got.Zones))
	}
	if got.Zones[0].Zone.Name != "example.com" {
		t.Errorf("zone name = %q, want %q", got.Zones[0].Zone.Name, "example.com")
	}
	if len(got.Zones[0].Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(got.Zones[0].Records))
	}
	if got.Zones[0].Records[0].Value != "10.0.0.1" {
		t.Errorf("record value = %q, want %q", got.Zones[0].Records[0].Value, "10.0.0.1")
	}
}

func TestFetchCDNConfig(t *testing.T) {
	nodeID := "node-cdn"
	siteID := uuid.New()
	tenantID := uuid.New()
	originID := uuid.New()

	cfg := cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{
			{
				Site: &domain.CDNSite{
					ID:           siteID,
					TenantID:     tenantID,
					Name:         "my-cdn-site",
					Domains:      []string{"cdn.example.com"},
					CacheEnabled: true,
					CacheTTL:     300,
					WAFEnabled:   true,
					WAFMode:      "block",
					Status:       "active",
				},
				Origins: []*domain.CDNOrigin{
					{
						ID:              originID,
						SiteID:          siteID,
						Address:         "origin.example.com",
						Scheme:          "https",
						Weight:          100,
						HealthCheckPath: "/healthz",
						Status:          "healthy",
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes/"+nodeID+"/config/cdn" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(envelope(t, cfg))
	}))
	defer srv.Close()

	c := New(srv.URL, nodeID, "cdn-token")
	got, err := c.FetchCDNConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchCDNConfig: %v", err)
	}
	if len(got.Sites) != 1 {
		t.Fatalf("len(sites) = %d, want 1", len(got.Sites))
	}
	if got.Sites[0].Site.Name != "my-cdn-site" {
		t.Errorf("site name = %q, want %q", got.Sites[0].Site.Name, "my-cdn-site")
	}
	if !got.Sites[0].Site.WAFEnabled {
		t.Error("site WAFEnabled = false, want true")
	}
	if len(got.Sites[0].Origins) != 1 {
		t.Fatalf("len(origins) = %d, want 1", len(got.Sites[0].Origins))
	}
	if got.Sites[0].Origins[0].Address != "origin.example.com" {
		t.Errorf("origin address = %q, want %q", got.Sites[0].Origins[0].Address, "origin.example.com")
	}
}

func TestFetchRouteConfig(t *testing.T) {
	nodeID := "node-route"
	routeID := uuid.New()
	tenantID := uuid.New()
	gwID := uuid.New()
	port := 8080

	cfg := route.NodeRouteConfig{
		Routes: []route.RouteWithGateway{
			{
				Route: &domain.Route{
					ID:              routeID,
					TenantID:        tenantID,
					Name:            "web-route",
					Protocol:        "tcp",
					EntryIP:         "203.0.113.1",
					EntryPort:       &port,
					GatewayID:       gwID,
					DestinationIP:   "10.0.0.5",
					DestinationPort: &port,
					Status:          "active",
				},
				GatewayWGIP: "100.64.0.1",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes/"+nodeID+"/config/routes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(envelope(t, cfg))
	}))
	defer srv.Close()

	c := New(srv.URL, nodeID, "route-token")
	got, err := c.FetchRouteConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchRouteConfig: %v", err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("len(routes) = %d, want 1", len(got.Routes))
	}
	if got.Routes[0].Route.Name != "web-route" {
		t.Errorf("route name = %q, want %q", got.Routes[0].Route.Name, "web-route")
	}
	if got.Routes[0].GatewayWGIP != "100.64.0.1" {
		t.Errorf("gateway wg ip = %q, want %q", got.Routes[0].GatewayWGIP, "100.64.0.1")
	}
	if got.Routes[0].Route.EntryIP != "203.0.113.1" {
		t.Errorf("entry ip = %q, want %q", got.Routes[0].Route.EntryIP, "203.0.113.1")
	}
	if *got.Routes[0].Route.EntryPort != 8080 {
		t.Errorf("entry port = %d, want 8080", *got.Routes[0].Route.EntryPort)
	}
}

func TestFetchConfigHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := New(srv.URL, "node-err", "err-token")
	ctx := context.Background()

	_, err := c.FetchBGPConfig(ctx)
	if err == nil {
		t.Fatal("FetchBGPConfig: expected error for HTTP 500, got nil")
	}

	_, err = c.FetchDNSConfig(ctx)
	if err == nil {
		t.Fatal("FetchDNSConfig: expected error for HTTP 500, got nil")
	}

	_, err = c.FetchCDNConfig(ctx)
	if err == nil {
		t.Fatal("FetchCDNConfig: expected error for HTTP 500, got nil")
	}

	_, err = c.FetchRouteConfig(ctx)
	if err == nil {
		t.Fatal("FetchRouteConfig: expected error for HTTP 500, got nil")
	}
}

func TestEnroll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/enroll" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Token != "enroll-tok-123" {
			t.Errorf("token = %q, want %q", body.Token, "enroll-tok-123")
		}

		result := EnrollResult{
			Status:      "enrolled",
			NodeID:      "new-node-id",
			APIToken:    "new-api-token",
			WireGuardIP: "100.64.0.10",
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(envelope(t, result))
	}))
	defer srv.Close()

	got, err := Enroll(context.Background(), srv.URL, "enroll-tok-123")
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if got.Status != "enrolled" {
		t.Errorf("status = %q, want %q", got.Status, "enrolled")
	}
	if got.NodeID != "new-node-id" {
		t.Errorf("node_id = %q, want %q", got.NodeID, "new-node-id")
	}
	if got.APIToken != "new-api-token" {
		t.Errorf("api_token = %q, want %q", got.APIToken, "new-api-token")
	}
	if got.WireGuardIP != "100.64.0.10" {
		t.Errorf("wireguard_ip = %q, want %q", got.WireGuardIP, "100.64.0.10")
	}
}

func TestEnrollError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid enrollment token"}`))
	}))
	defer srv.Close()

	_, err := Enroll(context.Background(), srv.URL, "bad-token")
	if err == nil {
		t.Fatal("Enroll: expected error for HTTP 401, got nil")
	}
}
