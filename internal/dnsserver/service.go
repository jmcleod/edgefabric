// Package dnsserver implements the node-side authoritative DNS server.
// It receives zone/record configuration from the controller via the DNS
// service's NodeDNSConfig and serves authoritative DNS responses.
package dnsserver

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/dns"
)

// Service is the node-side DNS server runtime interface.
// Implementations manage the local DNS server and reconcile the desired
// zone state from the controller into served DNS data.
type Service interface {
	// Start initializes the DNS server and begins listening.
	Start(ctx context.Context, listenAddr string) error

	// Stop shuts down the DNS server gracefully.
	Stop(ctx context.Context) error

	// Reconcile loads the desired DNS configuration from the controller
	// and updates the local zone data to match.
	Reconcile(ctx context.Context, config *dns.NodeDNSConfig) error

	// GetStatus returns the current runtime state of the DNS server.
	GetStatus(ctx context.Context) (*ServerStatus, error)
}

// ServerStatus represents the runtime status of the DNS server.
type ServerStatus struct {
	Listening    bool              `json:"listening"`
	ListenAddr   string            `json:"listen_addr"`
	ZoneCount    int               `json:"zone_count"`
	ZoneSerials  map[string]uint32 `json:"zone_serials"`
	QueriesTotal uint64            `json:"queries_total"`
}
