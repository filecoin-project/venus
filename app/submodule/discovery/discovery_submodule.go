package discovery

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/libp2p/go-libp2p-core/host"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/config"
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
	ExchangeHandler exchange.Server

	host                   host.Host
	PeerDiscoveryCallbacks []discovery.PeerDiscoveredCallback
	TipSetLoader           discovery.GetTipSetFunc
}

type discoveryConfig interface {
	GenesisCid() cid.Cid
}

// NewDiscoverySubmodule creates a new discovery submodule.
func NewDiscoverySubmodule(ctx context.Context,
	config discoveryConfig,
	bsConfig *config.BootstrapConfig,
	network *network.NetworkSubmodule,
	chainStore *chain.Store,
	messageStore *chain.MessageStore,
) (*DiscoverySubmodule, error) {
	periodStr := bsConfig.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// bootstrapper maintains connections to some subset of addresses
	ba := bsConfig.Addresses
	bpi, err := net.PeerAddrsToAddrInfo(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}

	minPeerThreshold := bsConfig.MinPeerThreshold

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
		HelloHandler:    discovery.NewHelloProtocolHandler(network.Host, network.PeerMgr, config.GenesisCid(), network.NetworkName),
		ExchangeHandler: exchange.NewServer(chainStore, messageStore, network.Host),
		PeerDiscoveryCallbacks: []discovery.PeerDiscoveredCallback{func(ci *block.ChainInfo) {
			peerTracker.Track(ci)
			bootStrapReady.Done()
		}},
		TipSetLoader: func() (*block.TipSet, error) {
			head := chainStore.GetHead()
			return chainStore.GetTipSet(head)
		},
	}, nil
}

// Start starts the discovery submodule for a node.  It blocks until bootstrap
// satisfies the configured security conditions.
func (discovery *DiscoverySubmodule) Start() error {
	// Register peer tracker disconnect function with network.
	discovery.PeerTracker.RegisterDisconnect(discovery.host.Network())

	// Start up 'hello' handshake service,recv HelloMessage ???
	peerDiscoveredCallback := func(ci *block.ChainInfo) {
		for _, fn := range discovery.PeerDiscoveryCallbacks {
			fn(ci)
		}
	}

	// Register the "hello" protocol with the network
	discovery.HelloHandler.Register(peerDiscoveredCallback, discovery.TipSetLoader)

	// Start bootstrapper.
	discovery.Bootstrapper.Start(context.Background())

	//registre exchange protocol
	discovery.ExchangeHandler.Register()

	// Wait for bootstrap to be sufficient connected
	discovery.BootstrapReady.Wait()
	return nil
}

// Stop stops the discovery submodule.
func (discovery *DiscoverySubmodule) Stop() {
	discovery.Bootstrapper.Stop()
}

func (discovery *DiscoverySubmodule) API() *DiscoveryAPI {
	return &DiscoveryAPI{discovery: discovery}
}
