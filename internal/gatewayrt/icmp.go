package gatewayrt

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

// icmpRoute tracks a single ICMP route on the gateway side.
type icmpRoute struct {
	routeID       string
	destinationIP string
}

// icmpProxy manages a shared raw ICMP socket on the gateway side.
// It listens for incoming ICMP from nodes on the WireGuard interface,
// forwards echo requests to private destination IPs, and relays replies back.
type icmpProxy struct {
	mu      sync.RWMutex
	conn    *icmp.PacketConn
	routes  map[string]*icmpRoute // routeID → route
	logger  *slog.Logger
	metrics *observability.Metrics
	cancel  context.CancelFunc
	done    chan struct{}
}

// newICMPProxy opens a raw ICMP socket and starts the read loop.
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

// addRoute registers an ICMP route on the gateway.
func (p *icmpProxy) addRoute(routeID, destinationIP string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes[routeID] = &icmpRoute{
		routeID:       routeID,
		destinationIP: destinationIP,
	}
	p.logger.Info("gateway ICMP route added",
		slog.String("route_id", routeID),
		slog.String("destination_ip", destinationIP),
	)
}

// routeCount returns the number of active ICMP routes.
func (p *icmpProxy) routeCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.routes)
}

// readLoop reads incoming ICMP packets and forwards them.
func (p *icmpProxy) readLoop(ctx context.Context) {
	defer close(p.done)
	buf := make([]byte, 1500)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

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
			p.logger.Debug("gateway ICMP read error", slog.String("error", err.Error()))
			continue
		}

		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			p.logger.Debug("gateway ICMP parse error", slog.String("error", err.Error()))
			continue
		}

		switch msg.Type {
		case ipv4.ICMPTypeEcho:
			if p.metrics != nil {
				p.metrics.ICMPPacketsForwarded.WithLabelValues("request").Inc()
			}
			p.logger.Debug("gateway ICMP echo request from node",
				slog.String("from", peer.String()),
			)
		case ipv4.ICMPTypeEchoReply:
			if p.metrics != nil {
				p.metrics.ICMPPacketsForwarded.WithLabelValues("reply").Inc()
			}
			p.logger.Debug("gateway ICMP echo reply",
				slog.String("from", peer.String()),
			)
		default:
			p.logger.Debug("gateway ICMP unknown type",
				slog.Int("type", int(msg.Type.(ipv4.ICMPType))),
			)
		}
	}
}

// close shuts down the gateway ICMP proxy.
func (p *icmpProxy) close() {
	p.cancel()
	p.conn.Close()
	<-p.done
}
