package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	ps "github.com/cskr/pubsub"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p"
	autonatsvc "github.com/libp2p/go-libp2p-autonat-svc"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/host"
	p2pmetrics "github.com/libp2p/go-libp2p-core/metrics"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/opts"
	libp2pps "github.com/libp2p/go-libp2p-pubsub"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/flags"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/strgdls"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/hello"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	vmerr "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var log = logging.Logger("node") // nolint: deadcode

var (
	// ErrNoMinerAddress is returned when the node is not configured to have any miner addresses.
	ErrNoMinerAddress = errors.New("no miner addresses configured")
)

type pubSubProcessorFunc func(ctx context.Context, msg pubsub.Message) error

type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey types.TipSetKey) (state.Tree, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}

type nodeChainSyncer interface {
	HandleNewTipset(ctx context.Context, tipsetCids types.TipSetKey) error
}

// Node represents a full Filecoin node.
type Node struct {
	host     host.Host
	PeerHost host.Host

	Consensus   consensus.Protocol
	ChainReader nodeChainReader
	Syncer      nodeChainSyncer
	PowerTable  consensus.PowerTableView

	BlockMiningAPI *block.MiningAPI
	PorcelainAPI   *porcelain.API
	RetrievalAPI   *retrieval.API
	StorageAPI     *storage.API

	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	HeaviestTipSetCh chan interface{}
	// HeavyTipSetHandled is a hook for tests because pubsub notifications
	// arrive async. It's called after handling a new heaviest tipset.
	// Remove this after replacing the tipset "pubsub" with a synchronous event bus:
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	HeaviestTipSetHandled func()

	// Incoming messages for block mining.
	Inbox *core.Inbox
	// Messages sent and not yet mined.
	Outbox *core.Outbox

	Wallet *wallet.Wallet

	// Mining stuff.
	AddNewlyMinedBlock newBlockFunc
	cancelMining       context.CancelFunc
	MiningWorker       mining.Worker
	MiningScheduler    mining.Scheduler
	mining             struct {
		sync.Mutex
		isMining bool
	}
	miningCtx    context.Context
	miningDoneWg *sync.WaitGroup

	// Storage Market Interfaces
	StorageMiner *storage.Miner

	// Retrieval Interfaces
	RetrievalMiner *retrieval.Miner

	// Network Fields
	BlockSub     pubsub.Subscription
	MessageSub   pubsub.Subscription
	HelloSvc     *hello.Handler
	Bootstrapper *net.Bootstrapper

	// Data Storage Fields

	// Repo is the repo this node was created with
	// it contains all persistent artifacts of the filecoin node
	Repo repo.Repo

	// SectorBuilder is used by the miner to fill and seal sectors.
	sectorBuilder sectorbuilder.SectorBuilder

	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher

	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface

	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// Blockservice is a higher level interface for fetching data
	blockservice bserv.BlockService

	// CborStore is a temporary interface for interacting with IPLD objects.
	cborStore *hamt.CborIpldStore

	// cancelSubscriptionsCtx is a handle to cancel the block and message subscriptions.
	cancelSubscriptionsCtx context.CancelFunc

	// OfflineMode, when true, disables libp2p
	OfflineMode bool

	// Router is a router from IPFS
	Router routing.Routing
}

// Config is a helper to aid in the construction of a filecoin node.
type Config struct {
	BlockTime   time.Duration
	Libp2pOpts  []libp2p.Option
	OfflineMode bool
	Verifier    verification.Verifier
	Rewarder    consensus.BlockRewarder
	Repo        repo.Repo
	IsRelay     bool
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

// IsRelay configures node to act as a libp2p relay.
func IsRelay() ConfigOpt {
	return func(c *Config) error {
		c.IsRelay = true
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

// VerifierConfigOption returns a function that sets the verifier to use in the node consensus
func VerifierConfigOption(verifier verification.Verifier) ConfigOpt {
	return func(c *Config) error {
		c.Verifier = verifier
		return nil
	}
}

// RewarderConfigOption returns a function that sets the rewarder to use in the node consensus
func RewarderConfigOption(rewarder consensus.BlockRewarder) ConfigOpt {
	return func(c *Config) error {
		c.Rewarder = rewarder
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

// readGenesisCid is a helper function that queries the provided datastore for
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

// buildHost determines if we are publically dialable.  If so use public
// Address, if not configure node to announce relay address.
func (nc *Config) buildHost(ctx context.Context, makeDHT func(host host.Host) (routing.Routing, error)) (host.Host, error) {
	// Node must build a host acting as a libp2p relay.  Additionally it
	// runs the autoNAT service which allows other nodes to check for their
	// own dialability by having this node attempt to dial them.
	makeDHTRightType := func(h host.Host) (routing.PeerRouting, error) {
		return makeDHT(h)
	}

	if nc.IsRelay {
		cfg := nc.Repo.Config()
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
			libp2p.ChainOptions(nc.Libp2pOpts...),
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
		libp2p.ChainOptions(nc.Libp2pOpts...),
	)
}

// Build instantiates a filecoin Node from the settings specified in the config.
func (nc *Config) Build(ctx context.Context) (*Node, error) {
	if nc.Repo == nil {
		nc.Repo = repo.NewInMemoryRepo()
	}

	bs := bstore.NewBlockstore(nc.Repo.Datastore())

	validator := blankValidator{}

	var peerHost host.Host
	var router routing.Routing

	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	nc.Libp2pOpts = append(nc.Libp2pOpts, libp2p.BandwidthReporter(bandwidthTracker))

	if !nc.OfflineMode {
		makeDHT := func(h host.Host) (routing.Routing, error) {
			r, err := dht.New(
				ctx,
				h,
				dhtopts.Datastore(nc.Repo.Datastore()),
				dhtopts.NamespacedValidator("v", validator),
				dhtopts.Protocols(net.FilecoinDHT),
			)
			if err != nil {
				return nil, errors.Wrap(err, "failed to setup routing")
			}
			router = r
			return r, err
		}

		var err error
		peerHost, err = nc.buildHost(ctx, makeDHT)
		if err != nil {
			return nil, err
		}
	} else {
		router = offroute.NewOfflineRouter(nc.Repo.Datastore(), validator)
		peerHost = rhost.Wrap(noopLibP2PHost{}, router)
	}

	// set up pinger
	pingService := ping.NewPingService(peerHost)

	// setup block validation
	// TODO when #2961 is resolved do the needful here.
	blkValid := consensus.NewDefaultBlockValidator(nc.BlockTime, clock.NewSystemClock())

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router)
	//nwork := bsnet.NewFromIpfsHost(innerHost, router)
	bswap := bitswap.New(ctx, nwork, bs)
	bservice := bserv.New(bs, bswap)
	fetcher := net.NewBitswapFetcher(ctx, bservice, blkValid)

	ipldCborStore := hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	genCid, err := readGenesisCid(nc.Repo.Datastore())
	if err != nil {
		return nil, err
	}

	// set up chainstore
	chainStore := chain.NewStore(nc.Repo.ChainDatastore(), &ipldCborStore, &state.TreeStateLoader{}, genCid)
	chainState := cst.NewChainStateProvider(chainStore, &ipldCborStore)
	powerTable := &consensus.MarketView{}

	// set up processor
	var processor consensus.Processor
	if nc.Rewarder == nil {
		processor = consensus.NewDefaultProcessor()
	} else {
		processor = consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), nc.Rewarder)
	}

	// set up consensus
	var nodeConsensus consensus.Protocol
	if nc.Verifier == nil {
		nodeConsensus = consensus.NewExpected(&ipldCborStore, bs, processor, blkValid, powerTable, genCid, &verification.RustVerifier{}, nc.BlockTime)
	} else {
		nodeConsensus = consensus.NewExpected(&ipldCborStore, bs, processor, blkValid, powerTable, genCid, nc.Verifier, nc.BlockTime)
	}

	// Set up libp2p network
	// TODO PubSub requires strict message signing, disabled for now
	// reference issue: #3124
	fsub, err := libp2pps.NewFloodSub(ctx, peerHost, libp2pps.WithMessageSigning(false))
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up network")
	}

	backend, err := wallet.NewDSBackend(nc.Repo.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewSyncer(nodeConsensus, chainStore, fetcher, chain.Syncing)
	msgPool := core.NewMessagePool(nc.Repo.Config().Mpool, consensus.NewIngestionValidator(chainState, nc.Repo.Config().Mpool))
	inbox := core.NewInbox(msgPool, core.InboxMaxAgeTipsets, chainStore)

	msgQueue := core.NewMessageQueue()
	outboxPolicy := core.NewMessageQueuePolicy(chainStore, core.OutboxMaxAgeRounds)
	msgPublisher := newDefaultMessagePublisher(pubsub.NewPublisher(fsub), net.MessageTopic, msgPool)
	outbox := core.NewOutbox(fcWallet, consensus.NewOutboundMessageValidator(), msgQueue, msgPublisher, outboxPolicy, chainStore, chainState)

	nd := &Node{
		blockservice: bservice,
		Blockstore:   bs,
		cborStore:    &ipldCborStore,
		Consensus:    nodeConsensus,
		ChainReader:  chainStore,
		Syncer:       chainSyncer,
		PowerTable:   powerTable,
		Fetcher:      fetcher,
		Exchange:     bswap,
		host:         peerHost,
		Inbox:        inbox,
		OfflineMode:  nc.OfflineMode,
		Outbox:       outbox,
		PeerHost:     peerHost,
		Repo:         nc.Repo,
		Wallet:       fcWallet,
		Router:       router,
	}

	nd.PorcelainAPI = porcelain.New(plumbing.New(&plumbing.APIDeps{
		Bitswap:       bswap,
		Chain:         chainState,
		Config:        cfg.NewConfig(nc.Repo),
		DAG:           dag.NewDAG(merkledag.NewDAGService(bservice)),
		Deals:         strgdls.New(nc.Repo.DealsDatastore()),
		Expected:      nodeConsensus,
		MsgPool:       msgPool,
		MsgPreviewer:  msg.NewPreviewer(chainStore, &ipldCborStore, bs),
		MsgQueryer:    msg.NewQueryer(chainStore, &ipldCborStore, bs),
		MsgWaiter:     msg.NewWaiter(chainStore, bs, &ipldCborStore),
		Network:       net.New(peerHost, pubsub.NewPublisher(fsub), pubsub.NewSubscriber(fsub), net.NewRouter(router), bandwidthTracker, net.NewPinger(peerHost, pingService)),
		Outbox:        outbox,
		SectorBuilder: nd.SectorBuilder,
		Wallet:        fcWallet,
	}))

	// Bootstrapping network peers.
	periodStr := nd.Repo.Config().Bootstrap.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// Bootstrapper maintains connections to some subset of addresses
	ba := nd.Repo.Config().Bootstrap.Addresses
	bpi, err := net.PeerAddrsToAddrInfo(ba)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}
	minPeerThreshold := nd.Repo.Config().Bootstrap.MinPeerThreshold
	nd.Bootstrapper = net.NewBootstrapper(bpi, nd.Host(), nd.Host().Network(), nd.Router, minPeerThreshold, period)

	return nd, nil
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := metrics.RegisterPrometheusEndpoint(node.Repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if err := metrics.RegisterJaeger(node.host.ID().Pretty(), node.Repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	var err error
	if err = node.ChainReader.Load(ctx); err != nil {
		return err
	}

	// Only set these up if there is a miner configured.
	if _, err := node.miningAddress(); err == nil {
		if err := node.setupMining(ctx); err != nil {
			log.Errorf("setup mining failed: %v", err)
			return err
		}
	}

	// Start up 'hello' handshake service
	syncCallBack := func(pid libp2ppeer.ID, cids []cid.Cid, height uint64) {
		cidSet := types.NewTipSetKey(cids...)
		err := node.Syncer.HandleNewTipset(context.Background(), cidSet)
		if err != nil {
			log.Infof("error handling blocks: %s", cidSet.String())
		}
	}
	node.HelloSvc = hello.New(node.Host(), node.ChainReader.GenesisCid(), syncCallBack, node.PorcelainAPI.ChainHead, node.Repo.Config().Net, flags.Commit)

	err = node.setupProtocols()
	if err != nil {
		return errors.Wrap(err, "failed to set up protocols:")
	}
	node.RetrievalMiner = retrieval.NewMiner(node)

	// subscribe to block notifications
	blkSub, err := node.PorcelainAPI.PubSubSubscribe(net.BlockTopic)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to blocks topic")
	}
	node.BlockSub = blkSub

	// subscribe to message notifications
	msgSub, err := node.PorcelainAPI.PubSubSubscribe(net.MessageTopic)
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
	head, err := node.PorcelainAPI.ChainHead()
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	go node.handleNewHeaviestTipSet(cctx, head)

	if !node.OfflineMode {
		node.Bootstrapper.Start(context.Background())
	}

	if err := node.setupHeartbeatServices(ctx); err != nil {
		return errors.Wrap(err, "failed to start heartbeat services")
	}

	return nil
}

func (node *Node) setupHeartbeatServices(ctx context.Context) error {
	mag := func() address.Address {
		addr, err := node.miningAddress()
		// the only error miningAddress() returns is ErrNoMinerAddress.
		// if there is no configured miner address, simply send a zero
		// address across the wire.
		if err != nil {
			return address.Undef
		}
		return addr
	}

	// start the primary heartbeat service
	if len(node.Repo.Config().Heartbeat.BeatTarget) > 0 {
		hbs := metrics.NewHeartbeatService(node.Host(), node.Repo.Config().Heartbeat, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go hbs.Start(ctx)
	}

	// check if we want to connect to an alert service. An alerting service is a heartbeat
	// service that can trigger alerts based on the contents of heatbeats.
	if alertTarget := os.Getenv("FIL_HEARTBEAT_ALERTS"); len(alertTarget) > 0 {
		ahbs := metrics.NewHeartbeatService(node.Host(), &config.HeartbeatConfig{
			BeatTarget:      alertTarget,
			BeatPeriod:      "10s",
			ReconnectPeriod: "10s",
			Nickname:        node.Repo.Config().Heartbeat.Nickname,
		}, node.PorcelainAPI.ChainHead, metrics.WithMinerAddressGetter(mag))
		go ahbs.Start(ctx)
	}
	return nil
}

func (node *Node) setupMining(ctx context.Context) error {
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
				log.Errorf("stopping mining. error: %s", output.Err.Error())
				node.StopMining(context.Background())
			} else {
				node.miningDoneWg.Add(1)
				go func() {
					if node.IsMining() {
						node.AddNewlyMinedBlock(node.miningCtx, output.NewBlock)
					}
					node.miningDoneWg.Done()
				}()
			}
		}
	}

}

func (node *Node) handleNewHeaviestTipSet(ctx context.Context, head types.TipSet) {
	for {
		select {
		case ts, ok := <-node.HeaviestTipSetCh:
			if !ok {
				return
			}
			newHead, ok := ts.(types.TipSet)
			if !ok {
				log.Error("non-tipset published on heaviest tipset channel")
				continue
			}
			if !newHead.Defined() {
				log.Error("tipset of size 0 published on heaviest tipset channel. ignoring and waiting for a new heaviest tipset.")
				continue
			}

			if err := node.Outbox.HandleNewHead(ctx, head, newHead); err != nil {
				log.Error("updating outbound message queue for new tipset", err)
			}
			if err := node.Inbox.HandleNewHead(ctx, head, newHead); err != nil {
				log.Error("updating message pool for new tipset", err)
			}
			head = newHead

			if node.StorageMiner != nil {
				err := node.StorageMiner.OnNewHeaviestTipSet(newHead)
				if err != nil {
					log.Error(err)
				}
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
	log.Debugf("Got a newly mined block from the mining worker: %s", b)
	if err := node.AddNewBlock(ctx, b); err != nil {
		log.Warningf("error adding new mined block: %s. err: %s", b.Cid().String(), err.Error())
	}
}

// miningAddress returns the address of the mining actor mining on behalf of
// the node.
func (node *Node) miningAddress() (address.Address, error) {
	addr := node.Repo.Config().Mining.MinerAddress
	if addr.Empty() {
		return address.Undef, ErrNoMinerAddress
	}

	return addr, nil
}

// MiningTimes returns the configured time it takes to mine a block, and also
// the mining delay duration, which is currently a fixed fraction of block time.
// Note this is mocked behavior, in production this time is determined by how
// long it takes to generate PoSTs.
func (node *Node) MiningTimes() (time.Duration, time.Duration) {
	blockTime := node.PorcelainAPI.BlockTime()
	mineDelay := blockTime / mining.MineDelayConversionFactor
	return blockTime, mineDelay
}

// StartMining causes the node to start feeding blocks to the mining worker and initializes
// the SectorBuilder for the mining address.
func (node *Node) StartMining(ctx context.Context) error {
	if node.IsMining() {
		return errors.New("Node is already mining")
	}
	minerAddr, err := node.miningAddress()
	if err != nil {
		return errors.Wrap(err, "failed to get mining address")
	}

	// ensure we have a sector builder
	if node.SectorBuilder() == nil {
		if err := node.setupMining(ctx); err != nil {
			return err
		}
	}

	minerOwnerAddr, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get mining owner address for miner %s", minerAddr)
	}

	_, mineDelay := node.MiningTimes()

	if node.MiningWorker == nil {
		if node.MiningWorker, err = node.CreateMiningWorker(ctx); err != nil {
			return err
		}
	}
	if node.MiningScheduler == nil {
		node.MiningScheduler = mining.NewScheduler(node.MiningWorker, mineDelay, node.PorcelainAPI.ChainHead)
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
					gasPrice := types.NewGasPrice(1)
					gasUnits := types.NewGasUnits(300)

					val := result.SealingResult
					// This call can fail due to, e.g. nonce collisions. Our miners existence depends on this.
					// We should deal with this, but MessageSendWithRetry is problematic.
					msgCid, err := node.PorcelainAPI.MessageSend(
						node.miningCtx,
						minerOwnerAddr,
						minerAddr,
						types.ZeroAttoFIL,
						gasPrice,
						gasUnits,
						"commitSector",
						val.SectorID,
						val.CommD[:],
						val.CommR[:],
						val.CommRStar[:],
						val.Proof[:],
					)
					if err != nil {
						log.Errorf("failed to send commitSector message from %s to %s for sector with id %d: %s", minerOwnerAddr, minerAddr, val.SectorID, err)
						continue
					}

					node.StorageMiner.OnCommitmentSent(val, msgCid, nil)
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

func initSectorBuilderForNode(ctx context.Context, node *Node) (sectorbuilder.SectorBuilder, error) {
	minerAddr, err := node.miningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	sectorSize, err := node.PorcelainAPI.MinerGetSectorSize(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sector size for miner w/address %s", minerAddr.String())
	}

	lastUsedSectorID, err := node.PorcelainAPI.MinerGetLastCommittedSectorID(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get last used sector id for miner w/address %s", minerAddr.String())
	}

	// TODO: Currently, weconfigure the RustSectorBuilder to store its
	// metadata in the staging directory, it should be in its own directory.
	//
	// Tracked here: https://github.com/filecoin-project/rust-fil-proofs/issues/402
	repoPath, err := node.Repo.Path()
	if err != nil {
		return nil, err
	}
	sectorDir, err := paths.GetSectorPath(node.Repo.Config().SectorBase.RootDir, repoPath)
	if err != nil {
		return nil, err
	}

	stagingDir, err := paths.StagingDir(sectorDir)
	if err != nil {
		return nil, err
	}

	sealedDir, err := paths.SealedDir(sectorDir)
	if err != nil {
		return nil, err
	}
	cfg := sectorbuilder.RustSectorBuilderConfig{
		BlockService:     node.blockservice,
		LastUsedSectorID: lastUsedSectorID,
		MetadataDir:      stagingDir,
		MinerAddr:        minerAddr,
		SealedSectorDir:  sealedDir,
		StagedSectorDir:  stagingDir,
		SectorClass:      types.NewSectorClass(sectorSize),
	}

	sb, err := sectorbuilder.NewRustSectorBuilder(cfg)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to initialize sector builder for miner %s", minerAddr.String()))
	}

	return sb, nil
}

func initStorageMinerForNode(ctx context.Context, node *Node) (*storage.Miner, error) {
	minerAddr, err := node.miningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node's mining address")
	}

	ownerAddress, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "no mining owner available, skipping storage miner setup")
	}
	workerAddress := ownerAddress

	sectorSize, err := node.PorcelainAPI.MinerGetSectorSize(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch miner's sector size")
	}

	prover := storage.NewProver(minerAddr, workerAddress, sectorSize, node.PorcelainAPI, node.PorcelainAPI)
	miner, err := storage.NewMiner(minerAddr, ownerAddress, workerAddress, prover, sectorSize, node, node.Repo.DealsDatastore(), node.PorcelainAPI)
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

func (node *Node) handleSubscription(ctx context.Context, f pubSubProcessorFunc, fname string, s pubsub.Subscription, sname string) {
	for {
		pubSubMsg, err := s.Next(ctx)
		if err != nil {
			log.Errorf("%s.Next(): %s", sname, err)
			return
		}

		if err := f(ctx, pubSubMsg); err != nil {
			if vmerr.ShouldRevert(err) {
				log.Infof("%s(): %s", fname, err)
			} else if err != context.Canceled {
				log.Errorf("%s(): %s", fname, err)
			}
		}
	}
}

// setupProtocols creates protocol clients and miners, then sets the node's APIs
// for each
func (node *Node) setupProtocols() error {
	_, mineDelay := node.MiningTimes()
	blockMiningAPI := block.New(
		node.AddNewBlock,
		node.ChainReader,
		node.IsMining,
		mineDelay,
		node.StartMining,
		node.StopMining,
		node.CreateMiningWorker)

	node.BlockMiningAPI = &blockMiningAPI

	// set up retrieval client and api
	retapi := retrieval.NewAPI(retrieval.NewClient(node.host, node.PorcelainAPI))
	node.RetrievalAPI = &retapi

	// set up storage client and api
	smc := storage.NewClient(node.host, node.PorcelainAPI)
	smcAPI := storage.NewAPI(smc)
	node.StorageAPI = &smcAPI
	return nil
}

// CreateMiningWorker creates a mining.Worker for the node using the configured
// getStateTree, getWeight, and getAncestors functions for the node
func (node *Node) CreateMiningWorker(ctx context.Context) (mining.Worker, error) {
	processor := consensus.NewDefaultProcessor()

	minerAddr, err := node.miningAddress()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mining address")
	}

	minerWorker, err := node.PorcelainAPI.MinerGetWorker(ctx, minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "could not get key from miner actor")
	}

	minerOwnerAddr, err := node.PorcelainAPI.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		log.Errorf("could not get owner address of miner actor")
		return nil, err
	}
	return mining.NewDefaultWorker(
		node.Inbox.Pool(), node.getStateTree, node.getWeight, node.getAncestors, processor, node.PowerTable,
		node.Blockstore, minerAddr, minerOwnerAddr, minerWorker, node.Wallet,
		node.PorcelainAPI), nil
}

// getStateTree is the default GetStateTree function for the mining worker.
func (node *Node) getStateTree(ctx context.Context, ts types.TipSet) (state.Tree, error) {
	return node.ChainReader.GetTipSetState(ctx, ts.Key())
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts types.TipSet) (uint64, error) {
	parent, err := ts.Parents()
	if err != nil {
		return uint64(0), err
	}
	// TODO handle genesis cid more gracefully
	if parent.Len() == 0 {
		return node.Consensus.Weight(ctx, ts, nil)
	}
	pSt, err := node.ChainReader.GetTipSetState(ctx, parent)
	if err != nil {
		return uint64(0), err
	}
	return node.Consensus.Weight(ctx, ts, pSt)
}

// getAncestors is the default GetAncestors function for the mining worker.
func (node *Node) getAncestors(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
	ancestorHeight := types.NewBlockHeight(consensus.AncestorRoundsNeeded)
	return chain.GetRecentAncestors(ctx, ts, node.ChainReader, newBlockHeight, ancestorHeight, sampling.LookbackParameter)
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

// IsMining returns a boolean indicating whether the node is mining blocks.
func (node *Node) IsMining() bool {
	node.mining.Lock()
	defer node.mining.Unlock()
	return node.mining.isMining
}
