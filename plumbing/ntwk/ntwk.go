package ntwk

import (
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmaoXrM4Z41PD48JY36YqQGKQpLGjyLA2cKcLsES7YddAq/go-libp2p-host"
)

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
}

// NewNetwork returns a new Network
func NewNetwork(host host.Host) *Network {
	return &Network{host: host}
}

// GetPeerID gets the current peer id from libp2p-host
func (network *Network) GetPeerID() peer.ID {
	return network.host.ID()
}
