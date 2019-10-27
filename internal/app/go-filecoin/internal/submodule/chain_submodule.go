package submodule

import (
	"context"
	"time"

	ps "github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct {
	BlockSub      pubsub.Subscription
	Consensus     consensus.Protocol
	ChainSelector nodeChainSelector
	ChainReader   nodeChainReader
	MessageStore  *chain.MessageStore
	Syncer        nodeChainSyncer
	SyncDispatch  nodeSyncDispatcher
	ActorState    *consensus.ActorStateStore

	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	HeaviestTipSetCh chan interface{}
	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc
	// ChainSynced is a latch that releases when a nodes chain reaches a caught-up state.
	// It serves as a barrier to be released when the initial chain sync has completed.
	// Services which depend on a more-or-less synced chain can wait for this before starting up.
	ChainSynced *moresync.Latch
	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher
	State   *cst.ChainStateReadWriter

	validator consensus.BlockValidator
	Processor *consensus.DefaultProcessor
}

type nodeChainSelector interface {
	NewWeight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	Weight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey block.TipSetKey) (state.Tree, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}

type nodeChainSyncer interface {
	HandleNewTipSet(ctx context.Context, ci *block.ChainInfo, trusted bool) error
	Status() chain.Status
}

type nodeSyncDispatcher interface {
	SendHello(*block.ChainInfo) error
	SendOwnBlock(*block.ChainInfo) error
	SendGossipBlock(*block.ChainInfo) error
	Start(context.Context)
}

type chainRepo interface {
	ChainDatastore() repo.Datastore
}

type chainConfig interface {
	GenesisCid() cid.Cid
	Clock() clock.Clock
	Rewarder() consensus.BlockRewarder
	BlockTime() time.Duration
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(ctx context.Context, config chainConfig, repo chainRepo, blockstore *BlockstoreSubmodule, network *NetworkSubmodule, discovery *DiscoverySubmodule, pvt *version.ProtocolVersionTable) (ChainSubmodule, error) {
	// initialize chain store
	chainStatusReporter := chain.NewStatusReporter()
	chainStore := chain.NewStore(repo.ChainDatastore(), blockstore.CborStore, &state.TreeStateLoader{}, chainStatusReporter, config.GenesisCid())

	// set up processor
	var processor *consensus.DefaultProcessor
	if config.Rewarder() == nil {
		processor = consensus.NewDefaultProcessor()
	} else {
		processor = consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), config.Rewarder(), builtin.DefaultActors)
	}

	// setup block validation
	// TODO when #2961 is resolved do the needful here.
	blkValid := consensus.NewDefaultBlockValidator(config.BlockTime(), config.Clock(), pvt)

	// register block validation on floodsub
	btv := net.NewBlockTopicValidator(blkValid)
	if err := network.fsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return ChainSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// set up consensus
	actorState := consensus.NewActorStateStore(chainStore, blockstore.CborStore, blockstore.Blockstore, processor)
	nodeConsensus := consensus.NewExpected(blockstore.CborStore, blockstore.Blockstore, processor, blkValid, actorState, config.GenesisCid(), config.BlockTime(), consensus.ElectionMachine{}, consensus.TicketMachine{})
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, actorState, config.GenesisCid(), pvt)

	// setup fecher
	graphsyncNetwork := gsnet.NewFromLibp2pHost(network.host)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := net.NewGraphSyncFetcher(ctx, gsync, blockstore.Blockstore, blkValid, config.Clock(), discovery.PeerTracker)

	messageStore := chain.NewMessageStore(blockstore.Blockstore)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewSyncer(nodeConsensus, nodeChainSelector, chainStore, messageStore, fetcher, chainStatusReporter, config.Clock())
	syncerDispatcher := syncer.NewDispatcher(chainSyncer)

	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.CborStore, builtin.DefaultActors)

	return ChainSubmodule{
		// BlockSub: nil,
		Consensus:     nodeConsensus,
		ChainSelector: nodeChainSelector,
		ChainReader:   chainStore,
		MessageStore:  messageStore,
		SyncDispatch:  syncerDispatcher,
		ActorState:    actorState,
		// HeaviestTipSetCh: nil,
		// cancelChainSync: nil,
		ChainSynced: moresync.NewLatch(1),
		Fetcher:     fetcher,
		State:       chainState,
		validator:   blkValid,
		Processor:   processor,
	}, nil
}
