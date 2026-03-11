package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- Tenant ---

// TenantSpec defines the desired state of a Tenant.
type TenantSpec struct {
	// Name is the display name for the tenant.
	Name string `json:"name"`
	// Slug is the URL-safe identifier for the tenant.
	Slug string `json:"slug"`
}

// TenantStatus defines the observed state of a Tenant.
type TenantStatus struct {
	// ID is the EdgeFabric API ID assigned to this tenant.
	ID string `json:"id,omitempty"`
	// Phase is the reconciliation phase (Pending, Ready, Failed).
	Phase string `json:"phase,omitempty"`
	// Message provides additional status information.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Tenant is the Schema for the tenants API.
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenants.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

// --- Node ---

// NodeSpec defines the desired state of a Node.
type NodeSpec struct {
	Name      string `json:"name"`
	Hostname  string `json:"hostname"`
	Region    string `json:"region,omitempty"`
	PublicIP  string `json:"publicIP"`
	TenantRef string `json:"tenantRef"` // Name of the Tenant CR in the same namespace.
}

// NodeStatus defines the observed state of a Node.
type NodeStatus struct {
	ID          string `json:"id,omitempty"`
	Phase       string `json:"phase,omitempty"`
	Version     string `json:"version,omitempty"`
	WireGuardIP string `json:"wireGuardIP,omitempty"`
	Message     string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Node is the Schema for the nodes API.
type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeList contains a list of Nodes.
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Node `json:"items"`
}

// --- Gateway ---

// GatewaySpec defines the desired state of a Gateway.
type GatewaySpec struct {
	Name      string `json:"name"`
	PublicIP  string `json:"publicIP"`
	TenantRef string `json:"tenantRef"`
}

// GatewayStatus defines the observed state of a Gateway.
type GatewayStatus struct {
	ID      string `json:"id,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Gateway is the Schema for the gateways API.
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec,omitempty"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayList contains a list of Gateways.
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gateway `json:"items"`
}

// --- DNSZone ---

// DNSRecordSpec defines a DNS record within a zone.
type DNSRecordSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // A, AAAA, CNAME, MX, TXT, NS, SRV
	Value    string `json:"value"`
	TTL      int    `json:"ttl,omitempty"`
	Priority *int   `json:"priority,omitempty"`
}

// DNSZoneSpec defines the desired state of a DNSZone.
type DNSZoneSpec struct {
	Name      string          `json:"name"` // e.g. "example.com"
	TenantRef string          `json:"tenantRef"`
	Records   []DNSRecordSpec `json:"records,omitempty"`
}

// DNSZoneStatus defines the observed state of a DNSZone.
type DNSZoneStatus struct {
	ID          string `json:"id,omitempty"`
	Phase       string `json:"phase,omitempty"`
	Serial      int    `json:"serial,omitempty"`
	RecordCount int    `json:"recordCount,omitempty"`
	Message     string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DNSZone is the Schema for the dnszones API.
type DNSZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSZoneSpec   `json:"spec,omitempty"`
	Status DNSZoneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DNSZoneList contains a list of DNSZones.
type DNSZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSZone `json:"items"`
}

// --- CDNSite ---

// CDNOriginSpec defines an origin server for a CDN site.
type CDNOriginSpec struct {
	Address             string `json:"address"` // host:port
	Scheme              string `json:"scheme"`  // http or https
	Weight              int    `json:"weight,omitempty"`
	HealthCheckPath     string `json:"healthCheckPath,omitempty"`
	HealthCheckInterval int    `json:"healthCheckInterval,omitempty"`
}

// CDNSiteSpec defines the desired state of a CDNSite.
type CDNSiteSpec struct {
	Name               string          `json:"name"`
	TenantRef          string          `json:"tenantRef"`
	Domains            []string        `json:"domains,omitempty"`
	TLSMode            string          `json:"tlsMode,omitempty"`
	CacheEnabled       bool            `json:"cacheEnabled,omitempty"`
	CacheTTL           int             `json:"cacheTTL,omitempty"`
	CompressionEnabled bool            `json:"compressionEnabled,omitempty"`
	RateLimitRPS       *int            `json:"rateLimitRPS,omitempty"`
	WAFEnabled         bool            `json:"wafEnabled,omitempty"`
	WAFMode            string          `json:"wafMode,omitempty"`
	NodeGroupRef       string          `json:"nodeGroupRef,omitempty"`
	Origins            []CDNOriginSpec `json:"origins,omitempty"`
}

// CDNSiteStatus defines the observed state of a CDNSite.
type CDNSiteStatus struct {
	ID          string `json:"id,omitempty"`
	Phase       string `json:"phase,omitempty"`
	DomainCount int    `json:"domainCount,omitempty"`
	Message     string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CDNSite is the Schema for the cdnsites API.
type CDNSite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CDNSiteSpec   `json:"spec,omitempty"`
	Status CDNSiteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CDNSiteList contains a list of CDNSites.
type CDNSiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CDNSite `json:"items"`
}

// --- Route ---

// RouteSpec defines the desired state of a Route.
type RouteSpec struct {
	Name            string `json:"name"`
	TenantRef       string `json:"tenantRef"`
	Protocol        string `json:"protocol"` // tcp, udp, icmp, all
	EntryIP         string `json:"entryIP"`
	EntryPort       *int   `json:"entryPort,omitempty"`
	DestinationIP   string `json:"destinationIP"`
	DestinationPort *int   `json:"destinationPort,omitempty"`
	GatewayRef      string `json:"gatewayRef"`
}

// RouteStatus defines the observed state of a Route.
type RouteStatus struct {
	ID      string `json:"id,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Route is the Schema for the routes API.
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteList contains a list of Routes.
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Route `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&Tenant{}, &TenantList{},
		&Node{}, &NodeList{},
		&Gateway{}, &GatewayList{},
		&DNSZone{}, &DNSZoneList{},
		&CDNSite{}, &CDNSiteList{},
		&Route{}, &RouteList{},
	)
}
