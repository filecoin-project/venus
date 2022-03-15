package discovery

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/util/moresync"
)

var log = logging.Logger("discover_module") // nolint

// DiscoverySubmodule enhances the `Node` with peer discovery capabilities.
type DiscoverySubmodule struct { //nolint
	Bootstrapper   *discovery.Bootstrapper
	BootstrapReady *moresync.Latch

	// PeerTracker maintains a list of peers.
	PeerTracker *discovery.PeerTracker

	// HelloHandler handle peer connections for the "hello" protocol.
	HelloHandler *discovery.HelloProtocolHandler
	// HelloHandler handle peer connections for the "hello" protocol.
	ExchangeHandler        exchange.Server
	ExchangeClient         exchange.Client
	host                   host.Host
	PeerDiscoveryCallbacks []discovery.PeerDiscoveredCallback
	TipSetLoader           discovery.GetTipSetFunc
}

type discoveryConfig interface {
	GenesisCid() cid.Cid
	Repo() repo.Repo
}

// NewDiscoverySubmodule creates a new discovery submodule.
func NewDiscoverySubmodule(ctx context.Context,
	discoverCfg discoveryConfig,
	network *network.NetworkSubmodule,
	chainStore *chain.Store,
	messageStore *chain.MessageStore,
) (*DiscoverySubmodule, error) {
	periodStr := discoverCfg.Repo().Config().Bootstrap.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// bootstrapper maintains connections to some subset of addresses
	ba := discoverCfg.Repo().Config().Bootstrap.Addresses
	bpi, err := net.PeerAddrsToAddrInfo(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}

	minPeerThreshold := discoverCfg.Repo().Config().Bootstrap.MinPeerThreshold

	exchangeClient := exchange.NewClient(network.Host, network.PeerMgr)
	// create a bootstrapper
	bootstrapper := discovery.NewBootstrapper(bpi, network.Host, network.Host.Network(), network.Router, minPeerThreshold, period)

	// set up peer tracking
	peerTracker := discovery.NewPeerTracker(network.Host.ID())

	bootStrapReady := moresync.NewLatch(uint(minPeerThreshold))

	return &DiscoverySubmodule{
		host:            network.Host,
		Bootstrapper:    bootstrapper,
		BootstrapReady:  bootStrapReady,
		PeerTracker:     peerTracker,
		ExchangeClient:  exchangeClient,
		HelloHandler:    discovery.NewHelloProtocolHandler(network.Host, network.PeerMgr, exchangeClient, chainStore, messageStore, discoverCfg.GenesisCid(), time.Duration(discoverCfg.Repo().Config().NetworkParams.BlockDelay)*time.Second),
		ExchangeHandler: exchange.NewServer(chainStore, messageStore, network.Host),
		PeerDiscoveryCallbacks: []discovery.PeerDiscoveredCallback{func(msg *types.ChainInfo) {
			bootStrapReady.Done()
		}},
		TipSetLoader: func() (*types.TipSet, error) {
			return chainStore.GetHead(), nil
		},
	}, nil
}

// Start starts the discovery submodule for a node.  It blocks until bootstrap
// satisfies the configured security conditions.
func (discovery *DiscoverySubmodule) Start(offline bool) error {
	// Register peer tracker disconnect function with network.
	discovery.PeerTracker.RegisterDisconnect(discovery.host.Network())

	// Start up 'hello' handshake service,recv HelloMessage ???
	peerDiscoveredCallback := func(ci *types.ChainInfo) {
		for _, fn := range discovery.PeerDiscoveryCallbacks {
			fn(ci)
		}
	}

	// Register the "hello" protocol with the network
	discovery.HelloHandler.Register(peerDiscoveredCallback, discovery.TipSetLoader)

	//registre exchange protocol
	discovery.ExchangeHandler.Register()

	// Start bootstrapper.
	if !offline {
		discovery.Bootstrapper.Start(context.Background())
		// Wait for bootstrap to be sufficient connected
		discovery.BootstrapReady.Wait()
	}
	log.Info("discovery module start")
	return nil
}

// Stop stops the discovery submodule.
func (discovery *DiscoverySubmodule) Stop() {
	discovery.Bootstrapper.Stop()
}

//API create a discovery api implement
func (discovery *DiscoverySubmodule) API() IDiscovery {
	return &discoveryAPI{discovery: discovery}
}

func (discovery *DiscoverySubmodule) V0API() IDiscovery {
	return &discoveryAPI{discovery: discovery}
}
