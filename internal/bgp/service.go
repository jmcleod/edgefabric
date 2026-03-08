// Package bgp defines the node-side BGP runtime interface and implementations.
// The BGP service runs on each node and manages local BGP sessions with
// upstream peers, announcing tenant-exclusive anycast prefixes.
package bgp

import (
	"context"

	"github.com/jmcleod/edgefabric/internal/domain"
)

// Service is the node-side BGP runtime interface.
// Implementations manage the local BGP speaker (e.g., GoBGP) and reconcile
// the desired state from the controller into actual BGP sessions.
type Service interface {
	// Start initializes the BGP speaker with the given router ID and local ASN.
	Start(ctx context.Context, routerID string, localASN uint32) error

	// Stop shuts down the BGP speaker gracefully.
	Stop(ctx context.Context) error

	// Reconcile converges the actual BGP state to match the desired sessions
	// from the controller. It adds/removes/updates peers as needed.
	Reconcile(ctx context.Context, desired []*domain.BGPSession) error

	// GetStatus returns the current runtime state of all BGP sessions.
	GetStatus(ctx context.Context) ([]SessionState, error)

	// AnnouncePrefix advertises a prefix to all peers via BGP.
	AnnouncePrefix(ctx context.Context, prefix string, nextHop string) error

	// WithdrawPrefix removes a previously announced prefix.
	WithdrawPrefix(ctx context.Context, prefix string) error
}

// SessionState represents the runtime status of a single BGP session.
type SessionState struct {
	SessionID     domain.ID `json:"session_id"`
	PeerASN       uint32    `json:"peer_asn"`
	PeerAddress   string    `json:"peer_address"`
	LocalASN      uint32    `json:"local_asn"`
	Status        string    `json:"status"` // "idle", "connect", "active", "opensent", "openconfirm", "established"
	PrefixesOut   int       `json:"prefixes_out"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}
