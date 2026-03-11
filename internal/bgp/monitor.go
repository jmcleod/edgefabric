package bgp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/observability"
)

// MonitorConfig configures BGP session monitoring.
type MonitorConfig struct {
	PollInterval time.Duration
}

// DefaultMonitorConfig returns sensible defaults.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{PollInterval: 15 * time.Second}
}

// peerState tracks the last-known status for a single BGP peer.
type peerState struct {
	lastStatus string
}

// Monitor watches BGP session state changes and publishes events.
type Monitor struct {
	svc      Service
	config   MonitorConfig
	eventBus *events.Bus
	metrics  *observability.Metrics
	logger   *slog.Logger

	mu         sync.Mutex
	peerStates map[string]*peerState // keyed by PeerAddress
	stop       chan struct{}
	stopped    chan struct{}
}

// NewMonitor creates a BGP session monitor.
func NewMonitor(
	svc Service,
	config MonitorConfig,
	eventBus *events.Bus,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *Monitor {
	return &Monitor{
		svc:        svc,
		config:     config,
		eventBus:   eventBus,
		metrics:    metrics,
		logger:     logger,
		peerStates: make(map[string]*peerState),
	}
}

// Start begins the monitoring loop.
func (m *Monitor) Start(ctx context.Context) {
	m.stop = make(chan struct{})
	m.stopped = make(chan struct{})

	go func() {
		defer close(m.stopped)

		// Initial poll — capture baseline state without firing events.
		m.poll(ctx, true)

		ticker := time.NewTicker(m.config.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.poll(ctx, false)
			case <-m.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop terminates the monitoring loop.
func (m *Monitor) Stop() {
	if m.stop != nil {
		close(m.stop)
		<-m.stopped
	}
}

// poll fetches current session states and detects transitions.
func (m *Monitor) poll(ctx context.Context, initial bool) {
	sessions, err := m.svc.GetStatus(ctx)
	if err != nil {
		m.logger.Warn("BGP monitor poll failed", slog.String("error", err.Error()))
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool, len(sessions))

	for _, sess := range sessions {
		seen[sess.PeerAddress] = true

		prev, exists := m.peerStates[sess.PeerAddress]
		peerASN := fmt.Sprintf("%d", sess.PeerASN)

		// Update gauge.
		if m.metrics != nil {
			if sess.Status == "established" {
				m.metrics.BGPSessionState.WithLabelValues(sess.PeerAddress, peerASN).Set(1)
			} else {
				m.metrics.BGPSessionState.WithLabelValues(sess.PeerAddress, peerASN).Set(0)
			}
		}

		if !exists {
			// First time seeing this peer — record state, no event on initial poll.
			m.peerStates[sess.PeerAddress] = &peerState{lastStatus: sess.Status}
			continue
		}

		if initial {
			prev.lastStatus = sess.Status
			continue
		}

		// Detect transitions.
		if prev.lastStatus != sess.Status {
			if sess.Status == "established" && prev.lastStatus != "established" {
				// Transitioned to established.
				m.publishEvent(ctx, events.BGPSessionEstablished, events.SeverityInfo,
					fmt.Sprintf("bgp/%s", sess.PeerAddress),
					map[string]string{
						"peer_address": sess.PeerAddress,
						"peer_asn":     peerASN,
						"from_status":  prev.lastStatus,
					},
				)
				if m.metrics != nil {
					m.metrics.BGPSessionTransitionsTotal.WithLabelValues(sess.PeerAddress, "established").Inc()
				}
			} else if prev.lastStatus == "established" && sess.Status != "established" {
				// Transitioned away from established.
				m.publishEvent(ctx, events.BGPSessionDown, events.SeverityCritical,
					fmt.Sprintf("bgp/%s", sess.PeerAddress),
					map[string]string{
						"peer_address": sess.PeerAddress,
						"peer_asn":     peerASN,
						"to_status":    sess.Status,
					},
				)
				if m.metrics != nil {
					m.metrics.BGPSessionTransitionsTotal.WithLabelValues(sess.PeerAddress, "down").Inc()
				}
			}
			prev.lastStatus = sess.Status
		}
	}

	// Clean up peers that no longer exist.
	for addr := range m.peerStates {
		if !seen[addr] {
			delete(m.peerStates, addr)
		}
	}
}

func (m *Monitor) publishEvent(ctx context.Context, eventType events.EventType, severity events.Severity, resource string, details map[string]string) {
	if m.eventBus == nil {
		return
	}
	m.eventBus.Publish(ctx, events.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Severity:  severity,
		Resource:  resource,
		Details:   details,
	})
}
