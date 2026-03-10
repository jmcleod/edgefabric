// Package nodeclient provides an HTTP client for node agents to poll
// configuration from the controller.
package nodeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jmcleod/edgefabric/internal/cdn"
	"github.com/jmcleod/edgefabric/internal/dns"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/route"
)

// Client polls the controller API for node configuration.
type Client struct {
	baseURL    string
	nodeID     string
	apiToken   string
	httpClient *http.Client
}

// New creates a controller client for a specific node.
func New(baseURL, nodeID, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		nodeID:   nodeID,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Enroll calls POST /api/v1/enroll with the given token and returns the
// enrollment result (node_id, api_token, wireguard_ip).
func Enroll(ctx context.Context, controllerURL, enrollmentToken string) (*EnrollResult, error) {
	body := fmt.Sprintf(`{"token":%q}`, enrollmentToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controllerURL+"/api/v1/enroll", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("enroll request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("enrollment failed (HTTP %d): %s", resp.StatusCode, data)
	}

	var result EnrollResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode enrollment response: %w", err)
	}
	return &result, nil
}

// EnrollResult mirrors the enrollment API response.
type EnrollResult struct {
	Status      string `json:"status"`
	NodeID      string `json:"node_id"`
	APIToken    string `json:"api_token"`
	WireGuardIP string `json:"wireguard_ip"`
}

// FetchBGPConfig polls GET /api/v1/nodes/{id}/config/bgp.
func (c *Client) FetchBGPConfig(ctx context.Context) ([]*domain.BGPSession, error) {
	var sessions []*domain.BGPSession
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v1/nodes/%s/config/bgp", c.nodeID), &sessions); err != nil {
		return nil, fmt.Errorf("fetch BGP config: %w", err)
	}
	return sessions, nil
}

// FetchDNSConfig polls GET /api/v1/nodes/{id}/config/dns.
func (c *Client) FetchDNSConfig(ctx context.Context) (*dns.NodeDNSConfig, error) {
	var config dns.NodeDNSConfig
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v1/nodes/%s/config/dns", c.nodeID), &config); err != nil {
		return nil, fmt.Errorf("fetch DNS config: %w", err)
	}
	return &config, nil
}

// FetchCDNConfig polls GET /api/v1/nodes/{id}/config/cdn.
func (c *Client) FetchCDNConfig(ctx context.Context) (*cdn.NodeCDNConfig, error) {
	var config cdn.NodeCDNConfig
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v1/nodes/%s/config/cdn", c.nodeID), &config); err != nil {
		return nil, fmt.Errorf("fetch CDN config: %w", err)
	}
	return &config, nil
}

// FetchRouteConfig polls GET /api/v1/nodes/{id}/config/routes.
func (c *Client) FetchRouteConfig(ctx context.Context) (*route.NodeRouteConfig, error) {
	var config route.NodeRouteConfig
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v1/nodes/%s/config/routes", c.nodeID), &config); err != nil {
		return nil, fmt.Errorf("fetch route config: %w", err)
	}
	return &config, nil
}

// getJSON performs an authenticated GET request and decodes the JSON response.
func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, data)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

