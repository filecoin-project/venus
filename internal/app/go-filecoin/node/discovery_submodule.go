package node

import "github.com/filecoin-project/go-filecoin/internal/pkg/discovery"

// DiscoverySubmodule enhances the `Node` with peer discovery capabilities.
type DiscoverySubmodule struct {
	Bootstrapper *discovery.Bootstrapper

	// PeerTracker maintains a list of peers.
	PeerTracker *discovery.PeerTracker

	// HelloHandler handle peer connections for the "hello" protocol.
	HelloHandler *discovery.HelloProtocolHandler
}
