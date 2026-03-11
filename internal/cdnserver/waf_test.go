package cdnserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/domain"
)

func TestWAFSQLiDetection(t *testing.T) {
	waf := NewWAF(WAFModeBlock, DefaultRules(), nil, nil)

	payloads := []struct {
		name  string
		path  string
		query string
	}{
		{"union-select", "/search", "q=1 UNION SELECT * FROM users"},
		{"tautology", "/login", "user=' OR '1'='1"},
		{"sleep", "/api", "id=1;sleep(5)"},
		{"comment", "/page", "id=1--"},
	}

	for _, p := range payloads {
		t.Run(p.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://example.com"+p.path+"?"+p.query, nil)
			match := waf.Inspect(r)
			if match == nil {
				t.Errorf("expected WAF match for %s payload", p.name)
			} else if match.Rule.Category != WAFCategorySQLi {
				t.Errorf("expected sqli category, got %s", match.Rule.Category)
			}
		})
	}
}

func TestWAFXSSDetection(t *testing.T) {
	waf := NewWAF(WAFModeBlock, DefaultRules(), nil, nil)

	payloads := []struct {
		name  string
		path  string
		query string
	}{
		{"script-tag", "/page", "q=<script>alert(1)</script>"},
		{"event-handler", "/page", "q=<img onerror=alert(1)>"},
		{"javascript-uri", "/page", "url=javascript:alert(1)"},
	}

	for _, p := range payloads {
		t.Run(p.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://example.com"+p.path+"?"+p.query, nil)
			match := waf.Inspect(r)
			if match == nil {
				t.Errorf("expected WAF match for %s payload", p.name)
			} else if match.Rule.Category != WAFCategoryXSS {
				t.Errorf("expected xss category, got %s", match.Rule.Category)
			}
		})
	}
}

func TestWAFPathTraversalDetection(t *testing.T) {
	waf := NewWAF(WAFModeBlock, DefaultRules(), nil, nil)

	payloads := []struct {
		name string
		path string
	}{
		{"dot-dot-slash", "/files/../../../etc/shadow"},
		{"etc-passwd", "/files/etc/passwd"},
	}

	for _, p := range payloads {
		t.Run(p.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://example.com"+p.path, nil)
			match := waf.Inspect(r)
			if match == nil {
				t.Errorf("expected WAF match for %s payload", p.name)
			} else if match.Rule.Category != WAFCategoryPathTraversal {
				t.Errorf("expected path_traversal category, got %s", match.Rule.Category)
			}
		})
	}
}

func TestWAFCleanRequests(t *testing.T) {
	waf := NewWAF(WAFModeBlock, DefaultRules(), nil, nil)

	requests := []struct {
		name  string
		path  string
		query string
	}{
		{"simple-get", "/index.html", ""},
		{"search-query", "/search", "q=hello+world"},
		{"api-call", "/api/v1/users", "page=1&limit=20"},
		{"static-asset", "/assets/style.css", "v=1234"},
		{"json-api", "/api/data", "filter=name&sort=asc"},
	}

	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			url := "http://example.com" + tc.path
			if tc.query != "" {
				url += "?" + tc.query
			}
			r, _ := http.NewRequest("GET", url, nil)
			r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			r.Header.Set("Referer", "https://example.com/page")
			match := waf.Inspect(r)
			if match != nil {
				t.Errorf("clean request triggered WAF: rule=%s field=%s", match.Rule.ID, match.Field)
			}
		})
	}
}

func TestWAFBlockModeRejects(t *testing.T) {
	svc := NewProxyService(nil, nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	siteID := domain.NewID()
	site := &domain.CDNSite{
		ID:                 siteID,
		Name:               "waf-block-test",
		Domains:            []string{"waf.example.com"},
		TLSMode:            domain.TLSModeDisabled,
		CompressionEnabled: false,
		WAFEnabled:         true,
		WAFMode:            "block",
		Status:             domain.CDNSiteActive,
	}

	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{{Site: site}},
	}
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Send SQLi payload.
	req, _ := http.NewRequest("GET", "http://"+addr+"/search?q=UNION+SELECT+*+FROM+users", nil)
	req.Host = "waf.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWAFDetectModeAllows(t *testing.T) {
	// Start an origin server.
	origin := httpTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("origin response"))
	})

	svc := NewProxyService(nil, nil)
	ctx := context.Background()

	addr := freePort(t)
	if err := svc.Start(ctx, addr); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(ctx)

	originHost, originPort, _ := net.SplitHostPort(origin.Listener.Addr().String())
	_ = originPort

	siteID := domain.NewID()
	site := &domain.CDNSite{
		ID:                 siteID,
		Name:               "waf-detect-test",
		Domains:            []string{"waf-detect.example.com"},
		TLSMode:            domain.TLSModeDisabled,
		CompressionEnabled: false,
		WAFEnabled:         true,
		WAFMode:            "detect",
		Status:             domain.CDNSiteActive,
	}

	originObj := &domain.CDNOrigin{
		ID:      domain.NewID(),
		SiteID:  siteID,
		Address: originHost + ":" + originPort,
		Scheme:  domain.CDNOriginHTTP,
		Weight:  1,
	}

	headerRules, _ := json.Marshal([]cdn.HeaderRule{})
	site.HeaderRules = headerRules

	config := &cdn.NodeCDNConfig{
		Sites: []cdn.SiteWithOrigins{{
			Site:    site,
			Origins: []*domain.CDNOrigin{originObj},
		}},
	}
	if err := svc.Reconcile(ctx, config); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Send SQLi payload — in detect mode, should still reach origin.
	req, _ := http.NewRequest("GET", "http://"+addr+"/search?q=UNION+SELECT+*+FROM+users", nil)
	req.Host = "waf-detect.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (detect mode should allow), got %d", resp.StatusCode)
	}
}

func TestWAFHeaderInspection(t *testing.T) {
	waf := NewWAF(WAFModeBlock, DefaultRules(), nil, nil)

	r, _ := http.NewRequest("GET", "http://example.com/page", nil)
	r.Header.Set("User-Agent", "<script>alert('xss')</script>")

	match := waf.Inspect(r)
	if match == nil {
		t.Error("expected WAF match on User-Agent header")
	} else if match.Field != "header:User-Agent" {
		t.Errorf("expected field header:User-Agent, got %s", match.Field)
	}
}

// httpTestServer is a test helper that creates a test HTTP server.
func httpTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}
