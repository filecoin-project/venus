package submodule

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/discovery"
	"github.com/filecoin-project/venus/internal/pkg/net"
	"github.com/filecoin-project/venus/internal/pkg/util/moresync"
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
	// HelloHandler handle peer connections for the "hello" protocol.
	ExchangeHandler exchange.Server
}

type discoveryConfig interface {
	GenesisCid() cid.Cid
}

// NewDiscoverySubmodule creates a new discovery submodule.
func NewDiscoverySubmodule(ctx context.Context,
	config discoveryConfig,
	bsConfig *config.BootstrapConfig,
	network *NetworkSubmodule,
	chainStore *chain.Store,
	messageStore *chain.MessageStore,
) (DiscoverySubmodule, error) {
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
		Bootstrapper:    bootstrapper,
		BootstrapReady:  moresync.NewLatch(uint(minPeerThreshold)),
		PeerTracker:     peerTracker,
		HelloHandler:    discovery.NewHelloProtocolHandler(network.Host, network.PeerMgr, config.GenesisCid(), network.NetworkName),
		ExchangeHandler: exchange.NewServer(chainStore, messageStore, network.Host),
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

	// Start up 'hello' handshake service,recv HelloMessage ???
	peerDiscoveredCallback := func(ci *block.ChainInfo) {
		err := node.Syncer().ChainSyncManager.BlockProposer().SendHello(ci)
		if err != nil {
			log.Errorf("error receiving chain info from hello %s: %s", ci, err)
			return
		}
		m.PeerTracker.Track(ci)
		m.BootstrapReady.Done()
	}

	// chain head callback
	chainHeadCallback := func() (*block.TipSet, error) {
		return node.Chain().State.GetTipSet(node.Chain().State.Head())
	}

	// Register the "hello" protocol with the network
	m.HelloHandler.Register(peerDiscoveredCallback, chainHeadCallback)

	//registre exchange protocol
	m.ExchangeHandler.Register()

	// Wait for bootstrap to be sufficient connected
	m.BootstrapReady.Wait()
	return nil
}

// Stop stops the discovery submodule.
func (m *DiscoverySubmodule) Stop() {
	m.Bootstrapper.Stop()
}
