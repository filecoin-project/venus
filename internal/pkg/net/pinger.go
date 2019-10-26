package net

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

// ErrPingSelf is returned if the pinger is instructed to ping itself.
var ErrPingSelf = errors.New("cannot ping self")

// Pinger wraps a libp2p ping service.  It exists to serve more helpful
// error messages in the case a node is pinging itself.
type Pinger struct {
	*ping.PingService
	self host.Host
}

// NewPinger creates a filecoin pinger provided with a pingService and a PID.
func NewPinger(h host.Host, p *ping.PingService) *Pinger {
	return &Pinger{
		PingService: p,
		self:        h,
	}
}

// Ping connects to other nodes on the network to test connections.  The
// Pinger will error if the caller Pings the Pinger's self id.
func (p *Pinger) Ping(ctx context.Context, pid peer.ID) (<-chan ping.Result, error) {
	if pid == p.self.ID() {
		return nil, ErrPingSelf
	}
	return p.PingService.Ping(ctx, pid), nil
}
