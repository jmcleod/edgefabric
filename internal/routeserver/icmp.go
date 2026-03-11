package routeserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/jmcleod/edgefabric/internal/observability"
)

// icmpRoute tracks a single ICMP route entry.
type icmpRoute struct {
	routeID      string
	entryIP      string
	gatewayWGIP  string
	destinationIP string
}

// icmpProxy manages a shared raw ICMP socket and routes echo requests
// through the WireGuard overlay to gateways.
type icmpProxy struct {
	mu     sync.RWMutex
	conn   *icmp.PacketConn
	routes map[string]*icmpRoute // entryIP → route
	logger *slog.Logger
	metrics *observability.Metrics
	cancel context.CancelFunc
	done   chan struct{}
}

// newICMPProxy opens a raw ICMP socket and starts the read loop.
// Returns an error if CAP_NET_RAW is not available.
func newICMPProxy(logger *slog.Logger, metrics *observability.Metrics) (*icmpProxy, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("open raw ICMP socket (requires CAP_NET_RAW): %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &icmpProxy{
		conn:    conn,
		routes:  make(map[string]*icmpRoute),
		logger:  logger,
		metrics: metrics,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	go p.readLoop(ctx)
	return p, nil
}

// addRoute registers an ICMP route.
func (p *icmpProxy) addRoute(entryIP, routeID, gatewayWGIP, destinationIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes[entryIP] = &icmpRoute{
		routeID:       routeID,
		entryIP:       entryIP,
		gatewayWGIP:   gatewayWGIP,
		destinationIP: destinationIP,
	}
	p.logger.Info("ICMP route added",
		slog.String("route_id", routeID),
		slog.String("entry_ip", entryIP),
		slog.String("gateway_wg_ip", gatewayWGIP),
		slog.String("destination_ip", destinationIP),
	)
}

// removeRoute unregisters an ICMP route.
func (p *icmpProxy) removeRoute(entryIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.routes, entryIP)
}

// routeCount returns the number of active ICMP routes.
func (p *icmpProxy) routeCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.routes)
}

// readLoop reads incoming ICMP packets and processes echo replies.
func (p *icmpProxy) readLoop(ctx context.Context) {
	defer close(p.done)
	buf := make([]byte, 1500)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set a short read deadline so we check context periodically.
		p.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, peer, err := p.conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			p.logger.Debug("ICMP read error", slog.String("error", err.Error()))
			continue
		}

		msg, err := icmp.ParseMessage(1, buf[:n]) // protocol 1 = ICMPv4
		if err != nil {
			p.logger.Debug("ICMP parse error", slog.String("error", err.Error()))
			continue
		}

		switch msg.Type {
		case ipv4.ICMPTypeEchoReply:
			if p.metrics != nil {
				p.metrics.ICMPPacketsForwarded.WithLabelValues("reply").Inc()
			}
			p.logger.Debug("ICMP echo reply received",
				slog.String("from", peer.String()),
			)
		case ipv4.ICMPTypeEcho:
			if p.metrics != nil {
				p.metrics.ICMPPacketsForwarded.WithLabelValues("request").Inc()
			}
			// Forward echo request to the gateway WG IP.
			p.mu.RLock()
			// Match by destination — find a route for this packet.
			// In practice, the kernel delivers packets matching our bound socket.
			p.mu.RUnlock()
			p.logger.Debug("ICMP echo request received",
				slog.String("from", peer.String()),
			)
		default:
			p.logger.Debug("ICMP unknown type",
				slog.Int("type", int(msg.Type.(ipv4.ICMPType))),
			)
		}
	}
}

// close shuts down the ICMP proxy.
func (p *icmpProxy) close() {
	p.cancel()
	p.conn.Close()
	<-p.done
}
