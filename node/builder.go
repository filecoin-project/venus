package node

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
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
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-merkledag"
	libp2p "github.com/libp2p/go-libp2p"
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
)

// Builder is a helper to aid in the construction of a filecoin node.
type Builder struct {
	BlockTime   time.Duration
	Libp2pOpts  []libp2p.Option
	OfflineMode bool
	Verifier    verification.Verifier
	Rewarder    consensus.BlockRewarder
	Repo        repo.Repo
	IsRelay     bool
	Clock       clock.Clock
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
	return func(nc *Builder) error {
		// Quietly having your options overridden leads to hair loss
		if len(nc.Libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		nc.Libp2pOpts = opts
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

// New creates a new node.
func New(ctx context.Context, opts ...BuilderOpt) (*Node, error) {
	n := &Builder{}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, err
		}
	}

	return n.build(ctx)
}

// Build instantiates a filecoin Node from the settings specified in the config.
func (nc *Builder) build(ctx context.Context) (*Node, error) {
	if nc.Repo == nil {
		nc.Repo = repo.NewInMemoryRepo()
	}
	if nc.Clock == nil {
		nc.Clock = clock.NewSystemClock()
	}

	bs := bstore.NewBlockstore(nc.Repo.Datastore())

	validator := blankValidator{}

	var peerHost host.Host
	var router routing.Routing

	bandwidthTracker := p2pmetrics.NewBandwidthCounter()
	nc.Libp2pOpts = append(nc.Libp2pOpts, libp2p.BandwidthReporter(bandwidthTracker))

	ipldCborStore := hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	genCid, err := readGenesisCid(nc.Repo.Datastore())
	if err != nil {
		return nil, err
	}

	chainStatusReporter := chain.NewStatusReporter()
	// set up chain and message stores
	chainStore := chain.NewStore(nc.Repo.ChainDatastore(), &ipldCborStore, &state.TreeStateLoader{}, chainStatusReporter, genCid)
	messageStore := chain.NewMessageStore(&ipldCborStore)
	chainState := cst.NewChainStateProvider(chainStore, messageStore, &ipldCborStore)
	powerTable := &consensus.MarketView{}

	// create protocol upgrade table
	network, err := networkNameFromGenesis(ctx, chainStore, bs)
	if err != nil {
		return nil, err
	}

	if !nc.OfflineMode {
		makeDHT := func(h host.Host) (routing.Routing, error) {
			r, err := dht.New(
				ctx,
				h,
				dhtopts.Datastore(nc.Repo.Datastore()),
				dhtopts.NamespacedValidator("v", validator),
				dhtopts.Protocols(net.FilecoinDHT(network)),
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
	blkValid := consensus.NewDefaultBlockValidator(nc.BlockTime, nc.Clock)

	// set up peer tracking
	peerTracker := net.NewPeerTracker(peerHost.ID())

	// set up bitswap
	nwork := bsnet.NewFromIpfsHost(peerHost, router)
	//nwork := bsnet.NewFromIpfsHost(innerHost, router)
	bswap := bitswap.New(ctx, nwork, bs)
	bservice := bserv.New(bs, bswap)

	graphsyncNetwork := gsnet.NewFromLibp2pHost(peerHost)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(bs)
	storer := gsstoreutil.StorerForBlockstore(bs)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := net.NewGraphSyncFetcher(ctx, gsync, bs, blkValid, peerTracker)

	// TODO: inject protocol upgrade table into code that requires it (#3360)
	_, err = version.ConfigureProtocolVersions(network)
	if err != nil {
		return nil, err
	}

	// set up processor
	var processor consensus.Processor
	if nc.Rewarder == nil {
		processor = consensus.NewDefaultProcessor()
	} else {
		processor = consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), nc.Rewarder)
	}

	// set up consensus
	nodeConsensus := consensus.NewExpected(&ipldCborStore, bs, processor, blkValid, powerTable, genCid, nc.BlockTime, consensus.ElectionMachine{}, consensus.TicketMachine{})

	// Set up libp2p network
	// TODO PubSub requires strict message signing, disabled for now
	// reference issue: #3124
	fsub, err := libp2pps.NewFloodSub(ctx, peerHost, libp2pps.WithMessageSigning(false))
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up network")
	}
	// register block validation on floodsub
	btv := net.NewBlockTopicValidator(blkValid)
	if err := fsub.RegisterTopicValidator(btv.Topic(network), btv.Validator(), btv.Opts()...); err != nil {
		return nil, errors.Wrap(err, "failed to register block validator")
	}

	backend, err := wallet.NewDSBackend(nc.Repo.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewSyncer(nodeConsensus, chainStore, messageStore, fetcher, chainStatusReporter, nc.Clock)
	msgPool := core.NewMessagePool(nc.Repo.Config().Mpool, consensus.NewIngestionValidator(chainState, nc.Repo.Config().Mpool))
	inbox := core.NewInbox(msgPool, core.InboxMaxAgeTipsets, chainStore, messageStore)

	msgQueue := core.NewMessageQueue()
	outboxPolicy := core.NewMessageQueuePolicy(messageStore, core.OutboxMaxAgeRounds)
	msgPublisher := core.NewDefaultMessagePublisher(pubsub.NewPublisher(fsub), net.MessageTopic(network), msgPool)
	outbox := core.NewOutbox(fcWallet, consensus.NewOutboundMessageValidator(), msgQueue, msgPublisher, outboxPolicy, chainStore, chainState)

	nd := &Node{
		blockservice: bservice,
		Blockstore:   bs,
		cborStore:    &ipldCborStore,
		Clock:        nc.Clock,
		Consensus:    nodeConsensus,
		ChainReader:  chainStore,
		ChainSynced:  moresync.NewLatch(1),
		MessageStore: messageStore,
		Syncer:       chainSyncer,
		PowerTable:   powerTable,
		PeerTracker:  peerTracker,
		Fetcher:      fetcher,
		Exchange:     bswap,
		host:         peerHost,
		Inbox:        inbox,
		OfflineMode:  nc.OfflineMode,
		Outbox:       outbox,
		NetworkName:  network,
		PeerHost:     peerHost,
		Repo:         nc.Repo,
		Wallet:       fcWallet,
		Router:       router,
	}

	nd.PorcelainAPI = porcelain.New(plumbing.New(&plumbing.APIDeps{
		Bitswap:       bswap,
		Chain:         chainState,
		Sync:          cst.NewChainSyncProvider(chainSyncer),
		Config:        cfg.NewConfig(nc.Repo),
		DAG:           dag.NewDAG(merkledag.NewDAGService(bservice)),
		Deals:         strgdls.New(nc.Repo.DealsDatastore()),
		Expected:      nodeConsensus,
		MsgPool:       msgPool,
		MsgPreviewer:  msg.NewPreviewer(chainStore, &ipldCborStore, bs),
		MsgQueryer:    msg.NewQueryer(chainStore, &ipldCborStore, bs),
		MsgWaiter:     msg.NewWaiter(chainStore, messageStore, bs, &ipldCborStore),
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

// buildHost determines if we are publically dialable.  If so use public
// Address, if not configure node to announce relay address.
func (nc *Builder) buildHost(ctx context.Context, makeDHT func(host host.Host) (routing.Routing, error)) (host.Host, error) {
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
