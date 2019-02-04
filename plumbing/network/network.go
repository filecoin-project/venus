package network

import (
  "gx/ipfs/QmaoXrM4Z41PD48JY36YqQGKQpLGjyLA2cKcLsES7YddAq/go-libp2p-host"
  "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
)

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
}

// NewNetwork returns a new Network
func NewNetwork(host host.Host) *Network {
	return &Network{host: host}
}

// Gets the current peer id from libp2p-host
func (network *Network) GetPeerId() peer.ID {
	return network.host.ID()
}
