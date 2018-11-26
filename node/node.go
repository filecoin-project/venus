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

	"gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	libp2ppeer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	routing "gx/ipfs/QmS4niovD1U6pRjUBXivr1zvvLBqiTKbERjFo994JU7oQS/go-libp2p-routing"
	"gx/ipfs/QmT5K5mHn2KUyCDBntKoojQJAJftNzutxzpYR33w8JdN6M/go-libp2p-floodsub"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmTwzvuH2eYPJLdp3sL4ZsdSwCcqzdwc1Vk9ssVbk25EA2/go-bitswap"
	bsnet "gx/ipfs/QmTwzvuH2eYPJLdp3sL4ZsdSwCcqzdwc1Vk9ssVbk25EA2/go-bitswap/network"
	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	rhost "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p/p2p/host/routed"
	"gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWw71Mz9PXKgYG8ZfTYN7Ax2Zm48Eurbne3wC2y7CKmLz/go-ipfs-exchange-interface"
	offroute "gx/ipfs/QmYYXrfYh14XcN5jhmK31HhdAG85HjHAg5czk3Eb9cGML4/go-ipfs-routing/offline"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dhtprotocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	dht "gx/ipfs/Qmb8TxXJEnL65XnmkEZfGd8mEFU6cxptEP4oCfTvcDusd8/go-libp2p-kad-dht"
	dhtopts "gx/ipfs/Qmb8TxXJEnL65XnmkEZfGd8mEFU6cxptEP4oCfTvcDusd8/go-libp2p-kad-dht/opts"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/filnet"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/hello"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var filecoinDHTProtocol dhtprotocol.ID = "/fil/kad/1.0.0"

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMethod is returned when processing a message that does not have a method.
	ErrNoMethod = errors.New("no method in message")
	// ErrNoRepo is returned when the configs repo is nil
	ErrNoRepo = errors.New("must pass a repo option to the node build process")
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
	// ErrNoDefaultMessageFromAddress is returned when the node's wallet is not configured to have a default address and the wallet contains more than one address.
	ErrNoDefaultMessageFromAddress = errors.New("could not produce a from-address for message sending")
)

type floodSubProcessorFunc func(ctx context.Context, msg *floodsub.Message) error

// Node represents a full Filecoin node.
type Node struct {
	host     host.Host
	PeerHost host.Host

	Consensus   consensus.Protocol
	ChainReader chain.ReadStore
	Syncer      chain.Syncer
	PowerTable  consensus.PowerTableView

	MessageWaiter *message.Waiter
	messageLock   sync.Mutex

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
	PubSub            *floodsub.PubSub
	BlockSub          *floodsub.Subscription
	MessageSub        *floodsub.Subscription
	Ping              *ping.PingService
	HelloSvc          *hello.Handler
	RelayBootstrapper *filnet.Bootstrapper
	Bootstrapper      *filnet.Bootstrapper
	OnlineStore       *hamt.CborIpldStore

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo repo.Repo

	// SectorBuilder is used by the miner to fill and seal sectors.
	sectorBuilder sectorbuilder.SectorBuilder

	// SectorStore is used to manage staging and sealed sectors.
	SectorStore proofs.SectorStore

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
func readGenesisCid(ds datastore.Datastore) (*cid.Cid, error) {
	bb, err := ds.Get(chain.GenesisKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read genesisKey")
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to cast genesisCid")
	}
	return &c, nil
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
		h, err := libp2p.New(ctx, nc.Libp2pOpts...)
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
	consensus := consensus.NewExpected(&cstOffline, bs, powerTable, genCid)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewDefaultSyncer(&cstOnline, &cstOffline, consensus, chainStore)
	chainReader, ok := chainStore.(chain.ReadStore)
	if !ok {
		return nil, errors.New("failed to cast chain.Store to chain.ReadStore")
	}
	messageWaiter := message.NewWaiter(chainReader, bs, &cstOffline)
	msgPool := core.NewMessagePool()

	// Set up libp2p pubsub
	fsub, err := floodsub.NewFloodSub(ctx, peerHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up floodsub")
	}
	backend, err := wallet.NewDSBackend(nc.Repo.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	nd := &Node{
		blockservice:  bservice,
		Blockstore:    bs,
		cborStore:     &cstOffline,
		OnlineStore:   &cstOnline,
		Consensus:     consensus,
		ChainReader:   chainReader,
		Syncer:        chainSyncer,
		PowerTable:    powerTable,
		MessageWaiter: messageWaiter,
		Exchange:      bswap,
		host:          peerHost,
		MsgPool:       msgPool,
		OfflineMode:   nc.OfflineMode,
		PeerHost:      peerHost,
		Ping:          pinger,
		PubSub:        fsub,
		Repo:          nc.Repo,
		Wallet:        fcWallet,
		blockTime:     nc.BlockTime,
		Router:        router,
	}

	// Bootstrapping network peers.
	periodStr := nd.Repo.Config().Bootstrap.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// Relay bootstrapper connects to relays that are always required to be connected
	ra := nd.Repo.Config().Bootstrap.Relays
	rpi, err := filnet.PeerAddrsToPeerInfos(ra)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse relay addresses [%s]", ra)
	}
	nd.RelayBootstrapper = filnet.NewBootstrapper(rpi, nd.Host(), nd.Host().Network(), len(ra), period)

	// Bootstrapper maintains connections to some subset of addresses
	ba := nd.Repo.Config().Bootstrap.Addresses
	bpi, err := filnet.PeerAddrsToPeerInfos(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}
	minPeerThreshold := nd.Repo.Config().Bootstrap.MinPeerThreshold
	nd.Bootstrapper = filnet.NewBootstrapper(bpi, nd.Host(), nd.Host().Network(), minPeerThreshold, period)

	// On-chain lookup service
	nd.lookup = lookup.NewChainLookupService(nd.ChainReader, nd.DefaultSenderAddress, bs)

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
	syncCallBack := func(pid libp2ppeer.ID, cids []*cid.Cid, height uint64) {
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
	msgSub, err := node.PubSub.Subscribe(MessageTopic)
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
		node.RelayBootstrapper.Start(context.Background())
		node.Bootstrapper.Start(context.Background())
	}

	hbs := metrics.NewHeartbeatService(node.Host(), node.Repo.Config().Heartbeat, node.ChainReader.Head)
	go hbs.Start(ctx)
	return nil
}

func (node *Node) setupMining(ctx context.Context) error {
	// initialize a sector store
	sstore := proofs.NewDiskBackedSectorStore(node.Repo.StagingDir(), node.Repo.SealedDir())
	if os.Getenv("FIL_USE_SMALL_SECTORS") == "true" {
		sstore = proofs.NewProofTestSectorStore(node.Repo.StagingDir(), node.Repo.SealedDir())
	}
	node.SectorStore = sstore

	// initialize a sector builder
	sectorBuilder, err := initSectorBuilderForNode(ctx, node)
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
				log.Errorf("Problem mining a block: %s", output.Err.Error())
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
				log.Error("Non-TipSet published on HeaviestTipSetCh")
				continue
			}
			if len(newHead) == 0 {
				log.Error("TipSet of size 0 published on HeaviestTipSetCh. Ignoring and waiting for a new Heaviest TipSet.")
				continue
			}

			// When a new best TipSet is promoted we remove messages in it from the
			// message pool (and add them back in if we have a re-org).
			if err := core.UpdateMessagePool(ctx, node.MsgPool, node.CborStore(), head, newHead); err != nil {
				log.Error("Error updating message pool for new TipSet:", err)
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

	node.RelayBootstrapper.Stop()
	node.Bootstrapper.Stop()

	fmt.Println("stopping filecoin :(")
}

type newBlockFunc func(context.Context, *types.Block)

func (node *Node) addNewlyMinedBlock(ctx context.Context, b *types.Block) {
	if err := node.AddNewBlock(ctx, b); err != nil {
		log.Warningf("Error adding new mined block: %s. err: %s", b.Cid().String(), err.Error())
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
					val := result.SealingResult
					// This call can fail due to, e.g. nonce collisions, so we retry to make sure we include it,
					// as our miners existence depends on this.
					// TODO: what is the right number of retries?
					_, err := node.SendMessageAndWait(node.miningCtx, 10 /* retries */, minerOwnerAddr, minerAddr, nil, "commitSector", val.SectorID, val.CommR[:], val.CommD[:])
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

	methodSignature, err := node.GetSignature(ctx, minerAddr, "getLastUsedSectorID")
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

func initSectorBuilderForNode(ctx context.Context, node *Node) (sectorbuilder.SectorBuilder, error) {
	minerAddr, err := node.MiningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	lastUsedSectorID, err := node.getLastUsedSectorID(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get last used sector id for miner w/address %s", minerAddr.String())
	}

	sb, err := sectorbuilder.Init(node.miningCtx, node.Repo.Datastore(), node.BlockService(), minerAddr, node.SectorStore, lastUsedSectorID)
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

// GetSignature fetches the signature for the given method on the appropriate actor.
func (node *Node) GetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	ctx = log.Start(ctx, "Node.GetSignature")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()
	st, err := node.ChainReader.LatestState(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load state tree")
	}

	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil || actor.Code == nil {
		return nil, errors.Wrap(err, "failed to get actor")
	}

	executable, err := st.GetBuiltinActorCode(actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	if method == "" {
		// this is allowed if it is a transfer only case
		return nil, ErrNoMethod
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}

// NextNonce returns the next nonce for the given address. It checks
// the actor's memory and also scans the message pool for any pending
// messages.
func NextNonce(ctx context.Context, node *Node, address address.Address) (nonce uint64, err error) {
	ctx = log.Start(ctx, "Node.NextNonce")
	defer func() {
		log.SetTag(ctx, "nonce", nonce)
		log.FinishWithErr(ctx, err)
	}()

	st, err := node.ChainReader.LatestState(ctx)
	if err != nil {
		return 0, err
	}

	nonce, err = core.NextNonce(ctx, st, node.MsgPool, address)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}

// newMessageWithNextNonce returns a new types.Message whose
// nonce is set to our best guess at the next appropriate value
// (see NextNonce).
// This function is unsafe. Creating multiple messages from the
// same actor before adding them to the message pool will result
// in duplicate nonces. Use node.SendMessage() instead.
func newMessageWithNextNonce(ctx context.Context, node *Node, from, to address.Address, value *types.AttoFIL, method string, params []byte) (_ *types.Message, err error) {
	ctx = log.Start(ctx, "Node.NewMessageWithNextNonce")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	nonce, err := NextNonce(ctx, node, from)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get next nonce")
	}
	return types.NewMessage(from, to, nonce, value, method, params), nil
}

// NewAddress creates a new account address on the default wallet backend.
func (node *Node) NewAddress() (address.Address, error) {
	backends := node.Wallet.Backends(wallet.DSBackendType)
	if len(backends) == 0 {
		return address.Address{}, fmt.Errorf("missing default ds backend")
	}

	backend := (backends[0]).(*wallet.DSBackend)
	return backend.NewAddress()
}

// WaitForMessage waits for a message to appear on chain.
func (node *Node) WaitForMessage(ctx context.Context, msgCid *cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return node.MessageWaiter.Wait(ctx, msgCid, cb)
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

	fromAddr, err := node.DefaultSenderAddress()
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
func (node *Node) CreateMiner(ctx context.Context, accountAddr address.Address, pledge uint64, pid libp2ppeer.ID, collateral *types.AttoFIL) (_ *address.Address, err error) {
	// Only create a miner if we don't already have one.
	if _, err := node.MiningAddress(); err != ErrNoMinerAddress {
		return nil, fmt.Errorf("Can only have one miner per node")
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

	smsgCid, err := node.SendMessage(ctx, accountAddr, address.StorageMarketAddress, collateral, "createMiner", big.NewInt(int64(pledge)), pubkey, pid)
	if err != nil {
		return nil, err
	}

	var minerAddress address.Address
	err = node.WaitForMessage(ctx, smsgCid, func(blk *types.Block, smsg *types.SignedMessage,
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

// DefaultSenderAddress produces a default address from which to send messages.
func (node *Node) DefaultSenderAddress() (address.Address, error) {
	ret, err := node.defaultWalletAddress()
	if err != nil || ret != (address.Address{}) {
		return ret, err
	}

	if len(node.Wallet.Addresses()) > 0 {
		// TODO: this works for now, but is likely not a great solution.
		// Need to figure out what better behaviour to define in regards
		// to default addresses.
		addr := node.Wallet.Addresses()[0]

		newConfig := node.Repo.Config()
		newConfig.Wallet.DefaultAddress = addr

		if err := node.Repo.ReplaceConfig(newConfig); err != nil {
			return address.Address{}, err
		}

		return addr, nil
	}

	return address.Address{}, ErrNoDefaultMessageFromAddress
}

func (node *Node) defaultWalletAddress() (address.Address, error) {
	addr, err := node.Repo.Config().Get("wallet.defaultAddress")
	if err != nil {
		return address.Address{}, err
	}
	return addr.(address.Address), nil
}

// SendMessageAndWait creates a message, adds it to the mempool and waits for inclusion.
// It will retry upto retries times, if there is a nonce error when including it.
// It returns the deserialized results, or an error.
func (node *Node) SendMessageAndWait(ctx context.Context, retries uint, from, to address.Address, val *types.AttoFIL, method string, params ...interface{}) (res []interface{}, err error) {
	for i := 0; i < int(retries); i++ {
		log.Debugf("SendMessageAndWait (%s) retry %d/%d", method, i, retries)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			msgCid, err := node.SendMessage(ctx, from, to, val, method, params...)
			if err != nil {
				return nil, errors.Wrap(err, "failed to add message to mempool")
			}

			err = node.WaitForMessage(
				ctx,
				msgCid,
				func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
					if receipt.ExitCode != uint8(0) {
						return vmErrors.VMExitCodeToError(receipt.ExitCode, miner.Errors)
					}

					signature, err := node.GetSignature(context.Background(), smsg.Message.To, smsg.Message.Method)
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

// SendMessage is a convinent helper around adding a new message to the message pool.
func (node *Node) SendMessage(ctx context.Context, from, to address.Address, val *types.AttoFIL, method string, params ...interface{}) (*cid.Cid, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "invalid params")
	}

	// lock to avoid race for message nonce
	node.messageLock.Lock()
	defer node.messageLock.Unlock()

	msg, err := newMessageWithNextNonce(ctx, node, from, to, val, method, encodedParams)
	if err != nil {
		return nil, errors.Wrap(err, "invalid message")
	}

	smsg, err := types.NewSignedMessage(*msg, node.Wallet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign message")
	}

	if err := node.addNewMessage(ctx, smsg); err != nil {
		return nil, errors.Wrap(err, "failed to submit message")
	}

	return smsg.Cid()
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

func (node *Node) handleSubscription(ctx context.Context, f floodSubProcessorFunc, fname string, s *floodsub.Subscription, sname string) {
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
