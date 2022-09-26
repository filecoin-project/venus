package network

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/venus/pkg/net/helloprotocol"

	"github.com/dchest/blake2b"
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
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	p2pmetrics "github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	yamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnet "github.com/filecoin-project/go-data-transfer/network"
	dtgstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/net"
	filexchange "github.com/filecoin-project/venus/pkg/net/exchange"
	"github.com/filecoin-project/venus/pkg/net/peermgr"
	"github.com/filecoin-project/venus/pkg/repo"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/types"

	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var networkLogger = logging.Logger("network_module")

// NetworkSubmodule enhances the `Node` with networking capabilities.
type NetworkSubmodule struct { //nolint
	NetworkName string

	Host    host.Host
	RawHost types.RawHost

	// Router is a router from IPFS
	Router routing.Routing

	Pubsub *libp2pps.PubSub

	// TODO: split chain bitswap from storage bitswap (issue: ???)
	Bitswap exchange.Interface

	Network *net.Network

	GraphExchange graphsync.GraphExchange

	HelloHandler *helloprotocol.HelloProtocolHandler

	PeerMgr        peermgr.IPeerMgr
	ExchangeClient filexchange.Client
	//data transfer
	DataTransfer     datatransfer.Manager
	DataTransferHost dtnet.DataTransferNetwork

	ScoreKeeper *net.ScoreKeeper

	cfg networkConfig
}

// API create a new network implement
func (networkSubmodule *NetworkSubmodule) API() v1api.INetwork {
	return &networkAPI{network: networkSubmodule}
}

func (networkSubmodule *NetworkSubmodule) V0API() v0api.INetwork {
	return &networkAPI{network: networkSubmodule}
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
func NewNetworkSubmodule(ctx context.Context, chainStore *chain.Store,
	messageStore *chain.MessageStore, config networkConfig) (*NetworkSubmodule, error) {
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
	var peerMgr peermgr.IPeerMgr

	rawHost, err := buildHost(ctx, config, libP2pOpts, config.Repo().Config())
	if err != nil {
		return nil, err
	}

	router, err := makeDHT(ctx, rawHost, config, networkName, bootNodes)
	if err != nil {
		return nil, err
	}

	peerHost := routedHost(rawHost, router)
	period, err := time.ParseDuration(config.Repo().Config().Bootstrap.Period)
	if err != nil {
		return nil, err
	}

	peerMgr, err = peermgr.NewPeerMgr(peerHost, router.(*dht.IpfsDHT), period, bootNodes)
	if err != nil {
		return nil, err
	}

	sk := net.NewScoreKeeper()
	gsub, err := net.NewGossipSub(ctx, peerHost, sk, networkName, config.Repo().Config().NetworkParams.DrandSchedule, bootNodes)
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
	network := net.New(peerHost, rawHost, net.NewRouter(router), bandwidthTracker)
	exchangeClient := filexchange.NewClient(peerHost, peerMgr)
	helloHandler := helloprotocol.NewHelloProtocolHandler(peerHost, peerMgr, exchangeClient, chainStore, messageStore, config.GenesisCid(), time.Duration(config.Repo().Config().NetworkParams.BlockDelay)*time.Second)
	// build the network submdule
	return &NetworkSubmodule{
		NetworkName:      networkName,
		Host:             peerHost,
		RawHost:          rawHost,
		Router:           router,
		Pubsub:           gsub,
		Bitswap:          bswap,
		GraphExchange:    gsync,
		ExchangeClient:   exchangeClient,
		Network:          network,
		DataTransfer:     dt,
		DataTransferHost: dtNet,
		PeerMgr:          peerMgr,
		HelloHandler:     helloHandler,
		cfg:              config,
		ScoreKeeper:      sk,
	}, nil
}

func (networkSubmodule *NetworkSubmodule) Start(ctx context.Context) error {
	// do NOT start `peerMgr` in `offline` mode
	if !networkSubmodule.cfg.OfflineMode() {
		go networkSubmodule.PeerMgr.Run(ctx)
	}
	return nil
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
func buildHost(ctx context.Context, config networkConfig, libP2pOpts []libp2p.Option, cfg *config.Config) (types.RawHost, error) {
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
		libp2p.ChainOptions(libP2pOpts...),
		libp2p.Ping(true),
		libp2p.DisableRelay(),
	}

	return libp2p.New(opts...)
}

func makeDHT(ctx context.Context, h types.RawHost, config networkConfig, networkName string, bootNodes []peer.AddrInfo) (routing.Routing, error) {
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

	return r, nil
}

func routedHost(rh types.RawHost, r routing.Routing) host.Host {
	return routedhost.Wrap(rh, r)
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

func HashMsgId(m *pubsub_pb.Message) string {
	hash := blake2b.Sum256(m.Data)
	return string(hash[:])
}
