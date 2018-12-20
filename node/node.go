package node

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"gx/ipfs/QmP2g3VxmC7g7fyRJDj1VJ72KHZbJ9UW24YjSWEj1XTb4H/go-ipfs-exchange-interface"
	dht "gx/ipfs/QmQsw6Nq2A345PqChdtbWVoYbSno7uqRDHwYmYpbPHmZNc/go-libp2p-kad-dht"
	dhtopts "gx/ipfs/QmQsw6Nq2A345PqChdtbWVoYbSno7uqRDHwYmYpbPHmZNc/go-libp2p-kad-dht/opts"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmTxeg52XprLb5j3yaP1nAP3K7sGNkG1pjrHEwBMGFfcf6/go-bitswap"
	bsnet "gx/ipfs/QmTxeg52XprLb5j3yaP1nAP3K7sGNkG1pjrHEwBMGFfcf6/go-bitswap/network"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p"
	rhost "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/host/routed"
	"gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	routing "gx/ipfs/QmZBH87CAPFHcc7cYmBqeSQ98zQ3SX9KUxiYgzPmLWNVKz/go-libp2p-routing"
	dhtprotocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmahxMNoNuSsgQefo9rkpcfRFmQrMN6Q99aztKXf63K7YJ/go-libp2p-host"
	"gx/ipfs/Qmc3BYVGtLs8y3p4uVpARWyo3Xk2oCBFF1AhYUVMPWgwUK/go-libp2p-pubsub"
	libp2ppeer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"
	offroute "gx/ipfs/QmdxhyAwBrnmJFsYPK6tyHh4Yy3gK8gbULErX1dRnpUMqu/go-ipfs-routing/offline"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2"
	api2impl "github.com/filecoin-project/go-filecoin/api2/impl"
	"github.com/filecoin-project/go-filecoin/api2/impl/msgapi"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/filnet"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/hello"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var filecoinDHTProtocol dhtprotocol.ID = "/fil/kad/1.0.0"

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoRepo is returned when the configs repo is nil
	ErrNoRepo = errors.New("must pass a repo option to the node build process")
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
)

type pubSubProcessorFunc func(ctx context.Context, msg *pubsub.Message) error

// Node represents a full Filecoin node.
type Node struct {
	host     host.Host
	PeerHost host.Host

	Consensus   consensus.Protocol
	ChainReader chain.ReadStore
	Syncer      chain.Syncer
	PowerTable  consensus.PowerTableView

	PlumbingAPI api2.Plumbing

	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	HeaviestTipSetCh chan interface{}
	// HeavyTipSetHandled is a hook for tests because pubsub notifications
	// arrive async. It's called after handling a new heaviest tipset.
	HeaviestTipSetHandled func()
	MsgPool               *core.MessagePool

	Wallet *wallet.Wallet

	// Mining stuff.
	MiningScheduler mining.Scheduler
	mining          struct {
		sync.Mutex
		isMining bool
	}
	miningCtx          context.Context
	cancelMining       context.CancelFunc
	miningDoneWg       *sync.WaitGroup
	AddNewlyMinedBlock newBlockFunc
	blockTime          time.Duration

	// Storage Market Interfaces
	StorageMinerClient *storage.Client
	StorageMiner       *storage.Miner

	// Retrieval Interfaces
	RetrievalClient *retrieval.Client
	RetrievalMiner  *retrieval.Miner

	// Network Fields
	PubSub       *pubsub.PubSub
	BlockSub     *pubsub.Subscription
	MessageSub   *pubsub.Subscription
	Ping         *ping.PingService
	HelloSvc     *hello.Handler
	Bootstrapper *filnet.Bootstrapper
	OnlineStore  *hamt.CborIpldStore

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo repo.Repo

	// SectorBuilder is used by the miner to fill and seal sectors.
	sectorBuilder sectorbuilder.SectorBuilder

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// Blockservice is a higher level interface for fetching data
	blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	cborStore *hamt.CborIpldStore

	// A lookup service for mapping on-chain miner address to libp2p identity.
	lookup lookup.PeerLookupService

	// cancelSubscriptionsCtx is a handle to cancel the block and message subscriptions.
	cancelSubscriptionsCtx context.CancelFunc

	// OfflineMode, when true, disables libp2p
	OfflineMode bool

	// Router is a router from IPFS
	Router routing.IpfsRouting
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	Libp2pOpts  []libp2p.Option
	Repo        repo.Repo
	OfflineMode bool
	BlockTime   time.Duration
}

// ConfigOpt is a configuration option for a filecoin node.
type ConfigOpt func(*Config) error

// OfflineMode enables or disables offline mode.
func OfflineMode(offlineMode bool) ConfigOpt {
	return func(c *Config) error {
		c.OfflineMode = offlineMode
		return nil
	}
}

// BlockTime sets the blockTime.
func BlockTime(blockTime time.Duration) ConfigOpt {
	return func(c *Config) error {
		c.BlockTime = blockTime
		return nil
	}
}

// Libp2pOptions returns a node config option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) ConfigOpt {
	return func(nc *Config) error {
		// Quietly having your options overridden leads to hair loss
		if len(nc.Libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		nc.Libp2pOpts = opts
		return nil
	}
}

// New creates a new node.
func New(ctx context.Context, opts ...ConfigOpt) (*Node, error) {
	n := &Config{}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	return n.Build(ctx)
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

// readGenesisCid is a helper function that queries the provided datastore forr
// an entry with the genesisKey cid, returning if found.
func readGenesisCid(ds datastore.Datastore) (cid.Cid, error) {
	bb, err := ds.Get(chain.GenesisKey)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to read genesisKey")
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to cast genesisCid")
	}
	return c, nil
}

// Build instantiates a filecoin Node from the settings specified in the config.
func (nc *Config) Build(ctx context.Context) (*Node, error) {
	if nc.Repo == nil {
		nc.Repo = repo.NewInMemoryRepo()
	}

	bs := bstore.NewBlockstore(nc.Repo.Datastore())

	validator := blankValidator{}

	var innerHost host.Host
	var router routing.IpfsRouting

	if !nc.OfflineMode {
		h, err := libp2p.New(ctx, libp2p.DisableRelay(), libp2p.ChainOptions(nc.Libp2pOpts...))
		if err != nil {
			return nil, err
		}
		r, err := dht.New(
			ctx, h,
			dhtopts.Datastore(nc.Repo.Datastore()),
			dhtopts.NamespacedValidator("v", validator),
			dhtopts.Protocols(filecoinDHTProtocol),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to setup routing")
		}

		innerHost = h
		router = r
	} else {
		innerHost = noopLibP2PHost{}
		router = offroute.NewOfflineRouter(nc.Repo.Datastore(), validator)
	}

	// set up pinger
	pinger := ping.NewPingService(innerHost)

	// use the DHT for routing information
	peerHost := rhost.Wrap(innerHost, router)

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router)
	//nwork := bsnet.NewFromIpfsHost(innerHost, router)
	bswap := bitswap.New(ctx, nwork, bs)
	bservice := bserv.New(bs, bswap)

	cstOnline := hamt.CborIpldStore{Blocks: bservice}
	cstOffline := hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	genCid, err := readGenesisCid(nc.Repo.Datastore())
	if err != nil {
		return nil, err
	}

	var chainStore chain.Store = chain.NewDefaultStore(nc.Repo.ChainDatastore(), &cstOffline, genCid)
	powerTable := &consensus.MarketView{}

	consensus := consensus.NewExpected(&cstOffline, bs, powerTable, genCid, &proofs.RustProver{})

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewDefaultSyncer(&cstOnline, &cstOffline, consensus, chainStore)
	chainReader, ok := chainStore.(chain.ReadStore)
	if !ok {
		return nil, errors.New("failed to cast chain.Store to chain.ReadStore")
	}
	msgPool := core.NewMessagePool()

	// Set up libp2p pubsub
	fsub, err := pubsub.NewFloodSub(ctx, peerHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up pubsub")
	}
	backend, err := wallet.NewDSBackend(nc.Repo.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	sigGetter := mthdsigapi.NewGetter(chainReader)
	msgSender := msgapi.NewSender(nc.Repo, fcWallet, chainReader, msgPool, fsub.Publish)
	msgWaiter := msgapi.NewWaiter(chainReader, bs, &cstOffline)
	plumbingAPI := api2impl.New(sigGetter, msgSender, msgWaiter)

	nd := &Node{
		blockservice: bservice,
		Blockstore:   bs,
		cborStore:    &cstOffline,
		OnlineStore:  &cstOnline,
		Consensus:    consensus,
		ChainReader:  chainReader,
		Syncer:       chainSyncer,
		PowerTable:   powerTable,
		PlumbingAPI:  plumbingAPI,
		Exchange:     bswap,
		host:         peerHost,
		MsgPool:      msgPool,
		OfflineMode:  nc.OfflineMode,
		PeerHost:     peerHost,
		Ping:         pinger,
		PubSub:       fsub,
		Repo:         nc.Repo,
		Wallet:       fcWallet,
		blockTime:    nc.BlockTime,
		Router:       router,
	}

	// Bootstrapping network peers.
	periodStr := nd.Repo.Config().Bootstrap.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// Bootstrapper maintains connections to some subset of addresses
	ba := nd.Repo.Config().Bootstrap.Addresses
	bpi, err := filnet.PeerAddrsToPeerInfos(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}
	minPeerThreshold := nd.Repo.Config().Bootstrap.MinPeerThreshold
	nd.Bootstrapper = filnet.NewBootstrapper(bpi, nd.Host(), nd.Host().Network(), minPeerThreshold, period)

	// On-chain lookup service
	defaultAddressGetter := func() (address.Address, error) {
		return msgapi.GetAndMaybeSetDefaultSenderAddress(nd.Repo, nd.Wallet)
	}
	nd.lookup = lookup.NewChainLookupService(nd.ChainReader, defaultAddressGetter, bs)

	return nd, nil
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := node.ChainReader.Load(ctx); err != nil {
		return err
	}

	// Only set these up, if there is a miner configured.
	if _, err := node.MiningAddress(); err == nil {
		if err := node.setupMining(ctx); err != nil {
			return err
		}
	}

	// Start up 'hello' handshake service
	syncCallBack := func(pid libp2ppeer.ID, cids []cid.Cid, height uint64) {
		// TODO it is possible the syncer interface should be modified to
		// make use of the additional context not used here (from addr + height).
		// To keep things simple for now this info is not used.
		err := node.Syncer.HandleNewBlocks(context.Background(), cids)
		if err != nil {
			log.Info("error handling blocks: %s", types.NewSortedCidSet(cids...).String())
		}
	}
	node.HelloSvc = hello.New(node.Host(), node.ChainReader.GenesisCid(), syncCallBack, node.ChainReader.Head)

	cni := storage.NewClientNodeImpl(
		dag.NewDAGService(node.BlockService()),
		node.Host(),
		node.Lookup(),
		node.CallQueryMethod)
	node.StorageMinerClient = storage.NewClient(cni)

	node.RetrievalClient = retrieval.NewClient(node)
	node.RetrievalMiner = retrieval.NewMiner(node)

	// subscribe to block notifications
	blkSub, err := node.PubSub.Subscribe(BlockTopic)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to blocks topic")
	}
	node.BlockSub = blkSub

	// subscribe to message notifications
	msgSub, err := node.PubSub.Subscribe(msgapi.Topic)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to message topic")
	}
	node.MessageSub = msgSub

	cctx, cancel := context.WithCancel(context.Background())
	node.cancelSubscriptionsCtx = cancel

	go node.handleSubscription(cctx, node.processBlock, "processBlock", node.BlockSub, "BlockSub")
	go node.handleSubscription(cctx, node.processMessage, "processMessage", node.MessageSub, "MessageSub")

	node.HeaviestTipSetHandled = func() {}
	node.HeaviestTipSetCh = node.ChainReader.HeadEvents().Sub(chain.NewHeadTopic)
	go node.handleNewHeaviestTipSet(cctx, node.ChainReader.Head())

	if !node.OfflineMode {
		node.Bootstrapper.Start(context.Background())
	}

	hbs := metrics.NewHeartbeatService(node.Host(), node.Repo.Config().Heartbeat, node.ChainReader.Head)
	go hbs.Start(ctx)
	return nil
}

func (node *Node) setupMining(ctx context.Context) error {
	// configure the underlying sector store, defaulting to the non-test version
	sectorStoreType := proofs.Live
	if os.Getenv("FIL_USE_SMALL_SECTORS") == "true" {
		sectorStoreType = proofs.ProofTest
	}

	// initialize a sector builder
	sectorBuilder, err := initSectorBuilderForNode(ctx, node, sectorStoreType)
	if err != nil {
		return errors.Wrap(err, "failed to initialize sector builder")
	}
	node.sectorBuilder = sectorBuilder

	return nil
}

func (node *Node) setIsMining(isMining bool) {
	node.mining.Lock()
	defer node.mining.Unlock()
	node.mining.isMining = isMining
}

func (node *Node) isMining() bool {
	node.mining.Lock()
	defer node.mining.Unlock()
	return node.mining.isMining
}

func (node *Node) handleNewMiningOutput(miningOutCh <-chan mining.Output) {
	defer func() {
		node.miningDoneWg.Done()
	}()
	for {
		select {
		case <-node.miningCtx.Done():
			return
		case output, ok := <-miningOutCh:
			if !ok {
				return
			}
			if output.Err != nil {
				log.Errorf("problem mining a block: %s", output.Err.Error())
			} else {
				node.miningDoneWg.Add(1)
				go func() {
					if node.isMining() {
						node.AddNewlyMinedBlock(node.miningCtx, output.NewBlock)
					}
					node.miningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewHeaviestTipSet(ctx context.Context, head consensus.TipSet) {
	for {
		select {
		case ts, ok := <-node.HeaviestTipSetCh:
			if !ok {
				return
			}
			newHead, ok := ts.(consensus.TipSet)
			if !ok {
				log.Error("non-tipset published on heaviest tipset channel")
				continue
			}
			if len(newHead) == 0 {
				log.Error("tipset of size 0 published on heaviest tipset channel. ignoring and waiting for a new heaviest tipset.")
				continue
			}

			// When a new best TipSet is promoted we remove messages in it from the
			// message pool (and add them back in if we have a re-org).
			if err := core.UpdateMessagePool(ctx, node.MsgPool, node.CborStore(), head, newHead); err != nil {
				log.Error("error updating message pool for new tipset:", err)
				continue
			}
			head = newHead

			if node.StorageMiner != nil {
				node.StorageMiner.OnNewHeaviestTipSet(newHead)
			}
			node.HeaviestTipSetHandled()
		case <-ctx.Done():
			return
		}
	}
}

func (node *Node) cancelSubscriptions() {
	if node.BlockSub != nil || node.MessageSub != nil {
		node.cancelSubscriptionsCtx()
	}

	if node.BlockSub != nil {
		node.BlockSub.Cancel()
		node.BlockSub = nil
	}

	if node.MessageSub != nil {
		node.MessageSub.Cancel()
		node.MessageSub = nil
	}
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop(ctx context.Context) {
	node.ChainReader.HeadEvents().Unsub(node.HeaviestTipSetCh)
	node.StopMining(ctx)

	node.cancelSubscriptions()
	node.ChainReader.Stop()

	if node.SectorBuilder() != nil {
		if err := node.SectorBuilder().Close(); err != nil {
			fmt.Printf("error closing sector builder: %s\n", err)
		}
		node.sectorBuilder = nil
	}

	if err := node.Host().Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.Bootstrapper.Stop()

	fmt.Println("stopping filecoin :(")
}

type newBlockFunc func(context.Context, *types.Block)

func (node *Node) addNewlyMinedBlock(ctx context.Context, b *types.Block) {
	if err := node.AddNewBlock(ctx, b); err != nil {
		log.Warningf("error adding new mined block: %s. err: %s", b.Cid().String(), err.Error())
	}
}

// MiningAddress returns the address of the mining actor mining on behalf of
// the node.
func (node *Node) MiningAddress() (address.Address, error) {
	addr := node.Repo.Config().Mining.MinerAddress
	if addr == (address.Address{}) {
		return address.Address{}, ErrNoMinerAddress
	}

	return addr, nil
}

// MiningTimes returns the configured time it takes to mine a block, and also
// the mining delay duration, which is currently a fixed fraction of block time.
// Note this is mocked behavior, in production this time is determined by how
// long it takes to generate PoSTs.
func (node *Node) MiningTimes() (time.Duration, time.Duration) {
	mineDelay := node.blockTime / mining.MineDelayConversionFactor
	return node.blockTime, mineDelay
}

// StartMining causes the node to start feeding blocks to the mining worker and initializes
// the SectorBuilder for the mining address.
func (node *Node) StartMining(ctx context.Context) error {
	if node.isMining() {
		return errors.New("Node is already mining")
	}
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}

	// ensure we have a sector builder
	if node.SectorBuilder() == nil {
		if err := node.setupMining(ctx); err != nil {
			return err
		}
	}

	minerOwnerAddr, err := node.MiningOwnerAddress(ctx, minerAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get mining owner address for miner %s", minerAddr)
	}

	blockTime, mineDelay := node.MiningTimes()

	if node.MiningScheduler == nil {
		getStateFromKey := func(ctx context.Context, tsKey string) (state.Tree, error) {
			tsas, err := node.ChainReader.GetTipSetAndState(ctx, tsKey)
			if err != nil {
				return nil, err
			}
			return state.LoadStateTree(ctx, node.CborStore(), tsas.TipSetStateRoot, builtin.Actors)
		}
		getState := func(ctx context.Context, ts consensus.TipSet) (state.Tree, error) {
			return getStateFromKey(ctx, ts.String())
		}
		getWeight := func(ctx context.Context, ts consensus.TipSet) (uint64, uint64, error) {
			parent, err := ts.Parents()
			if err != nil {
				return uint64(0), uint64(0), err
			}
			// TODO handle genesis cid more gracefully
			if parent.Len() == 0 {
				return node.Consensus.Weight(ctx, ts, nil)
			}
			pSt, err := getStateFromKey(ctx, parent.String())
			if err != nil {
				return uint64(0), uint64(0), err
			}
			return node.Consensus.Weight(ctx, ts, pSt)
		}
		worker := mining.NewDefaultWorker(node.MsgPool, getState, getWeight, consensus.ApplyMessages, node.PowerTable, node.Blockstore, node.CborStore(), minerAddr, blockTime)
		node.MiningScheduler = mining.NewScheduler(worker, mineDelay, node.ChainReader.Head)
	}

	// paranoid check
	if !node.MiningScheduler.IsStarted() {
		node.miningCtx, node.cancelMining = context.WithCancel(context.Background())
		outCh, doneWg := node.MiningScheduler.Start(node.miningCtx)

		node.miningDoneWg = doneWg
		node.AddNewlyMinedBlock = node.addNewlyMinedBlock
		node.miningDoneWg.Add(1)
		go node.handleNewMiningOutput(outCh)
	}

	// initialize a storage miner
	storageMiner, err := initStorageMinerForNode(ctx, node)
	if err != nil {
		return errors.Wrap(err, "failed to initialize storage miner")
	}
	node.StorageMiner = storageMiner

	// loop, turning sealing-results into commitSector messages to be included
	// in the chain
	go func() {
		for {
			select {
			case result := <-node.SectorBuilder().SectorSealResults():
				if result.SealingErr != nil {
					log.Errorf("failed to seal sector with id %d: %s", result.SectorID, result.SealingErr.Error())
				} else if result.SealingResult != nil {

					// TODO: determine these algorithmically by simulating call and querying historical prices
					gasPrice := types.NewGasPrice(0)
					gasCost := types.NewGasCost(0)

					val := result.SealingResult
					// This call can fail due to, e.g. nonce collisions, so we retry to make sure we include it,
					// as our miners existence depends on this.
					// TODO: what is the right number of retries?
					_, err := node.SendMessageAndWait(node.miningCtx, 10 /* retries */, minerOwnerAddr, minerAddr, nil, "commitSector", gasPrice, gasCost, val.SectorID, val.CommR[:], val.CommD[:])
					if err != nil {
						log.Errorf("failed to send commitSector message from %s to %s for sector with id %d: %s", minerOwnerAddr, minerAddr, val.SectorID, err)
						continue
					}

					node.StorageMiner.OnCommitmentAddedToChain(val, nil)
				}
			case <-node.miningCtx.Done():
				return
			}
		}
	}()

	// schedules sealing of staged piece-data
	if node.Repo.Config().Mining.AutoSealIntervalSeconds > 0 {
		go func() {
			for {
				select {
				case <-node.miningCtx.Done():
					return
				case <-time.After(time.Duration(node.Repo.Config().Mining.AutoSealIntervalSeconds) * time.Second):
					log.Info("auto-seal has been triggered")
					if err := node.SectorBuilder().SealAllStagedSectors(node.miningCtx); err != nil {
						log.Errorf("scheduler received error from node.SectorBuilder.SealAllStagedSectors (%s) - exiting", err.Error())
						return
					}
				}
			}
		}()
	} else {
		log.Debug("auto-seal is disabled")
	}
	node.setIsMining(true)

	return nil
}

func (node *Node) getLastUsedSectorID(ctx context.Context, minerAddr address.Address) (uint64, error) {
	rets, retCode, err := node.CallQueryMethod(ctx, minerAddr, "getLastUsedSectorID", []byte{}, nil)
	if err != nil {
		return 0, errors.Wrap(err, "failed to call query method getLastUsedSectorID")
	}
	if retCode != 0 {
		return 0, errors.New("non-zero status code returned by getLastUsedSectorID")
	}

	methodSignature, err := node.PlumbingAPI.ActorGetSignature(ctx, minerAddr, "getLastUsedSectorID")
	if err != nil {
		return 0, errors.Wrap(err, "failed to get method signature for getLastUsedSectorID")
	}

	lastUsedSectorIDVal, err := abi.Deserialize(rets[0], methodSignature.Return[0])
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert returned ABI value")
	}
	lastUsedSectorID, ok := lastUsedSectorIDVal.Val.(uint64)
	if !ok {
		return 0, errors.New("failed to convert returned ABI value to uint64")
	}

	return lastUsedSectorID, nil
}

func initSectorBuilderForNode(ctx context.Context, node *Node, sectorStoreType proofs.SectorStoreType) (sectorbuilder.SectorBuilder, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	lastUsedSectorID, err := node.getLastUsedSectorID(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get last used sector id for miner w/address %s", minerAddr.String())
	}

	// TODO: Where should we store the RustSectorBuilder metadata? Currently, we
	// configure the RustSectorBuilder to store its metadata in the staging
	// directory.

	cfg := sectorbuilder.RustSectorBuilderConfig{
		BlockService:     node.blockservice,
		LastUsedSectorID: lastUsedSectorID,
		MetadataDir:      node.Repo.StagingDir(),
		MinerAddr:        minerAddr,
		SealedSectorDir:  node.Repo.SealedDir(),
		SectorStoreType:  sectorStoreType,
		StagedSectorDir:  node.Repo.StagingDir(),
	}

	sb, err := sectorbuilder.NewRustSectorBuilder(cfg)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to initialize sector builder for miner %s", minerAddr.String()))
	}

	return sb, nil
}

func initStorageMinerForNode(ctx context.Context, node *Node) (*storage.Miner, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	miningOwnerAddr, err := node.MiningOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "no mining owner available, skipping storage miner setup")
	}

	miner, err := storage.NewMiner(ctx, minerAddr, miningOwnerAddr, node)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate storage miner")
	}

	return miner, nil
}

// StopMining stops mining on new blocks.
func (node *Node) StopMining(ctx context.Context) {
	node.setIsMining(false)

	if node.cancelMining != nil {
		node.cancelMining()
	}

	if node.miningDoneWg != nil {
		node.miningDoneWg.Wait()
	}

	// TODO: stop node.StorageMiner
}

// NewAddress creates a new account address on the default wallet backend.
func (node *Node) NewAddress() (address.Address, error) {
	return wallet.NewAddress(node.Wallet)
}

// CallQueryMethod calls a method on an actor using the state of the heaviest
// tipset. It doesn't make any changes to the state/blockchain. It is useful
// for interrogating actor state. The caller address is optional; if not
// provided, an address will be chosen from the node's wallet.
func (node *Node) CallQueryMethod(ctx context.Context, to address.Address, method string, args []byte, optFrom *address.Address) (_ [][]byte, _ uint8, err error) {
	ctx = log.Start(ctx, "Node.CallQueryMethod")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	headTs := node.ChainReader.Head()
	tsas, err := node.ChainReader.GetTipSetAndState(ctx, headTs.String())
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve state")
	}
	st, err := state.LoadStateTree(ctx, node.CborStore(), tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve state")
	}
	h, err := headTs.Height()
	if err != nil {
		return nil, 1, errors.Wrap(err, "getting base tipset height")
	}

	fromAddr, err := msgapi.GetAndMaybeSetDefaultSenderAddress(node.Repo, node.Wallet)
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve default sender address")
	}

	if optFrom != nil {
		fromAddr = *optFrom
	}

	vms := vm.NewStorageMap(node.Blockstore)
	return consensus.CallQueryMethod(ctx, st, vms, to, method, args, fromAddr, types.NewBlockHeight(h))
}

// CreateMiner creates a new miner actor for the given account and returns its address.
// It will wait for the the actor to appear on-chain and add set the address to mining.minerAddress in the config.
// TODO: This should live in a MinerAPI or some such. It's here until we have a proper API layer.
func (node *Node) CreateMiner(ctx context.Context, accountAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasCost, pledge uint64, pid libp2ppeer.ID, collateral *types.AttoFIL) (_ *address.Address, err error) {
	// Only create a miner if we don't already have one.
	if _, err := node.MiningAddress(); err != ErrNoMinerAddress {
		return nil, fmt.Errorf("can only have one miner per node")
	}

	ctx = log.Start(ctx, "Node.CreateMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// TODO: make this more streamlined in the wallet
	backend, err := node.Wallet.Find(accountAddr)
	if err != nil {
		return nil, err
	}
	info, err := backend.GetKeyInfo(accountAddr)
	if err != nil {
		return nil, err
	}
	pubkey, err := info.PublicKey()
	if err != nil {
		return nil, err
	}

	smsgCid, err := node.PlumbingAPI.MessageSend(ctx, accountAddr, address.StorageMarketAddress, collateral, gasPrice, gasLimit, "createMiner", big.NewInt(int64(pledge)), pubkey, pid)
	if err != nil {
		return nil, err
	}

	var minerAddress address.Address
	err = node.PlumbingAPI.MessageWait(ctx, smsgCid, func(blk *types.Block, smsg *types.SignedMessage,
		receipt *types.MessageReceipt) error {
		if receipt.ExitCode != uint8(0) {
			return vmErrors.VMExitCodeToError(receipt.ExitCode, storagemarket.Errors)
		}
		minerAddress, err = address.NewFromBytes(receipt.Return[0])
		return err
	})
	if err != nil {
		return nil, err
	}

	err = node.saveMinerAddressToConfig(minerAddress)
	if err != nil {
		return &minerAddress, err
	}

	err = node.setupMining(ctx)

	return &minerAddress, err
}

func (node *Node) saveMinerAddressToConfig(addr address.Address) error {
	r := node.Repo
	newConfig := r.Config()
	newConfig.Mining.MinerAddress = addr

	return r.ReplaceConfig(newConfig)
}

// SendMessageAndWait creates a message, adds it to the mempool and waits for inclusion.
// It will retry upto retries times, if there is a nonce error when including it.
// It returns the deserialized results, or an error.
func (node *Node) SendMessageAndWait(ctx context.Context, retries uint, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasCost, params ...interface{}) (res []interface{}, err error) {
	for i := 0; i < int(retries); i++ {
		log.Debugf("SendMessageAndWait (%s) retry %d/%d", method, i, retries)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			msgCid, err := node.PlumbingAPI.MessageSend(ctx, from, to, val, gasPrice, gasLimit, method, params...)
			if err != nil {
				return nil, errors.Wrap(err, "failed to add message to mempool")
			}

			err = node.PlumbingAPI.MessageWait(
				ctx,
				msgCid,
				func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
					if receipt.ExitCode != uint8(0) {
						return vmErrors.VMExitCodeToError(receipt.ExitCode, miner.Errors)
					}

					signature, err := node.PlumbingAPI.ActorGetSignature(context.Background(), smsg.Message.To, smsg.Message.Method)
					// Note: GetSignature could fail if the To is an empty actor or a transfer. This is likely a bug.
					if err != nil {
						return err
					}
					retValues := make([]interface{}, len(receipt.Return))
					for i := 0; i < len(receipt.Return); i++ {
						val, err := abi.DecodeValues(receipt.Return[i], []abi.Type{signature.Return[i]})
						if err != nil {
							return err
						}
						retValues[i] = abi.FromValues(val)
					}
					return nil
				},
			)

			if err != nil {
				// TODO: expose nonce errors, instead of using strings.
				// TODO: we might want to retry on other errors, add these here when needed.
				if strings.Contains(err.Error(), "nonce too high") || strings.Contains(err.Error(), "nonce too low") {
					log.Warningf("SendMessageAndWait: failed to send message, retrying: %s", err)
					// cleanup message pool
					node.MsgPool.Remove(msgCid)
					continue
				}
				return nil, errors.Wrap(err, "unexpected error")
			}

			return res, nil
		}
	}

	return nil, errors.Wrapf(err, "failed to send message after %d retries", retries)
}

// MiningOwnerAddress returns the owner of the passed in mining address.
// TODO: find a better home for this method
func (node *Node) MiningOwnerAddress(ctx context.Context, miningAddr address.Address) (address.Address, error) {
	res, code, err := node.CallQueryMethod(ctx, miningAddr, "getOwner", nil, nil)
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to getOwner")
	}
	if code != 0 {
		return address.Address{}, fmt.Errorf("failed to getOwner from the miner: exitCode = %d", code)
	}

	return address.NewFromBytes(res[0])
}

// BlockHeight returns the current block height of the chain.
func (node *Node) BlockHeight() (*types.BlockHeight, error) {
	head := node.ChainReader.Head()
	if head == nil {
		return nil, errors.New("invalid nil head")
	}
	height, err := head.Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(height), nil
}

func (node *Node) handleSubscription(ctx context.Context, f pubSubProcessorFunc, fname string, s *pubsub.Subscription, sname string) {
	for {
		pubSubMsg, err := s.Next(ctx)
		if err != nil {
			log.Errorf("%s.Next(): %s", sname, err)
			return
		}

		if err := f(ctx, pubSubMsg); err != nil {
			log.Errorf("%s(): %s", fname, err)
		}
	}
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.host
}

// SectorBuilder returns the nodes sectorBuilder.
func (node *Node) SectorBuilder() sectorbuilder.SectorBuilder {
	return node.sectorBuilder
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *hamt.CborIpldStore {
	return node.cborStore
}

// Lookup returns the nodes lookup service.
func (node *Node) Lookup() lookup.PeerLookupService {
	return node.lookup
}
