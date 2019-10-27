package submodule

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
)

var log = logging.Logger("node") // nolint: deadcode

// DiscoverySubmodule enhances the `Node` with peer discovery capabilities.
type DiscoverySubmodule struct {
	Bootstrapper *discovery.Bootstrapper

	// PeerTracker maintains a list of peers.
	PeerTracker *discovery.PeerTracker

	// HelloHandler handle peer connections for the "hello" protocol.
	HelloHandler *discovery.HelloProtocolHandler
}

type discoveryConfig interface {
	GenesisCid() cid.Cid
}

// NewDiscoverySubmodule creates a new discovery submodule.
func NewDiscoverySubmodule(ctx context.Context, config discoveryConfig, bsConfig *config.BootstrapConfig, network *NetworkSubmodule) (DiscoverySubmodule, error) {
	periodStr := bsConfig.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return DiscoverySubmodule{}, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// bootstrapper maintains connections to some subset of addresses
	ba := bsConfig.Addresses
	bpi, err := net.PeerAddrsToAddrInfo(ba)
	if err != nil {
		return DiscoverySubmodule{}, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}

	minPeerThreshold := bsConfig.MinPeerThreshold

	// create a bootstrapper
	bootstrapper := discovery.NewBootstrapper(bpi, network.PeerHost, network.PeerHost.Network(), network.Router, minPeerThreshold, period)

	// set up peer tracking
	peerTracker := discovery.NewPeerTracker(network.PeerHost.ID())

	return DiscoverySubmodule{
		Bootstrapper: bootstrapper,
		PeerTracker:  peerTracker,
		HelloHandler: discovery.NewHelloProtocolHandler(network.PeerHost, config.GenesisCid(), network.NetworkName),
	}, nil
}

type discoveryNode interface {
	Network() NetworkSubmodule
	Chain() ChainSubmodule
}

// Start starts the discovery submodule for a node.
func (m *DiscoverySubmodule) Start(node discoveryNode) error {
	// Start bootstrapper.
	m.Bootstrapper.Start(context.Background())

	// Register peer tracker disconnect function with network.
	m.PeerTracker.RegisterDisconnect(node.Network().PeerHost.Network())

	// Start up 'hello' handshake service
	peerDiscoveredCallback := func(ci *block.ChainInfo) {
		m.PeerTracker.Track(ci)
		err := node.Chain().SyncDispatch.SendHello(ci)
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
		node.Chain().ChainSynced.Done()
	}

	// chain head callback
	chainHeadCallback := func() (block.TipSet, error) {
		return node.Chain().State.GetTipSet(node.Chain().State.Head())
	}

	// Register the "hello" protocol with the network
	m.HelloHandler.Register(peerDiscoveredCallback, chainHeadCallback)

	return nil
}

// Stop stops the discovery submodule.
func (m *DiscoverySubmodule) Stop() {
	m.Bootstrapper.Stop()
}
