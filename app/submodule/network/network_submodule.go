package network

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	blocks "github.com/ipfs/go-block-format"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log"

	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/filecoin-project/venus/pkg/net"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/host"
	p2pmetrics "github.com/libp2p/go-libp2p-core/metrics"
	smux "github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	mplex "github.com/libp2p/go-libp2p-mplex"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	yamux "github.com/libp2p/go-libp2p-yamux"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

var networkLogger = logging.Logger("network_module")

// NetworkSubmodule enhances the `Node` with networking capabilities.
type NetworkSubmodule struct { //nolint
	NetworkName string

	Host host.Host

	// Router is a router from IPFS
	Router routing.Routing

	Pubsub *libp2pps.PubSub

	// TODO: split chain bitswap from storage bitswap (issue: ???)
	Bitswap exchange.Interface

	Network *net.Network

	GraphExchange graphsync.GraphExchange

	blockstore blockstoreutil.Blockstore
	PeerMgr    net.IPeerMgr
	//data transfer
	DataTransfer     datatransfer.Manager
	DataTransferHost dtnet.DataTransferNetwork
}

//API create a new network implement
func (networkSubmodule *NetworkSubmodule) API() apiface.INetwork {
	return &networkAPI{network: networkSubmodule}
}

func (networkSubmodule *NetworkSubmodule) Stop(ctx context.Context) {
	if err := networkSubmodule.Host.Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}
}

type networkConfig interface {
	GenesisCid() cid.Cid
	OfflineMode() bool
	IsRelay() bool
	Libp2pOpts() []libp2p.Option
}

type networkRepo interface {
	Config() *config.Config
	ChainDatastore() repo.Datastore
	Path() (string, error)
}

// NewNetworkSubmodule creates a new network submodule.
func NewNetworkSubmodule(ctx context.Context, config networkConfig, repo networkRepo, blockstore *blockstore.BlockstoreSubmodule) (*NetworkSubmodule, error) {
	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	libP2pOpts := append(config.Libp2pOpts(), libp2p.BandwidthReporter(bandwidthTracker), makeSmuxTransportOption(true))

	var networkName string
	var err error
	if !repo.Config().NetworkParams.DevNet {
		networkName = "testnetnet"
	} else {
		networkName, err = retrieveNetworkName(ctx, config.GenesisCid(), blockstore.CborStore)
		if err != nil {
			return nil, err
		}
	}

	// peer manager
	bootNodes, err := net.ParseAddresses(ctx, repo.Config().Bootstrap.Addresses)
	if err != nil {
		return nil, err
	}

	// set up host
	var peerHost host.Host
	var router routing.Routing
	var pubsubMessageSigning bool
	var peerMgr net.IPeerMgr
	// if !config.OfflineMode() {
	makeDHT := func(h host.Host) (routing.Routing, error) {
		mode := dht.ModeAuto
		opts := []dht.Option{dht.Mode(mode),
			dht.Datastore(repo.ChainDatastore()),
			dht.ProtocolPrefix(net.FilecoinDHT(networkName)),
			dht.QueryFilter(dht.PublicQueryFilter),
			dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			dht.DisableProviders(),
			dht.BootstrapPeers(bootNodes...),
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

	peerHost, err = buildHost(ctx, config, libP2pOpts, repo, makeDHT)
	if err != nil {
		return nil, err
	}
	// require message signing in online mode when we have priv key
	pubsubMessageSigning = true

	period, err := time.ParseDuration(repo.Config().Bootstrap.Period)
	if err != nil {
		return nil, err
	}

	peerMgr, err = net.NewPeerMgr(peerHost, router.(*dht.IpfsDHT), period, bootNodes)
	if err != nil {
		return nil, err
	}

	// do NOT start `peerMgr` in `offline` mode
	if !config.OfflineMode() {
		go peerMgr.Run(ctx)
	}
	// } else {
	// 	router = offroute.NewOfflineRouter(repo.ChainDatastore(), validator)
	// 	peerHost = rhost.Wrap(NewNoopLibP2PHost(), router)
	// 	pubsubMessageSigning = false
	// 	peerMgr = &net.MockPeerMgr{}
	// }

	// Set up libp2p network
	// The gossipsub heartbeat timeout needs to be set sufficiently low
	// to enable publishing on first connection.  The default of one
	// second is not acceptable for tests.
	libp2pps.GossipSubHeartbeatInterval = 100 * time.Millisecond
	options := []libp2pps.Option{
		// Gossipsubv1.1 configuration
		libp2pps.WithFloodPublish(true),

		//  buffer, 32 -> 10K
		libp2pps.WithValidateQueueSize(10 << 10),
		//  worker, 1x cpu -> 2x cpu
		libp2pps.WithValidateWorkers(runtime.NumCPU() * 2),
		//  goroutine, 8K -> 16K
		libp2pps.WithValidateThrottle(16 << 10),

		libp2pps.WithMessageSigning(pubsubMessageSigning),
		libp2pps.WithDiscovery(&discovery.NoopDiscovery{}),
	}
	gsub, err := libp2pps.NewGossipSub(ctx, peerHost, options...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up network")
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
	//sc := storedcounter.New(repo.ChainDatastore(), datastore.NewKey("/datatransfer/api/counter"))
	dtNet := dtnet.NewFromLibp2pHost(peerHost)
	dtDs := namespace.Wrap(repo.ChainDatastore(), datastore.NewKey("/datatransfer/api/transfers"))
	transport := dtgstransport.NewTransport(peerHost.ID(), gsync)

	repoPath, err := repo.Path()
	if err != nil {
		return nil, err
	}

	dirPath := filepath.Join(repoPath, "data-transfer")
	_ = os.MkdirAll(dirPath, 0777) //todo fix for test
	dt, err := dtimpl.NewDataTransfer(dtDs, dirPath, dtNet, transport)
	if err != nil {
		return nil, err
	}
	// build network
	network := net.New(peerHost, net.NewRouter(router), bandwidthTracker)

	// build the network submdule
	return &NetworkSubmodule{
		NetworkName:      networkName,
		Host:             peerHost,
		Router:           router,
		Pubsub:           gsub,
		Bitswap:          bswap,
		GraphExchange:    gsync,
		Network:          network,
		DataTransfer:     dt,
		DataTransferHost: dtNet,
		PeerMgr:          peerMgr,
		blockstore:       blockstore.Blockstore,
	}, nil
}

func (networkSubmodule *NetworkSubmodule) FetchMessagesByCids(
	ctx context.Context,
	cids []cid.Cid,
) ([]*types.UnsignedMessage, error) {
	out := make([]*types.UnsignedMessage, len(cids))
	blks, err := networkSubmodule.fetchCids(ctx, cids)
	if err != nil {
		return nil, err
	}
	for index, blk := range blks {
		var msg types.UnsignedMessage
		err := msg.UnmarshalCBOR(bytes.NewReader(blk.RawData()))
		if err != nil {
			return nil, err
		}
		out[index] = &msg
	}
	return out, nil
}

func (networkSubmodule *NetworkSubmodule) FetchSignedMessagesByCids(
	ctx context.Context,
	cids []cid.Cid,
) ([]*types.SignedMessage, error) {
	out := make([]*types.SignedMessage, len(cids))
	blks, err := networkSubmodule.fetchCids(ctx, cids)
	if err != nil {
		return nil, err
	}
	for index, blk := range blks {
		var msg types.SignedMessage
		err := msg.UnmarshalCBOR(bytes.NewReader(blk.RawData()))
		if err != nil {
			return nil, err
		}
		out[index] = &msg
	}
	return out, nil
}

func (networkSubmodule *NetworkSubmodule) fetchCids(
	ctx context.Context,
	cids []cid.Cid,
) ([]blocks.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cidIndex := make(map[cid.Cid]int)
	for i, c := range cids {
		cidIndex[c] = i
	}

	if len(cids) != len(cidIndex) {
		return nil, fmt.Errorf("duplicate CIDs in fetchCids input")
	}

	msgBlocks := make([]blocks.Block, len(cids))
	srv := bserv.New(networkSubmodule.blockstore, networkSubmodule.Bitswap)
	for block := range srv.GetBlocks(ctx, cids) {
		ix, ok := cidIndex[block.Cid()]
		if !ok {
			// Ignore duplicate/unexpected blocks. This shouldn't
			// happen, but we can be safe.
			networkLogger.Errorw("received duplicate/unexpected block when syncing", "cid", block.Cid())
			continue
		}

		// Record that we've received the block.
		delete(cidIndex, block.Cid())
		msgBlocks[ix] = block
	}

	if len(cidIndex) > 0 {
		err := ctx.Err()
		if err == nil {
			err = fmt.Errorf("failed to fetch %d messages for unknown reasons", len(cidIndex))
		}
		return nil, err
	}

	return msgBlocks, nil
}

func retrieveNetworkName(ctx context.Context, genCid cid.Cid, cborStore cbor.IpldStore) (string, error) {
	var genesis types.BlockHeader
	err := cborStore.Get(ctx, genCid, &genesis)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get block %s", genCid.String())
	}

	return appstate.NewView(cborStore, genesis.ParentStateRoot).InitNetworkName(ctx)
}

// address determines if we are publically dialable.  If so use public
// address, if not configure node to announce relay address.
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
			libp2p.EnableNATService(),
		)
		if err != nil {
			return nil, err
		}
		return relayHost, nil
	}
	return libp2p.New(
		ctx,
		libp2p.UserAgent("venus"),
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
			networkLogger.Warnf("unknown or duplicate muxer in LIBP2P_MUX_PREFS: %s", id)
			continue
		}
		delete(muxers, id)
		opts = append(opts, libp2p.Muxer(id, tpt))
	}

	return libp2p.ChainOptions(opts...)
}
