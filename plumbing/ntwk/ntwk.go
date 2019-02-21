package ntwk

import (
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"
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
