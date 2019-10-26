package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
)

// DiscoverySubmodule enhances the `Node` with peer discovery capabilities.
type DiscoverySubmodule struct {
	Bootstrapper *discovery.Bootstrapper

	// PeerTracker maintains a list of peers.
	PeerTracker *discovery.PeerTracker

	// HelloHandler handle peer connections for the "hello" protocol.
	HelloHandler *discovery.HelloProtocolHandler
}

// Start starts the discovery submodule for a node.
func (m *DiscoverySubmodule) Start(node *Node) error {
	// Start bootstrapper.
	m.Bootstrapper.Start(context.Background())

	// Register peer tracker disconnect function with network.
	m.PeerTracker.RegisterDisconnect(node.Network.host.Network())

	// Start up 'hello' handshake service
	peerDiscoveredCallback := func(ci *block.ChainInfo) {
		m.PeerTracker.Track(ci)
		err := node.Chain.SyncDispatch.SendHello(ci)
		if err != nil {
			log.Errorf("error receiving chain info from hello %s: %s", ci, err)
			return
		}
		// For now, consider the initial bootstrap done after the syncer has (synchronously)
		// processed the chain up to the head reported by the first peer to respond to hello.
		// This is an interim sequence until a secure network bootstrap is implemented:
		// https://github.com/filecoin-project/go-filecoin/issues/2674.
		// For now, we trust that the first node to respond will be a configured bootstrap node
		// and that we trust that node to inform us of the chain head.
		// TODO: when the syncer rejects too-far-ahead blocks received over pubsub, don't consider
		// sync done until it's caught up enough that it will accept blocks from pubsub.
		// This might require additional rounds of hello.
		// See https://github.com/filecoin-project/go-filecoin/issues/1105
		node.Chain.ChainSynced.Done()
	}

	// chain head callback
	chainHeadCallback := func() (block.TipSet, error) {
		return node.Chain.State.GetTipSet(node.Chain.State.Head())
	}

	// Register the "hello" protocol with the network
	m.HelloHandler.Register(peerDiscoveredCallback, chainHeadCallback)

	return nil
}
