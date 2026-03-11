package routeserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/events"
	"github.com/jmcleod/edgefabric/internal/observability"
)

const (
	defaultRouteHealthInterval      = 30 * time.Second
	defaultRouteHealthTimeout       = 5 * time.Second
	defaultRouteUnhealthyThreshold  = 3
	defaultRouteHealthyThreshold    = 1
)

// RouteHealthConfig configures route destination health checking.
type RouteHealthConfig struct {
	Interval           time.Duration
	Timeout            time.Duration
	UnhealthyThreshold int
	HealthyThreshold   int
}

// DefaultRouteHealthConfig returns sensible defaults.
func DefaultRouteHealthConfig() RouteHealthConfig {
	return RouteHealthConfig{
		Interval:           defaultRouteHealthInterval,
		Timeout:            defaultRouteHealthTimeout,
		UnhealthyThreshold: defaultRouteUnhealthyThreshold,
		HealthyThreshold:   defaultRouteHealthyThreshold,
	}
}

// RouteTarget describes a route destination to health-check.
type RouteTarget struct {
	RouteID     domain.ID
	RouteName   string
	Protocol    domain.RouteProtocol
	GatewayWGIP string
	EntryPort   int
}

// routeHealthState tracks per-route health.
type routeHealthState struct {
	target          RouteTarget
	healthy         bool
	consecutiveFail int
	consecutiveOK   int
}

// RouteHealthChecker periodically probes route destinations.
type RouteHealthChecker struct {
	mu       sync.RWMutex
	routes   map[domain.ID]*routeHealthState
	config   RouteHealthConfig
	eventBus *events.Bus
	metrics  *observability.Metrics
	logger   *slog.Logger
	stop     chan struct{}
	stopped  chan struct{}
}

// NewRouteHealthChecker creates a route health checker.
func NewRouteHealthChecker(
	config RouteHealthConfig,
	eventBus *events.Bus,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *RouteHealthChecker {
	return &RouteHealthChecker{
		routes:   make(map[domain.ID]*routeHealthState),
		config:   config,
		eventBus: eventBus,
		metrics:  metrics,
		logger:   logger,
	}
}

// Start begins the periodic health check loop.
func (c *RouteHealthChecker) Start() {
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
func (c *RouteHealthChecker) Stop() {
	if c.stop != nil {
		close(c.stop)
		<-c.stopped
	}
}

// UpdateTargets replaces the set of routes being monitored.
// Called from the route reconciliation loop when routes change.
func (c *RouteHealthChecker) UpdateTargets(targets []RouteTarget) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newRoutes := make(map[domain.ID]*routeHealthState, len(targets))
	for _, t := range targets {
		if existing, ok := c.routes[t.RouteID]; ok {
			// Preserve health state for existing routes.
			existing.target = t
			newRoutes[t.RouteID] = existing
		} else {
			newRoutes[t.RouteID] = &routeHealthState{
				target:  t,
				healthy: true, // Assume healthy until proven otherwise.
			}
		}
	}
	c.routes = newRoutes
}

// IsRouteHealthy returns whether a given route is currently healthy.
func (c *RouteHealthChecker) IsRouteHealthy(routeID domain.ID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if state, ok := c.routes[routeID]; ok {
		return state.healthy
	}
	return false
}

func (c *RouteHealthChecker) checkAll() {
	c.mu.RLock()
	// Snapshot current routes to avoid holding lock during probes.
	states := make([]*routeHealthState, 0, len(c.routes))
	for _, s := range c.routes {
		states = append(states, s)
	}
	c.mu.RUnlock()

	for _, state := range states {
		c.checkRoute(state)
	}
}

func (c *RouteHealthChecker) checkRoute(state *routeHealthState) {
	t := state.target

	// Only TCP probes for now; UDP routes are assumed healthy.
	if t.Protocol != domain.RouteProtocolTCP || t.EntryPort == 0 || t.GatewayWGIP == "" {
		return
	}

	addr := fmt.Sprintf("%s:%d", t.GatewayWGIP, t.EntryPort)
	conn, err := net.DialTimeout("tcp", addr, c.config.Timeout)
	success := err == nil
	if conn != nil {
		conn.Close()
	}

	c.markResult(state, success)
}

func (c *RouteHealthChecker) markResult(state *routeHealthState, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	t := state.target
	routeID := t.RouteID.String()
	result := "success"
	if !success {
		result = "failure"
	}

	if c.metrics != nil {
		c.metrics.RouteHealthChecksTotal.WithLabelValues(routeID, t.RouteName, result).Inc()
	}

	wasHealthy := state.healthy

	if success {
		state.consecutiveOK++
		state.consecutiveFail = 0
		if state.consecutiveOK >= c.config.HealthyThreshold {
			state.healthy = true
		}
	} else {
		state.consecutiveFail++
		state.consecutiveOK = 0
		if state.consecutiveFail >= c.config.UnhealthyThreshold {
			state.healthy = false
		}
	}

	if c.metrics != nil {
		val := float64(0)
		if state.healthy {
			val = 1
		}
		c.metrics.RouteHealthy.WithLabelValues(routeID, t.RouteName).Set(val)
	}

	// Fire events on state transitions.
	if wasHealthy && !state.healthy {
		c.publishEvent(events.RouteHealthCheckFailed, events.SeverityCritical,
			fmt.Sprintf("route/%s", routeID),
			map[string]string{
				"route_name":         t.RouteName,
				"gateway_ip":         t.GatewayWGIP,
				"entry_port":         fmt.Sprintf("%d", t.EntryPort),
				"consecutive_failures": fmt.Sprintf("%d", state.consecutiveFail),
			},
		)
	} else if !wasHealthy && state.healthy {
		c.publishEvent(events.RouteHealthCheckRecovered, events.SeverityInfo,
			fmt.Sprintf("route/%s", routeID),
			map[string]string{
				"route_name": t.RouteName,
				"gateway_ip": t.GatewayWGIP,
				"entry_port": fmt.Sprintf("%d", t.EntryPort),
			},
		)
	}
}

func (c *RouteHealthChecker) publishEvent(eventType events.EventType, severity events.Severity, resource string, details map[string]string) {
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
