package submodule

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
)

var log = logging.Logger("node") // nolint: deadcode

// DiscoverySubmodule enhances the `Node` with peer discovery capabilities.
type DiscoverySubmodule struct {
	Bootstrapper   *discovery.Bootstrapper
	BootstrapReady *moresync.Latch

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
	bootstrapper := discovery.NewBootstrapper(bpi, network.Host, network.Host.Network(), network.Router, minPeerThreshold, period)

	// set up peer tracking
	peerTracker := discovery.NewPeerTracker(network.Host.ID())

	return DiscoverySubmodule{
		Bootstrapper:   bootstrapper,
		BootstrapReady: moresync.NewLatch(uint(minPeerThreshold)),
		PeerTracker:    peerTracker,
		HelloHandler:   discovery.NewHelloProtocolHandler(network.Host, config.GenesisCid(), network.NetworkName),
	}, nil
}

type discoveryNode interface {
	Network() NetworkSubmodule
	Chain() ChainSubmodule
	Syncer() SyncerSubmodule
}

// Start starts the discovery submodule for a node.  It blocks until bootstrap
// satisfies the configured security conditions.
func (m *DiscoverySubmodule) Start(node discoveryNode) error {
	// Start bootstrapper.
	m.Bootstrapper.Start(context.Background())

	// Register peer tracker disconnect function with network.
	m.PeerTracker.RegisterDisconnect(node.Network().Host.Network())

	// Start up 'hello' handshake service
	peerDiscoveredCallback := func(ci *block.ChainInfo) {
		m.PeerTracker.Track(ci)
		m.BootstrapReady.Done()
		err := node.Syncer().ChainSyncManager.BlockProposer().SendHello(ci)
		if err != nil {
			log.Errorf("error receiving chain info from hello %s: %s", ci, err)
			return
		}
	}

	// chain head callback
	chainHeadCallback := func() (block.TipSet, error) {
		return node.Chain().State.GetTipSet(node.Chain().State.Head())
	}

	// Register the "hello" protocol with the network
	m.HelloHandler.Register(peerDiscoveredCallback, chainHeadCallback)

	// Wait for bootstrap to be sufficient connected
	m.BootstrapReady.Wait()

	return nil
}

// Stop stops the discovery submodule.
func (m *DiscoverySubmodule) Stop() {
	m.Bootstrapper.Stop()
}
