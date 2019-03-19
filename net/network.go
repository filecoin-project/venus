package net

import (
	"context"
	"fmt"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmU7iTrsNaJfu1Rf5DrvaJLH9wJtQwmP4Dj8oPduprAU68/go-libp2p-swarm"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZZseAa9xcK6tT3YpaShNUAEpyRAoWmUL5ojH3uGNepAc/go-libp2p-metrics"
	"gx/ipfs/QmcNGX5RaxPPCYwa6yGXM1EcUbrreTTinixLcYGmMwf1sx/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"

	"github.com/filecoin-project/go-filecoin/net/pubsub"
)

// SwarmConnectResult represents the data generated when trying to connect to another p2p
// network member.
type SwarmConnectResult struct {
	Peer    string
	Success bool
}

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
	*pubsub.Subscriber
	*pubsub.Publisher
	metrics.Reporter
	*Router
	*ping.PingService
}

// New returns a new Network
func New(
	host host.Host,
	publisher *pubsub.Publisher,
	subscriber *pubsub.Subscriber,
	router *Router,
	reporter metrics.Reporter,
	pinger *ping.PingService,
) *Network {
	return &Network{
		host:        host,
		PingService: pinger,
		Publisher:   publisher,
		Reporter:    reporter,
		Router:      router,
		Subscriber:  subscriber,
	}
}

// GetPeerAddresses gets the current addresses of the node
func (network *Network) GetPeerAddresses() []ma.Multiaddr {
	return network.host.Addrs()
}

// GetPeerID gets the current peer id from libp2p-host
func (network *Network) GetPeerID() peer.ID {
	return network.host.ID()
}

// GetBandwidthStats gets stats on the current bandwidth usage of the network
func (network *Network) GetBandwidthStats() metrics.Stats {
	return network.Reporter.GetBandwidthTotals()
}

// Connect connects to peers at the given addresses
func (network *Network) Connect(ctx context.Context, addrs []string) ([]SwarmConnectResult, error) {
	swrm, ok := network.host.Network().(*swarm.Swarm)
	if !ok {
		return nil, fmt.Errorf("peerhost network was not a swarm")
	}

	pis, err := PeerAddrsToPeerInfos(addrs)
	if err != nil {
		return nil, err
	}

	output := make([]SwarmConnectResult, len(pis))
	for i, pi := range pis {
		swrm.Backoff().Clear(pi.ID)

		output[i].Peer = pi.ID.Pretty()

		if err := network.host.Connect(ctx, pi); err != nil {
			return nil, errors.Wrapf(err, "peer: %s", output[i].Peer)
		}
	}

	return output, nil
}
