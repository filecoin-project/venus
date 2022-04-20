package network

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	blocks "github.com/ipfs/go-block-format"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	p2pmetrics "github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	yamux "github.com/libp2p/go-libp2p-yamux"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"

	apiwrapper "github.com/filecoin-project/venus/app/submodule/network/v0api"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/repo"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"

	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
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

	Blockstore blockstoreutil.Blockstore
	PeerMgr    net.IPeerMgr
	//data transfer
	DataTransfer     datatransfer.Manager
	DataTransferHost dtnet.DataTransferNetwork
}

//API create a new network implement
func (networkSubmodule *NetworkSubmodule) API() v1api.INetwork {
	return &networkAPI{network: networkSubmodule}
}

func (networkSubmodule *NetworkSubmodule) V0API() v0api.INetwork {
	return &apiwrapper.WrapperV1INetwork{INetwork: &networkAPI{network: networkSubmodule}}
}

func (networkSubmodule *NetworkSubmodule) Stop(ctx context.Context) {
	networkLogger.Infof("closing bitswap")
	if err := networkSubmodule.Bitswap.Close(); err != nil {
		networkLogger.Errorf("error closing bitswap: %s", err.Error())
	}
	networkLogger.Infof("closing host")
	if err := networkSubmodule.Host.Close(); err != nil {
		networkLogger.Errorf("error closing host: %s", err.Error())
	}
	if err := networkSubmodule.Router.(*dht.IpfsDHT).Close(); err != nil {
		networkLogger.Errorf("error closing dht: %s", err.Error())
	}
}

type networkConfig interface {
	GenesisCid() cid.Cid
	OfflineMode() bool
	IsRelay() bool
	Libp2pOpts() []libp2p.Option
	Repo() repo.Repo
}

// NewNetworkSubmodule creates a new network submodule.
func NewNetworkSubmodule(ctx context.Context, config networkConfig) (*NetworkSubmodule, error) {
	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	libP2pOpts := append(config.Libp2pOpts(), libp2p.BandwidthReporter(bandwidthTracker), makeSmuxTransportOption())

	var networkName string
	var err error
	if !config.Repo().Config().NetworkParams.DevNet {
		networkName = "testnetnet"
	} else {
		config.Repo().ChainDatastore()
		networkName, err = retrieveNetworkName(ctx, config.GenesisCid(), cbor.NewCborStore(config.Repo().Datastore()))
		if err != nil {
			return nil, err
		}
	}

	// peer manager
	bootNodes, err := net.ParseAddresses(ctx, config.Repo().Config().Bootstrap.Addresses)
	if err != nil {
		return nil, err
	}

	// set up host
	var peerHost host.Host
	var router routing.Routing
	var pubsubMessageSigning bool
	var peerMgr net.IPeerMgr
	makeDHT := func(h host.Host) (routing.Routing, error) {
		mode := dht.ModeAuto
		opts := []dht.Option{dht.Mode(mode),
			dht.Datastore(config.Repo().ChainDatastore()),
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

	peerHost, err = buildHost(ctx, config, libP2pOpts, config.Repo().Config(), makeDHT)
	if err != nil {
		return nil, err
	}
	// require message signing in online mode when we have priv key
	pubsubMessageSigning = true

	period, err := time.ParseDuration(config.Repo().Config().Bootstrap.Period)
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
	bswap := bitswap.New(ctx, nwork, config.Repo().Datastore(), bitswapOptions...)

	// set up graphsync
	graphsyncNetwork := gsnet.NewFromLibp2pHost(peerHost)
	lsys := storeutil.LinkSystemForBlockstore(config.Repo().Datastore())
	gsync := graphsyncimpl.New(ctx, graphsyncNetwork, lsys, graphsyncimpl.RejectAllRequestsByDefault())

	//dataTransger
	//sc := storedcounter.New(repo.ChainDatastore(), datastore.NewKey("/datatransfer/api/counter"))
	// go-data-transfer protocol retries:
	// 1s, 5s, 25s, 2m5s, 5m x 11 ~= 1 hour
	dtRetryParams := dtnet.RetryParameters(time.Second, 5*time.Minute, 15, 5)
	dtn := dtnet.NewFromLibp2pHost(peerHost, dtRetryParams)

	dtNet := dtnet.NewFromLibp2pHost(peerHost)
	dtDs := namespace.Wrap(config.Repo().ChainDatastore(), datastore.NewKey("/datatransfer/api/transfers"))
	transport := dtgstransport.NewTransport(peerHost.ID(), gsync)

	dt, err := dtimpl.NewDataTransfer(dtDs, dtn, transport)
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
		Blockstore:       config.Repo().Datastore(),
	}, nil
}

func (networkSubmodule *NetworkSubmodule) FetchMessagesByCids(
	ctx context.Context,
	service bserv.BlockService,
	cids []cid.Cid,
) ([]*types.Message, error) {
	out := make([]*types.Message, len(cids))
	err := networkSubmodule.fetchCids(ctx, service, cids, func(idx int, blk blocks.Block) error {
		var msg types.Message
		if err := msg.UnmarshalCBOR(bytes.NewReader(blk.RawData())); err != nil {
			return err
		}
		out[idx] = &msg
		return nil
	})
	return out, err
}

func (networkSubmodule *NetworkSubmodule) FetchSignedMessagesByCids(
	ctx context.Context,
	service bserv.BlockService,
	cids []cid.Cid,
) ([]*types.SignedMessage, error) {
	out := make([]*types.SignedMessage, len(cids))
	err := networkSubmodule.fetchCids(ctx, service, cids, func(idx int, blk blocks.Block) error {
		var msg types.SignedMessage
		if err := msg.UnmarshalCBOR(bytes.NewReader(blk.RawData())); err != nil {
			return err
		}
		out[idx] = &msg
		return nil
	})
	return out, err
}

func (networkSubmodule *NetworkSubmodule) fetchCids(
	ctx context.Context,
	srv bserv.BlockService,
	cids []cid.Cid,
	onfetchOneBlock func(int, blocks.Block) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cidIndex := make(map[cid.Cid]int)
	for i, c := range cids {
		cidIndex[c] = i
	}

	if len(cids) != len(cidIndex) {
		return fmt.Errorf("duplicate CIDs in fetchCids input")
	}

	msgBlocks := make([]blocks.Block, len(cids))
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
		if onfetchOneBlock != nil {
			if err := onfetchOneBlock(ix, block); err != nil {
				return err
			}
		}
	}

	// 'cidIndex' should be 0 here, that means we had fetched all blocks in 'cids'.
	if len(cidIndex) > 0 {
		err := ctx.Err()
		if err == nil {
			err = fmt.Errorf("failed to fetch %d messages for unknown reasons", len(cidIndex))
		}
		return err
	}

	return nil
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
func buildHost(ctx context.Context, config networkConfig, libP2pOpts []libp2p.Option, cfg *config.Config, makeDHT func(host host.Host) (routing.Routing, error)) (host.Host, error) {
	// Node must build a host acting as a libp2p relay.  Additionally it
	// runs the autoNAT service which allows other nodes to check for their
	// own dialability by having this node attempt to dial them.
	makeDHTRightType := func(h host.Host) (routing.PeerRouting, error) {
		return makeDHT(h)
	}

	if config.IsRelay() {
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
			libp2p.EnableRelay(), // TODO ?
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

	opts := []libp2p.Option{
		libp2p.UserAgent("venus"),
		libp2p.Routing(makeDHTRightType),
		libp2p.ChainOptions(libP2pOpts...),
		libp2p.Ping(true),
		libp2p.DisableRelay(),
	}

	return libp2p.New(opts...)
}

func makeSmuxTransportOption() libp2p.Option {
	const yamuxID = "/yamux/1.0.0"

	ymxtpt := *yamux.DefaultTransport
	ymxtpt.AcceptBacklog = 512

	if os.Getenv("YAMUX_DEBUG") != "" {
		ymxtpt.LogOutput = os.Stderr
	}

	return libp2p.Muxer(yamuxID, &ymxtpt)
}
