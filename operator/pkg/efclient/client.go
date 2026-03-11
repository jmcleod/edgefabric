// Package efclient provides an HTTP client for the EdgeFabric REST API.
// It is used by the Kubernetes operator to create, read, update, and delete
// resources in the EdgeFabric control plane.
package efclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the EdgeFabric REST API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// New creates a new EdgeFabric API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIError represents an error response from the EdgeFabric API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("edgefabric API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// --- Generic Resource Response ---

// ResourceResponse represents a generic create/get response with an ID.
type ResourceResponse struct {
	ID string `json:"id"`
}

// --- Tenant ---

// CreateTenantRequest is the request body for creating a tenant.
type CreateTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (c *Client) CreateTenant(ctx context.Context, req CreateTenantRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, "/api/v1/tenants", req)
}

func (c *Client) GetTenant(ctx context.Context, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, "/api/v1/tenants/"+id)
}

func (c *Client) UpdateTenant(ctx context.Context, id string, req CreateTenantRequest) error {
	return doPut(ctx, c, "/api/v1/tenants/"+id, req)
}

func (c *Client) DeleteTenant(ctx context.Context, id string) error {
	return doDelete(ctx, c, "/api/v1/tenants/"+id)
}

// --- Node ---

// CreateNodeRequest is the request body for creating a node.
type CreateNodeRequest struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Region   string `json:"region,omitempty"`
	PublicIP string `json:"public_ip"`
	TenantID string `json:"tenant_id"`
}

func (c *Client) CreateNode(ctx context.Context, req CreateNodeRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, "/api/v1/nodes", req)
}

func (c *Client) GetNode(ctx context.Context, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, "/api/v1/nodes/"+id)
}

func (c *Client) UpdateNode(ctx context.Context, id string, req CreateNodeRequest) error {
	return doPut(ctx, c, "/api/v1/nodes/"+id, req)
}

func (c *Client) DeleteNode(ctx context.Context, id string) error {
	return doDelete(ctx, c, "/api/v1/nodes/"+id)
}

// --- Gateway ---

// CreateGatewayRequest is the request body for creating a gateway.
type CreateGatewayRequest struct {
	Name     string `json:"name"`
	PublicIP string `json:"public_ip"`
	TenantID string `json:"tenant_id"`
}

func (c *Client) CreateGateway(ctx context.Context, req CreateGatewayRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, "/api/v1/gateways", req)
}

func (c *Client) GetGateway(ctx context.Context, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, "/api/v1/gateways/"+id)
}

func (c *Client) UpdateGateway(ctx context.Context, id string, req CreateGatewayRequest) error {
	return doPut(ctx, c, "/api/v1/gateways/"+id, req)
}

func (c *Client) DeleteGateway(ctx context.Context, id string) error {
	return doDelete(ctx, c, "/api/v1/gateways/"+id)
}

// --- DNSZone ---

// CreateDNSZoneRequest is the request body for creating a DNS zone.
type CreateDNSZoneRequest struct {
	Name     string          `json:"name"`
	TenantID string          `json:"tenant_id"`
	Records  []DNSRecordBody `json:"records,omitempty"`
}

// DNSRecordBody is a DNS record in the API request.
type DNSRecordBody struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl,omitempty"`
	Priority *int   `json:"priority,omitempty"`
}

func (c *Client) CreateDNSZone(ctx context.Context, tenantID string, req CreateDNSZoneRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/dns/zones", tenantID), req)
}

func (c *Client) GetDNSZone(ctx context.Context, tenantID, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/dns/zones/%s", tenantID, id))
}

func (c *Client) UpdateDNSZone(ctx context.Context, tenantID, id string, req CreateDNSZoneRequest) error {
	return doPut(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/dns/zones/%s", tenantID, id), req)
}

func (c *Client) DeleteDNSZone(ctx context.Context, tenantID, id string) error {
	return doDelete(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/dns/zones/%s", tenantID, id))
}

// --- CDNSite ---

// CreateCDNSiteRequest is the request body for creating a CDN site.
type CreateCDNSiteRequest struct {
	Name               string           `json:"name"`
	TenantID           string           `json:"tenant_id"`
	Domains            []string         `json:"domains,omitempty"`
	TLSMode            string           `json:"tls_mode,omitempty"`
	CacheEnabled       bool             `json:"cache_enabled,omitempty"`
	CacheTTL           int              `json:"cache_ttl,omitempty"`
	CompressionEnabled bool             `json:"compression_enabled,omitempty"`
	RateLimitRPS       *int             `json:"rate_limit_rps,omitempty"`
	WAFEnabled         bool             `json:"waf_enabled,omitempty"`
	WAFMode            string           `json:"waf_mode,omitempty"`
	NodeGroupID        string           `json:"node_group_id,omitempty"`
	Origins            []CDNOriginBody  `json:"origins,omitempty"`
}

// CDNOriginBody is a CDN origin in the API request.
type CDNOriginBody struct {
	Address             string `json:"address"`
	Scheme              string `json:"scheme"`
	Weight              int    `json:"weight,omitempty"`
	HealthCheckPath     string `json:"health_check_path,omitempty"`
	HealthCheckInterval int    `json:"health_check_interval,omitempty"`
}

func (c *Client) CreateCDNSite(ctx context.Context, tenantID string, req CreateCDNSiteRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/cdn/sites", tenantID), req)
}

func (c *Client) GetCDNSite(ctx context.Context, tenantID, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/cdn/sites/%s", tenantID, id))
}

func (c *Client) UpdateCDNSite(ctx context.Context, tenantID, id string, req CreateCDNSiteRequest) error {
	return doPut(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/cdn/sites/%s", tenantID, id), req)
}

func (c *Client) DeleteCDNSite(ctx context.Context, tenantID, id string) error {
	return doDelete(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/cdn/sites/%s", tenantID, id))
}

// --- Route ---

// CreateRouteRequest is the request body for creating a route.
type CreateRouteRequest struct {
	Name            string `json:"name"`
	TenantID        string `json:"tenant_id"`
	Protocol        string `json:"protocol"`
	EntryIP         string `json:"entry_ip"`
	EntryPort       *int   `json:"entry_port,omitempty"`
	DestinationIP   string `json:"destination_ip"`
	DestinationPort *int   `json:"destination_port,omitempty"`
	GatewayID       string `json:"gateway_id"`
}

func (c *Client) CreateRoute(ctx context.Context, tenantID string, req CreateRouteRequest) (*ResourceResponse, error) {
	return doCreate[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/routes", tenantID), req)
}

func (c *Client) GetRoute(ctx context.Context, tenantID, id string) (*ResourceResponse, error) {
	return doGet[ResourceResponse](ctx, c, fmt.Sprintf("/api/v1/tenants/%s/routes/%s", tenantID, id))
}

func (c *Client) UpdateRoute(ctx context.Context, tenantID, id string, req CreateRouteRequest) error {
	return doPut(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/routes/%s", tenantID, id), req)
}

func (c *Client) DeleteRoute(ctx context.Context, tenantID, id string) error {
	return doDelete(ctx, c, fmt.Sprintf("/api/v1/tenants/%s/routes/%s", tenantID, id))
}

// --- HTTP helpers ---

func doCreate[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, readError(resp)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func doGet[T any](ctx context.Context, c *Client, path string) (*T, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func doPut(ctx context.Context, c *Client, path string, body any) error {
	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return readError(resp)
	}
	return nil
}

func doDelete(ctx context.Context, c *Client, path string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return readError(resp)
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	return c.HTTPClient.Do(req)
}

func readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &APIError{
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}
