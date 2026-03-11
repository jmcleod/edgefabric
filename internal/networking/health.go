package networking

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/observability"
)

const (
	defaultOverlayHealthInterval      = 30 * time.Second
	defaultOverlayHealthTimeout       = 5 * time.Second
	defaultOverlayUnhealthyThreshold  = 3
	defaultOverlayHealthyThreshold    = 1
	defaultOverlayProbePort           = 9090
)

// OverlayHealthConfig configures WireGuard overlay health checking.
type OverlayHealthConfig struct {
	Interval           time.Duration
	Timeout            time.Duration
	UnhealthyThreshold int
	HealthyThreshold   int
	ProbePort          int // TCP port to probe on overlay peers. Default: 9090 (health server)
}

// DefaultOverlayHealthConfig returns sensible defaults.
func DefaultOverlayHealthConfig() OverlayHealthConfig {
	return OverlayHealthConfig{
		Interval:           defaultOverlayHealthInterval,
		Timeout:            defaultOverlayHealthTimeout,
		UnhealthyThreshold: defaultOverlayUnhealthyThreshold,
		HealthyThreshold:   defaultOverlayHealthyThreshold,
		ProbePort:          defaultOverlayProbePort,
	}
}

// OverlayTarget is a peer to health-check over the overlay.
type OverlayTarget struct {
	Name string // e.g., "controller" or peer node name
	IP   string // WireGuard overlay IP
}

// overlayPeerState tracks per-peer health.
type overlayPeerState struct {
	target          OverlayTarget
	healthy         bool
	consecutiveFail int
	consecutiveOK   int
}

// OverlayHealthChecker periodically probes WireGuard overlay peers.
type OverlayHealthChecker struct {
	mu       sync.RWMutex
	peers    []*overlayPeerState
	config   OverlayHealthConfig
	eventBus *events.Bus
	metrics  *observability.Metrics
	logger   *slog.Logger
	stop     chan struct{}
	stopped  chan struct{}
}

// NewOverlayHealthChecker creates a checker for the given overlay targets.
func NewOverlayHealthChecker(
	targets []OverlayTarget,
	config OverlayHealthConfig,
	eventBus *events.Bus,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *OverlayHealthChecker {
	peers := make([]*overlayPeerState, len(targets))
	for i, t := range targets {
		peers[i] = &overlayPeerState{
			target:  t,
			healthy: true, // Assume healthy until proven otherwise.
		}
	}
	return &OverlayHealthChecker{
		peers:    peers,
		config:   config,
		eventBus: eventBus,
		metrics:  metrics,
		logger:   logger,
	}
}

// Start begins the periodic health check loop.
func (c *OverlayHealthChecker) Start() {
	c.stop = make(chan struct{})
	c.stopped = make(chan struct{})

	go func() {
		defer close(c.stopped)
		// Initial check.
		c.checkAll()

		ticker := time.NewTicker(c.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.checkAll()
			case <-c.stop:
				return
			}
		}
	}()
}

// Stop terminates the health check loop.
func (c *OverlayHealthChecker) Stop() {
	if c.stop != nil {
		close(c.stop)
		<-c.stopped
	}
}

// IsHealthy returns whether a given peer is currently healthy.
func (c *OverlayHealthChecker) IsHealthy(peerIP string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.peers {
		if p.target.IP == peerIP {
			return p.healthy
		}
	}
	return false
}

func (c *OverlayHealthChecker) checkAll() {
	for _, peer := range c.peers {
		c.checkPeer(peer)
	}
}

func (c *OverlayHealthChecker) checkPeer(peer *overlayPeerState) {
	addr := fmt.Sprintf("%s:%d", peer.target.IP, c.config.ProbePort)
	conn, err := net.DialTimeout("tcp", addr, c.config.Timeout)
	success := err == nil
	if conn != nil {
		conn.Close()
	}

	c.markResult(peer, success)
}

func (c *OverlayHealthChecker) markResult(peer *overlayPeerState, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	peerName := peer.target.Name
	result := "success"
	if !success {
		result = "failure"
	}

	if c.metrics != nil {
		c.metrics.OverlayHealthChecksTotal.WithLabelValues(peerName, result).Inc()
	}

	wasHealthy := peer.healthy

	if success {
		peer.consecutiveOK++
		peer.consecutiveFail = 0
		if peer.consecutiveOK >= c.config.HealthyThreshold {
			peer.healthy = true
		}
	} else {
		peer.consecutiveFail++
		peer.consecutiveOK = 0
		if peer.consecutiveFail >= c.config.UnhealthyThreshold {
			peer.healthy = false
		}
	}

	if c.metrics != nil {
		val := float64(0)
		if peer.healthy {
			val = 1
		}
		c.metrics.OverlayPeerHealthy.WithLabelValues(peerName).Set(val)
	}

	// Fire events on state transitions.
	if wasHealthy && !peer.healthy {
		c.publishEvent(events.OverlayPeerUnreachable, events.SeverityCritical,
			fmt.Sprintf("overlay/%s", peer.target.IP),
			map[string]string{
				"peer_name":           peerName,
				"peer_ip":             peer.target.IP,
				"consecutive_failures": fmt.Sprintf("%d", peer.consecutiveFail),
			},
		)
	} else if !wasHealthy && peer.healthy {
		c.publishEvent(events.OverlayPeerRecovered, events.SeverityInfo,
			fmt.Sprintf("overlay/%s", peer.target.IP),
			map[string]string{
				"peer_name": peerName,
				"peer_ip":   peer.target.IP,
			},
		)
	}
}

func (c *OverlayHealthChecker) publishEvent(eventType events.EventType, severity events.Severity, resource string, details map[string]string) {
	if c.eventBus == nil {
		return
	}
	c.eventBus.Publish(context.Background(), events.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Severity:  severity,
		Resource:  resource,
		Details:   details,
	})
}
