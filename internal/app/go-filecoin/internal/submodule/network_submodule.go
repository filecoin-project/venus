package submodule

import (
	"context"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"os"
	"runtime"
	"strings"
	"time"

	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/discovery"
	"github.com/filecoin-project/venus/internal/pkg/net"
	appstate "github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-graphsync"
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
	smux "github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	mplex "github.com/libp2p/go-libp2p-mplex"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	yamux "github.com/libp2p/go-libp2p-yamux"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
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

	PeerMgr net.IPeerMgr
	//data transfer
	DataTransfer     datatransfer.Manager
	DataTransferHost dtnet.DataTransferNetwork
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
	libP2pOpts := append(config.Libp2pOpts(), libp2p.BandwidthReporter(bandwidthTracker), makeSmuxTransportOption(true))

	networkName, err := retrieveNetworkName(ctx, config.GenesisCid(), blockstore.CborStore)
	if err != nil {
		return NetworkSubmodule{}, err
	}

	// set up host
	var peerHost host.Host
	var router routing.Routing
	validator := blankValidator{}
	var pubsubMessageSigning bool
	var peerMgr net.IPeerMgr
	if !config.OfflineMode() {
		makeDHT := func(h host.Host) (routing.Routing, error) {
			mode := dht.ModeServer
			opts := []dht.Option{dht.Mode(mode),
				dht.Datastore(repo.Datastore()),
				dht.NamespacedValidator("v", validator),
				dht.ProtocolPrefix(net.FilecoinDHT(networkName)),
				dht.QueryFilter(dht.PublicQueryFilter),
				dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
				dht.DisableProviders(),
				dht.DisableValues()}
			r, err := dht.New(
				ctx, h, opts...,
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

		//peer manager
		bootNodes, err := net.ParseAddresses(ctx, repo.Config().Bootstrap.Addresses)
		if err != nil {
			return NetworkSubmodule{}, err
		}
		period, err := time.ParseDuration(repo.Config().Bootstrap.Period)
		if err != nil {
			return NetworkSubmodule{}, err
		}

		peerMgr, err = net.NewPeerMgr(peerHost, router.(*dht.IpfsDHT), period, bootNodes)
		if err != nil {
			return NetworkSubmodule{}, err
		}

		go peerMgr.Run(ctx)
	} else {
		router = offroute.NewOfflineRouter(repo.Datastore(), validator)
		peerHost = rhost.Wrap(NewNoopLibP2PHost(), router)
		pubsubMessageSigning = false
		peerMgr = &net.MockPeerMgr{}
	}

	// Set up libp2p network
	// The gossipsub heartbeat timeout needs to be set sufficiently low
	// to enable publishing on first connection.  The default of one
	// second is not acceptable for tests.
	libp2pps.GossipSubHeartbeatInterval = 100 * time.Millisecond
	options := []libp2pps.Option{
		// Gossipsubv1.1 configuration
		libp2pps.WithFloodPublish(true),

		//  校验 buffer 队列, 32 -> 10K
		libp2pps.WithValidateQueueSize(10 << 10),
		// 校验 worker 数量, 1x cpu -> 2x cpu
		libp2pps.WithValidateWorkers(runtime.NumCPU() * 2),
		// 校验 goroutine 数量阈值 8K -> 16K
		libp2pps.WithValidateThrottle(16 << 10),

		libp2pps.WithMessageSigning(pubsubMessageSigning),
		libp2pps.WithDiscovery(&discovery.NoopDiscovery{}),
	}
	gsub, err := libp2pps.NewGossipSub(ctx, peerHost, options...)
	if err != nil {
		return NetworkSubmodule{}, errors.Wrap(err, "failed to set up network")
	}

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router, bsnet.Prefix("/chain"))
	bitswapOptions := []bitswap.Option{bitswap.ProvideEnabled(false)}
	bswap := bitswap.New(ctx, nwork, blockstore.Blockstore, bitswapOptions...)

	// set up graphsync
	graphsyncNetwork := gsnet.NewFromLibp2pHost(peerHost)
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsyncimpl.New(ctx, graphsyncNetwork, loader, storer, graphsyncimpl.RejectAllRequestsByDefault())

	//dataTransger
	sc := storedcounter.New(repo.Datastore(), datastore.NewKey("/datatransfer/client/counter"))
	dtNet := dtnet.NewFromLibp2pHost(peerHost)
	dtDs := namespace.Wrap(repo.Datastore(), datastore.NewKey("/datatransfer/client/transfers"))
	transport := dtgstransport.NewTransport(peerHost.ID(), gsync)
	dt, err := dtimpl.NewDataTransfer(dtDs, dtNet, transport, sc)
	if err != nil {
		return NetworkSubmodule{}, err
	}
	// build network
	network := net.New(peerHost, net.NewRouter(router), bandwidthTracker)

	// build the network submdule
	return NetworkSubmodule{
		NetworkName:      networkName,
		Host:             peerHost,
		Router:           router,
		pubsub:           gsub,
		Bitswap:          bswap,
		GraphExchange:    gsync,
		Network:          network,
		DataTransfer:     dt,
		DataTransferHost: dtNet,
		PeerMgr:          peerMgr,
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
			libp2p.Ping(true),
		)
		if err != nil {
			return nil, err
		}

		// Set up autoNATService as a streamhandler on the host.
		_, err = autonatsvc.NewAutoNATService(ctx, relayHost, true)
		if err != nil {
			return nil, err
		}
		return relayHost, nil
	}
	return libp2p.New(
		ctx,
		libp2p.Routing(makeDHTRightType),
		libp2p.ChainOptions(libP2pOpts...),
		libp2p.Ping(true),
		libp2p.DisableRelay(),
	)
}

func makeSmuxTransportOption(mplexExp bool) libp2p.Option {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	ymxtpt := *yamux.DefaultTransport
	ymxtpt.AcceptBacklog = 512

	if os.Getenv("YAMUX_DEBUG") != "" {
		ymxtpt.LogOutput = os.Stderr
	}

	muxers := map[string]smux.Multiplexer{yamuxID: &ymxtpt}
	if mplexExp {
		muxers[mplexID] = mplex.DefaultTransport
	}

	// Allow muxer preference order overriding
	order := []string{yamuxID, mplexID}
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		order = strings.Fields(prefs)
	}

	opts := make([]libp2p.Option, 0, len(order))
	for _, id := range order {
		tpt, ok := muxers[id]
		if !ok {
			log.Warnf("unknown or duplicate muxer in LIBP2P_MUX_PREFS: %s", id)
			continue
		}
		delete(muxers, id)
		opts = append(opts, libp2p.Muxer(id, tpt))
	}

	return libp2p.ChainOptions(opts...)
}
