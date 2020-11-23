package discovery

import (
	"context"
	"time"

	libp2pdisc "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
)

// NoopDiscovery satisfies the discovery interface without doing anything
type NoopDiscovery struct{}

// FindPeers returns a dead channel that is always closed
func (sd *NoopDiscovery) FindPeers(ctx context.Context, ns string, opts ...libp2pdisc.Option) (<-chan peer.AddrInfo, error) { // nolint: staticcheck
	closedCh := make(chan peer.AddrInfo) // nolint: staticcheck
	// the output is immediately closed, discovery requests end immediately
	// Callstack:
	// https://github.com/libp2p/go-libp2p-pubsub/blob/55f4ad6eb98b9e617e46641e7078944781abb54c/discovery.go#L157
	// https://github.com/libp2p/go-libp2p-pubsub/blob/55f4ad6eb98b9e617e46641e7078944781abb54c/discovery.go#L287
	// https://github.com/libp2p/go-libp2p-discovery/blob/master/backoffconnector.go#L52
	close(closedCh)
	return closedCh, nil
}

// Advertise does nothing and returns 1 hour.
func (sd *NoopDiscovery) Advertise(ctx context.Context, ns string, opts ...libp2pdisc.Option) (time.Duration, error) { // nolint: staticcheck
	return time.Hour, nil
}
