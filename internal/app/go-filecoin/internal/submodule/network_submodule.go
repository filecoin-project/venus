package submodule

import (
	"context"
	"time"

	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	graphsync "github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p"
	autonatsvc "github.com/libp2p/go-libp2p-autonat-svc"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/host"
	p2pmetrics "github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
)

// NetworkSubmodule enhances the `Node` with networking capabilities.
type NetworkSubmodule struct {
	NetworkName string

	Host host.Host

	// Router is a router from IPFS
	Router routing.Routing

	pubsub *libp2pps.PubSub

	// TODO: split chain bitswap from storage bitswap (issue: ???)
	Bitswap exchange.Interface

	Network *net.Network

	GraphExchange graphsync.GraphExchange
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type networkConfig interface {
	GenesisCid() cid.Cid
	OfflineMode() bool
	IsRelay() bool
	Libp2pOpts() []libp2p.Option
}

type networkRepo interface {
	Config() *config.Config
	Datastore() ds.Batching
}

// NewNetworkSubmodule creates a new network submodule.
func NewNetworkSubmodule(ctx context.Context, config networkConfig, repo networkRepo, blockstore *BlockstoreSubmodule) (NetworkSubmodule, error) {
	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	libP2pOpts := append(config.Libp2pOpts(), libp2p.BandwidthReporter(bandwidthTracker))

	networkName, err := retrieveNetworkName(ctx, config.GenesisCid(), blockstore.CborStore)
	if err != nil {
		return NetworkSubmodule{}, err
	}

	// set up host
	var peerHost host.Host
	var router routing.Routing
	validator := blankValidator{}
	var pubsubMessageSigning bool
	if !config.OfflineMode() {
		makeDHT := func(h host.Host) (routing.Routing, error) {
			r, err := dht.New(
				ctx,
				h,
				dhtopts.Datastore(repo.Datastore()),
				dhtopts.NamespacedValidator("v", validator),
				dhtopts.Protocols(net.FilecoinDHT(networkName)),
			)
			if err != nil {
				return nil, errors.Wrap(err, "failed to setup routing")
			}
			router = r
			return r, err
		}

		var err error
		peerHost, err = buildHost(ctx, config, libP2pOpts, repo, makeDHT)
		if err != nil {
			return NetworkSubmodule{}, err
		}
		// require message signing in online mode when we have priv key
		pubsubMessageSigning = true
	} else {
		router = offroute.NewOfflineRouter(repo.Datastore(), validator)
		peerHost = rhost.Wrap(noopLibP2PHost{}, router)
		pubsubMessageSigning = false
	}

	// Set up libp2p network
	// The gossipsub heartbeat timeout needs to be set sufficiently low
	// to enable publishing on first connection.  The default of one
	// second is not acceptable for tests.
	libp2pps.GossipSubHeartbeatInterval = 100 * time.Millisecond
	gsub, err := libp2pps.NewGossipSub(ctx, peerHost, libp2pps.WithMessageSigning(pubsubMessageSigning), libp2pps.WithDiscovery(&discovery.NoopDiscovery{}))
	if err != nil {
		return NetworkSubmodule{}, errors.Wrap(err, "failed to set up network")
	}

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router)
	//nwork := bsnet.NewFromIpfsHost(innerHost, router)
	bswap := bitswap.New(ctx, nwork, blockstore.Blockstore)

	// set up pinger
	pingService := ping.NewPingService(peerHost)

	// set up graphsync
	graphsyncNetwork := gsnet.NewFromLibp2pHost(peerHost)
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsyncimpl.New(ctx, graphsyncNetwork, loader, storer, graphsyncimpl.RejectAllRequestsByDefault())

	// build network
	network := net.New(peerHost, net.NewRouter(router), bandwidthTracker, net.NewPinger(peerHost, pingService))
	// build the network submdule
	return NetworkSubmodule{
		NetworkName:   networkName,
		Host:          peerHost,
		Router:        router,
		pubsub:        gsub,
		Bitswap:       bswap,
		GraphExchange: gsync,
		Network:       network,
	}, nil
}

func retrieveNetworkName(ctx context.Context, genCid cid.Cid, cborStore cbor.IpldStore) (string, error) {
	var genesis block.Block
	err := cborStore.Get(ctx, genCid, &genesis)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get block %s", genCid.String())
	}

	return appstate.NewView(cborStore, genesis.StateRoot.Cid).InitNetworkName(ctx)
}

// buildHost determines if we are publically dialable.  If so use public
// Address, if not configure node to announce relay address.
func buildHost(ctx context.Context, config networkConfig, libP2pOpts []libp2p.Option, repo networkRepo, makeDHT func(host host.Host) (routing.Routing, error)) (host.Host, error) {
	// Node must build a host acting as a libp2p relay.  Additionally it
	// runs the autoNAT service which allows other nodes to check for their
	// own dialability by having this node attempt to dial them.
	makeDHTRightType := func(h host.Host) (routing.PeerRouting, error) {
		return makeDHT(h)
	}

	if config.IsRelay() {
		cfg := repo.Config()
		publicAddr, err := ma.NewMultiaddr(cfg.Swarm.PublicRelayAddress)
		if err != nil {
			return nil, err
		}
		publicAddrFactory := func(lc *libp2p.Config) error {
			lc.AddrsFactory = func(addrs []ma.Multiaddr) []ma.Multiaddr {
				if cfg.Swarm.PublicRelayAddress == "" {
					return addrs
				}
				return append(addrs, publicAddr)
			}
			return nil
		}
		relayHost, err := libp2p.New(
			ctx,
			libp2p.EnableRelay(circuit.OptHop),
			libp2p.EnableAutoRelay(),
			libp2p.Routing(makeDHTRightType),
			publicAddrFactory,
			libp2p.ChainOptions(libP2pOpts...),
		)
		if err != nil {
			return nil, err
		}
		// Set up autoNATService as a streamhandler on the host.
		_, err = autonatsvc.NewAutoNATService(ctx, relayHost)
		if err != nil {
			return nil, err
		}
		return relayHost, nil
	}
	return libp2p.New(
		ctx,
		libp2p.EnableAutoRelay(),
		libp2p.Routing(makeDHTRightType),
		libp2p.ChainOptions(libP2pOpts...),
	)
}
