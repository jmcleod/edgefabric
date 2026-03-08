package domain

import "time"

// RouteProtocol enumerates the protocols supported for routing.
type RouteProtocol string

const (
	RouteProtocolTCP  RouteProtocol = "tcp"
	RouteProtocolUDP  RouteProtocol = "udp"
	RouteProtocolICMP RouteProtocol = "icmp"
	RouteProtocolAll  RouteProtocol = "all"
)

// RouteStatus represents whether a route is active.
type RouteStatus string

const (
	RouteStatusActive   RouteStatus = "active"
	RouteStatusDisabled RouteStatus = "disabled"
)

// Route is a traffic routing rule from anycast entry through nodes to gateways.
type Route struct {
	ID              ID            `json:"id" db:"id"`
	TenantID        ID            `json:"tenant_id" db:"tenant_id"`
	Name            string        `json:"name" db:"name"`
	Protocol        RouteProtocol `json:"protocol" db:"protocol"`
	EntryIP         string        `json:"entry_ip" db:"entry_ip"`
	EntryPort       *int          `json:"entry_port,omitempty" db:"entry_port"`
	GatewayID       ID            `json:"gateway_id" db:"gateway_id"`
	DestinationIP   string        `json:"destination_ip" db:"destination_ip"`
	DestinationPort *int          `json:"destination_port,omitempty" db:"destination_port"`
	NodeGroupID     *ID           `json:"node_group_id,omitempty" db:"node_group_id"`
	Status          RouteStatus   `json:"status" db:"status"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" db:"updated_at"`
}
