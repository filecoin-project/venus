package ntwk

import (
	"gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	"gx/ipfs/QmfRHxh8bt4jWLKRhNvR5fn7mFACrQBFLqV4wyoymEExKV/go-libp2p-host"
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
