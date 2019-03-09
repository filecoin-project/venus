package net

import (
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-metrics"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/net/pubsub"
)

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
	*pubsub.Subscriber
	*pubsub.Publisher
	metrics.Reporter
	*Router
}

// New returns a new Network
func New(
	host host.Host,
	publisher *pubsub.Publisher,
	subscriber *pubsub.Subscriber,
	router *Router,
	reporter metrics.Reporter,
) *Network {
	return &Network{
		host:       host,
		Publisher:  publisher,
		Reporter:   reporter,
		Router:     router,
		Subscriber: subscriber,
	}
}

// GetPeerID gets the current peer id from libp2p-host
func (network *Network) GetPeerID() peer.ID {
	return network.host.ID()
}

// GetBandwidthStats gets stats on the current bandwidth usage of the network
func (network *Network) GetBandwidthStats() metrics.Stats {
	return network.Reporter.GetBandwidthTotals()
}
