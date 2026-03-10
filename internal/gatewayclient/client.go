// Package gatewayclient provides an HTTP client for gateway agents to poll
// configuration from the controller.
package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jmcleod/edgefabric/internal/route"
)

// Client polls the controller API for gateway configuration.
type Client struct {
	baseURL    string
	gatewayID  string
	apiToken   string
	httpClient *http.Client
}

// New creates a controller client for a specific gateway.
func New(baseURL, gatewayID, apiToken string) *Client {
	return &Client{
		baseURL:   baseURL,
		gatewayID: gatewayID,
		apiToken:  apiToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchRouteConfig polls GET /api/v1/gateways/{id}/config/routes.
func (c *Client) FetchRouteConfig(ctx context.Context) (*route.GatewayRouteConfig, error) {
	var config route.GatewayRouteConfig
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v1/gateways/%s/config/routes", c.gatewayID), &config); err != nil {
		return nil, fmt.Errorf("fetch route config: %w", err)
	}
	return &config, nil
}

// apiEnvelope unwraps the standard {data: ...} response envelope used by
// all controller API endpoints (via apiutil.JSON).
type apiEnvelope struct {
	Data json.RawMessage `json:"data"`
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

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("empty data in response")
	}
	if err := json.Unmarshal(envelope.Data, dst); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}
	return nil
}
