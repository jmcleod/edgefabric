package cdnserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
)

const (
	defaultHealthCheckInterval = 30 * time.Second
	defaultHealthCheckPath     = "/"
	defaultHealthCheckTimeout  = 5 * time.Second
	healthyThreshold           = 1 // Successes to mark healthy.
	unhealthyThreshold         = 3 // Failures to mark unhealthy.
)

// HealthChecker periodically checks the health of CDN origins.
type HealthChecker struct {
	mu      sync.RWMutex
	origins []*originHealth
	client  *http.Client
	stop    chan struct{}
	stopped chan struct{}
}

type originHealth struct {
	origin          *domain.CDNOrigin
	healthy         bool
	consecutiveFail int
	consecutiveOK   int
}

// NewHealthChecker creates a new origin health checker.
func NewHealthChecker(origins []*domain.CDNOrigin) *HealthChecker {
	oh := make([]*originHealth, len(origins))
	for i, o := range origins {
		oh[i] = &originHealth{
			origin:  o,
			healthy: true, // Assume healthy until proven otherwise.
		}
	}
	return &HealthChecker{
		origins: oh,
		client: &http.Client{
			Timeout: defaultHealthCheckTimeout,
		},
	}
}

// Start begins periodic health checking of all origins.
func (hc *HealthChecker) Start() {
	hc.stop = make(chan struct{})
	hc.stopped = make(chan struct{})

	go func() {
		defer close(hc.stopped)
		// Run an initial check immediately.
		hc.checkAll()

		ticker := time.NewTicker(defaultHealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.checkAll()
			case <-hc.stop:
				return
			}
		}
	}()
}

// Stop terminates health checking.
func (hc *HealthChecker) Stop() {
	if hc.stop != nil {
		close(hc.stop)
		<-hc.stopped
	}
}

// HealthyOrigins returns the origins currently marked as healthy.
func (hc *HealthChecker) HealthyOrigins() []*domain.CDNOrigin {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var result []*domain.CDNOrigin
	for _, oh := range hc.origins {
		if oh.healthy {
			result = append(result, oh.origin)
		}
	}
	return result
}

func (hc *HealthChecker) checkAll() {
	for _, oh := range hc.origins {
		hc.checkOrigin(oh)
	}
}

func (hc *HealthChecker) checkOrigin(oh *originHealth) {
	path := oh.origin.HealthCheckPath
	if path == "" {
		path = defaultHealthCheckPath
	}

	scheme := string(oh.origin.Scheme)
	if scheme == "" {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s%s", scheme, oh.origin.Address, path)
	ctx, cancel := context.WithTimeout(context.Background(), defaultHealthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		hc.markResult(oh, false)
		return
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		hc.markResult(oh, false)
		return
	}
	resp.Body.Close()

	// Consider 2xx and 3xx as healthy.
	hc.markResult(oh, resp.StatusCode >= 200 && resp.StatusCode < 400)
}

func (hc *HealthChecker) markResult(oh *originHealth, success bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if success {
		oh.consecutiveOK++
		oh.consecutiveFail = 0
		if oh.consecutiveOK >= healthyThreshold {
			oh.healthy = true
		}
	} else {
		oh.consecutiveFail++
		oh.consecutiveOK = 0
		if oh.consecutiveFail >= unhealthyThreshold {
			oh.healthy = false
		}
	}
}
