package node

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/block"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p"
	autonatsvc "github.com/libp2p/go-libp2p-autonat-svc"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/host"
	p2pmetrics "github.com/libp2p/go-libp2p-core/metrics"
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
	"github.com/filecoin-project/go-filecoin/journal"
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/strgdls"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/util/moresync"
	"github.com/filecoin-project/go-filecoin/version"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	BlockTime   time.Duration
	Libp2pOpts  []libp2p.Option
	OfflineMode bool
	Verifier    verification.Verifier
	Rewarder    consensus.BlockRewarder
	Repo        repo.Repo
	Journal     journal.Journal
	IsRelay     bool
	Clock       clock.Clock
	genCid      cid.Cid
}

// BuilderOpt is an option for building a filecoin node.
type BuilderOpt func(*Builder) error

// OfflineMode enables or disables offline mode.
func OfflineMode(offlineMode bool) BuilderOpt {
	return func(c *Builder) error {
		c.OfflineMode = offlineMode
		return nil
	}
}

// IsRelay configures node to act as a libp2p relay.
func IsRelay() BuilderOpt {
	return func(c *Builder) error {
		c.IsRelay = true
		return nil
	}
}

// BlockTime sets the blockTime.
func BlockTime(blockTime time.Duration) BuilderOpt {
	return func(c *Builder) error {
		c.BlockTime = blockTime
		return nil
	}
}

// Libp2pOptions returns a node config option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) BuilderOpt {
	return func(b *Builder) error {
		// Quietly having your options overridden leads to hair loss
		if len(b.Libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		b.Libp2pOpts = opts
		return nil
	}
}

// VerifierConfigOption returns a function that sets the verifier to use in the node consensus
func VerifierConfigOption(verifier verification.Verifier) BuilderOpt {
	return func(c *Builder) error {
		c.Verifier = verifier
		return nil
	}
}

// RewarderConfigOption returns a function that sets the rewarder to use in the node consensus
func RewarderConfigOption(rewarder consensus.BlockRewarder) BuilderOpt {
	return func(c *Builder) error {
		c.Rewarder = rewarder
		return nil
	}
}

// ClockConfigOption returns a function that sets the clock to use in the node.
func ClockConfigOption(clk clock.Clock) BuilderOpt {
	return func(c *Builder) error {
		c.Clock = clk
		return nil
	}
}

// JournalConfigOption returns a function that sets the journal to use in the node.
func JournalConfigOption(jrl journal.Journal) BuilderOpt {
	return func(c *Builder) error {
		c.Journal = jrl
		return nil
	}
}

// New creates a new node.
func New(ctx context.Context, opts ...BuilderOpt) (*Node, error) {
	// initialize builder and set base values
	n := &Builder{
		OfflineMode: false,
	}

	// apply builder options
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	// build the node
	return n.build(ctx)
}

func (b *Builder) build(ctx context.Context) (*Node, error) {
	//
	// Set default values on un-initialized fields
	//

	if b.Clock == nil {
		b.Clock = clock.NewSystemClock()
	}

	if b.Repo == nil {
		b.Repo = repo.NewInMemoryRepo()
	}
	if b.Journal == nil {
		b.Journal = journal.NewNoopJournal()
	}

	var err error

	// fetch genesis block id
	b.genCid, err = readGenesisCid(b.Repo.Datastore())
	if err != nil {
		return nil, err
	}

	// create the node
	nd := &Node{
		OfflineMode: b.OfflineMode,
		Clock:       b.Clock,
		Repo:        b.Repo,
	}

	nd.Blockstore, err = b.buildBlockstore(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Blockstore")
	}

	nd.Network, err = b.buildNetwork(ctx, nd.Repo.Config().Bootstrap, &nd.Blockstore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Network")
	}

	nd.VersionTable, err = version.ConfigureProtocolVersions(nd.Network.NetworkName)
	if err != nil {
		return nil, err
	}

	nd.Blockservice, err = b.buildBlockservice(ctx, &nd.Blockstore, &nd.Network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Blockservice")
	}

	nd.Chain, err = b.buildChain(ctx, &nd.Blockstore, &nd.Network, nd.VersionTable)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Chain")
	}

	nd.Wallet, err = b.buildWallet(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Wallet")
	}

	nd.Messaging, err = b.buildMessaging(ctx, &nd.Network, &nd.Chain, &nd.Wallet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.Messaging")
	}

	nd.StorageNetworking, err = b.buildStorgeNetworking(ctx, &nd.Network)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.StorageNetworking")
	}

	nd.BlockMining, err = b.buildBlockMining(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.BlockMining")
	}

	nd.SectorStorage, err = b.buildSectorStorage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.SectorStorage")
	}
	nd.HelloProtocol, err = b.buildHelloProtocol(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.HelloProtocol")
	}

	nd.StorageProtocol, err = b.buildStorageProtocol(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.StorageProtocol")
	}

	nd.RetrievalProtocol, err = b.buildRetrievalProtocol(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.RetrievalProtocol")
	}

	nd.FaultSlasher, err = b.buildFaultSlasher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build node.FaultSlasher")
	}

	nd.PorcelainAPI = porcelain.New(plumbing.New(&plumbing.APIDeps{
		Bitswap:       nd.Network.bitswap,
		Chain:         nd.Chain.State,
		Sync:          cst.NewChainSyncProvider(nd.Chain.Syncer),
		Config:        cfg.NewConfig(b.Repo),
		DAG:           dag.NewDAG(merkledag.NewDAGService(nd.Blockservice.blockservice)),
		Deals:         strgdls.New(b.Repo.DealsDatastore()),
		Expected:      nd.Chain.Consensus,
		MsgPool:       nd.Messaging.msgPool,
		MsgPreviewer:  msg.NewPreviewer(nd.Chain.ChainReader, nd.Blockstore.cborStore, nd.Blockstore.Blockstore, nd.Chain.processor),
		ActState:      nd.Chain.ActorState,
		MsgWaiter:     msg.NewWaiter(nd.Chain.ChainReader, nd.Chain.MessageStore, nd.Blockstore.Blockstore, nd.Blockstore.cborStore),
		Network:       nd.Network.Network,
		Outbox:        nd.Messaging.Outbox,
		SectorBuilder: nd.SectorBuilder,
		Wallet:        nd.Wallet.Wallet,
	}))

	return nd, nil
}

func (b *Builder) buildNetwork(ctx context.Context, config *config.BootstrapConfig, blockstore *BlockstoreSubmodule) (NetworkSubmodule, error) {
	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	b.Libp2pOpts = append(b.Libp2pOpts, libp2p.BandwidthReporter(bandwidthTracker))

	networkName, err := retrieveNetworkName(ctx, b.genCid, blockstore.Blockstore)
	if err != nil {
		return NetworkSubmodule{}, err
	}

	periodStr := config.Period
	period, err := time.ParseDuration(periodStr)
	if err != nil {
		return NetworkSubmodule{}, errors.Wrapf(err, "couldn't parse bootstrap period %s", periodStr)
	}

	// bootstrapper maintains connections to some subset of addresses
	ba := config.Addresses
	bpi, err := net.PeerAddrsToAddrInfo(ba)
	if err != nil {
		return NetworkSubmodule{}, errors.Wrapf(err, "couldn't parse bootstrap addresses [%s]", ba)
	}

	minPeerThreshold := config.MinPeerThreshold

	// set up host
	var peerHost host.Host
	var router routing.Routing
	validator := blankValidator{}
	if !b.OfflineMode {
		makeDHT := func(h host.Host) (routing.Routing, error) {
			r, err := dht.New(
				ctx,
				h,
				dhtopts.Datastore(b.Repo.Datastore()),
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
		peerHost, err = b.buildHost(ctx, makeDHT)
		if err != nil {
			return NetworkSubmodule{}, err
		}
	} else {
		router = offroute.NewOfflineRouter(b.Repo.Datastore(), validator)
		peerHost = rhost.Wrap(noopLibP2PHost{}, router)
	}

	// create a bootstrapper
	bootstrapper := net.NewBootstrapper(bpi, peerHost, peerHost.Network(), router, minPeerThreshold, period)

	// set up peer tracking
	peerTracker := net.NewPeerTracker(peerHost.ID())

	// Set up libp2p network
	// TODO: PubSub requires strict message signing, disabled for now
	// reference issue: #3124
	fsub, err := libp2pps.NewFloodSub(ctx, peerHost, libp2pps.WithMessageSigning(false))
	if err != nil {
		return NetworkSubmodule{}, errors.Wrap(err, "failed to set up network")
	}

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router)
	//nwork := bsnet.NewFromIpfsHost(innerHost, router)
	bswap := bitswap.New(ctx, nwork, blockstore.Blockstore)

	// set up pinger
	pingService := ping.NewPingService(peerHost)

	// build network
	network := net.New(peerHost, pubsub.NewPublisher(fsub), pubsub.NewSubscriber(fsub), net.NewRouter(router), bandwidthTracker, net.NewPinger(peerHost, pingService))

	// build the network submdule
	return NetworkSubmodule{
		NetworkName:  networkName,
		host:         peerHost,
		PeerHost:     peerHost,
		Bootstrapper: bootstrapper,
		PeerTracker:  peerTracker,
		Router:       router,
		fsub:         fsub,
		bitswap:      bswap,
		Network:      network,
	}, nil
}

func (b *Builder) buildMessaging(ctx context.Context, network *NetworkSubmodule, chain *ChainSubmodule, wallet *WalletSubmodule) (MessagingSubmodule, error) {
	msgPool := message.NewPool(b.Repo.Config().Mpool, consensus.NewIngestionValidator(chain.State, b.Repo.Config().Mpool))
	inbox := message.NewInbox(msgPool, message.InboxMaxAgeTipsets, chain.ChainReader, chain.MessageStore)

	msgQueue := message.NewQueue()
	outboxPolicy := message.NewMessageQueuePolicy(chain.MessageStore, message.OutboxMaxAgeRounds)
	msgPublisher := message.NewDefaultPublisher(pubsub.NewPublisher(network.fsub), net.MessageTopic(network.NetworkName), msgPool)
	outbox := message.NewOutbox(wallet.Wallet, consensus.NewOutboundMessageValidator(), msgQueue, msgPublisher, outboxPolicy, chain.ChainReader, chain.State, b.Journal.Topic("outbox"))

	return MessagingSubmodule{
		Inbox:   inbox,
		Outbox:  outbox,
		msgPool: msgPool,
	}, nil
}

func (b *Builder) buildBlockstore(ctx context.Context) (BlockstoreSubmodule, error) {
	// set up block store
	bs := bstore.NewBlockstore(b.Repo.Datastore())
	// setup a ipldCbor on top of the local store
	ipldCborStore := hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	return BlockstoreSubmodule{
		Blockstore: bs,
		cborStore:  &ipldCborStore,
	}, nil
}

func (b *Builder) buildBlockservice(ctx context.Context, blockstore *BlockstoreSubmodule, network *NetworkSubmodule) (BlockserviceSubmodule, error) {
	bservice := bserv.New(blockstore.Blockstore, network.bitswap)

	return BlockserviceSubmodule{
		blockservice: bservice,
	}, nil
}

func (b *Builder) buildChain(ctx context.Context, blockstore *BlockstoreSubmodule, network *NetworkSubmodule, pvt *version.ProtocolVersionTable) (ChainSubmodule, error) {
	// initialize chain store
	chainStatusReporter := chain.NewStatusReporter()
	chainStore := chain.NewStore(b.Repo.ChainDatastore(), blockstore.cborStore, &state.TreeStateLoader{}, chainStatusReporter, b.genCid)

	// set up processor
	var processor *consensus.DefaultProcessor
	if b.Rewarder == nil {
		processor = consensus.NewDefaultProcessor()
	} else {
		processor = consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), b.Rewarder, builtin.DefaultActors)
	}

	// setup block validation
	// TODO when #2961 is resolved do the needful here.
	blkValid := consensus.NewDefaultBlockValidator(b.BlockTime, b.Clock, pvt)

	// register block validation on floodsub
	btv := net.NewBlockTopicValidator(blkValid)
	if err := network.fsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return ChainSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// set up consensus
	actorState := consensus.NewActorStateStore(chainStore, blockstore.cborStore, blockstore.Blockstore, processor)
	nodeConsensus := consensus.NewExpected(blockstore.cborStore, blockstore.Blockstore, processor, blkValid, actorState, b.genCid, b.BlockTime, consensus.ElectionMachine{}, consensus.TicketMachine{})
	nodeChainSelector := consensus.NewChainSelector(blockstore.cborStore, actorState, b.genCid, pvt)

	// setup fecher
	graphsyncNetwork := gsnet.NewFromLibp2pHost(network.host)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := net.NewGraphSyncFetcher(ctx, gsync, blockstore.Blockstore, blkValid, b.Clock, network.PeerTracker)

	messageStore := chain.NewMessageStore(blockstore.cborStore)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewSyncer(nodeConsensus, nodeChainSelector, chainStore, messageStore, fetcher, chainStatusReporter, b.Clock)

	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.cborStore, builtin.DefaultActors)

	return ChainSubmodule{
		// BlockSub: nil,
		Consensus:     nodeConsensus,
		ChainSelector: nodeChainSelector,
		ChainReader:   chainStore,
		MessageStore:  messageStore,
		Syncer:        chainSyncer,
		ActorState:    actorState,
		// HeaviestTipSetCh: nil,
		// cancelChainSync: nil,
		ChainSynced: moresync.NewLatch(1),
		Fetcher:     fetcher,
		State:       chainState,
		validator:   blkValid,
		processor:   processor,
	}, nil
}

func (b *Builder) buildBlockMining(ctx context.Context) (BlockMiningSubmodule, error) {
	return BlockMiningSubmodule{
		// BlockMiningAPI:     nil,
		// AddNewlyMinedBlock: nil,
		// cancelMining:       nil,
		// MiningWorker:       nil,
		// MiningScheduler:    nil,
		// mining:       nil,
		// miningDoneWg: nil,
		// MessageSub:   nil,
	}, nil
}

func (b *Builder) buildSectorStorage(ctx context.Context) (SectorBuilderSubmodule, error) {
	return SectorBuilderSubmodule{
		// sectorBuilder: nil,
	}, nil
}

func (b *Builder) buildHelloProtocol(ctx context.Context) (HelloProtocolSubmodule, error) {
	return HelloProtocolSubmodule{
		// HelloSvc: nil,
	}, nil
}

func (b *Builder) buildStorageProtocol(ctx context.Context) (StorageProtocolSubmodule, error) {
	return StorageProtocolSubmodule{
		// StorageAPI: nil,
		// StorageMiner: nil,
	}, nil
}

func (b *Builder) buildRetrievalProtocol(ctx context.Context) (RetrievalProtocolSubmodule, error) {
	return RetrievalProtocolSubmodule{
		// RetrievalAPI: nil,
		// RetrievalMiner: nil,
	}, nil
}

func (b *Builder) buildWallet(ctx context.Context) (WalletSubmodule, error) {
	backend, err := wallet.NewDSBackend(b.Repo.WalletDatastore())
	if err != nil {
		return WalletSubmodule{}, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	return WalletSubmodule{
		Wallet: fcWallet,
	}, nil
}

func (b *Builder) buildStorgeNetworking(ctx context.Context, network *NetworkSubmodule) (StorageNetworkingSubmodule, error) {
	return StorageNetworkingSubmodule{
		Exchange: network.bitswap,
	}, nil
}

func (b *Builder) buildFaultSlasher(ctx context.Context) (FaultSlasherSubmodule, error) {
	return FaultSlasherSubmodule{
		// StorageFaultSlasher: nil,
	}, nil
}

func retrieveNetworkName(ctx context.Context, genCid cid.Cid, bs bstore.Blockstore) (string, error) {
	cborStore := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	var genesis block.Block

	err := cborStore.Get(ctx, genCid, &genesis)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get block %s", genCid.String())
	}

	tree, err := state.LoadStateTree(ctx, cborStore, genesis.StateRoot)
	if err != nil {
		return "", errors.Wrapf(err, "failed to load node for %s", genesis.StateRoot)
	}
	initActor, err := tree.GetActor(ctx, address.InitAddress)
	if err != nil {
		return "", errors.Wrapf(err, "failed to load init actor at %s", address.InitAddress.String())
	}

	block, err := bs.Get(initActor.Head)
	if err != nil {
		return "", errors.Wrapf(err, "failed to load init actor state at %s", initActor.Head)
	}

	node, err := cbornode.DecodeBlock(block)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode init actor state")
	}

	networkName, _, err := node.Resolve([]string{"network"})
	if err != nil || networkName == nil {
		return "", errors.Wrapf(err, "failed to retrieve network name")
	}

	return networkName.(string), nil
}

// buildHost determines if we are publically dialable.  If so use public
// Address, if not configure node to announce relay address.
func (b *Builder) buildHost(ctx context.Context, makeDHT func(host host.Host) (routing.Routing, error)) (host.Host, error) {
	// Node must build a host acting as a libp2p relay.  Additionally it
	// runs the autoNAT service which allows other nodes to check for their
	// own dialability by having this node attempt to dial them.
	makeDHTRightType := func(h host.Host) (routing.PeerRouting, error) {
		return makeDHT(h)
	}

	if b.IsRelay {
		cfg := b.Repo.Config()
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
			libp2p.ChainOptions(b.Libp2pOpts...),
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
		libp2p.ChainOptions(b.Libp2pOpts...),
	)
}
