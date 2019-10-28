package submodule

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
)

type syncerConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	Clock() clock.Clock
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

type nodeChainSelector interface {
	NewWeight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	Weight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

type SyncerSubmodule struct {
	BlockSub      pubsub.Subscription
	ChainSelector nodeChainSelector
	Consensus     consensus.Protocol
	Syncer        nodeChainSyncer
	SyncDispatch  nodeSyncDispatcher

	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc

	// ChainSynced is a latch that releases when a nodes chain reaches a caught-up state.
	// It serves as a barrier to be released when the initial chain sync has completed.
	// Services which depend on a more-or-less synced chain can wait for this before starting up.
	ChainSynced *moresync.Latch
	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher

	validator consensus.BlockValidator
}

// NewSyncerSubmodule creates a new chain submodule.
func NewSyncerSubmodule(ctx context.Context, config syncerConfig, repo chainRepo, blockstore *BlockstoreSubmodule, network *NetworkSubmodule, discovery *DiscoverySubmodule, chn *ChainSubmodule, pvt *version.ProtocolVersionTable) (SyncerSubmodule, error) {
	// setup block validation
	// TODO when #2961 is resolved do the needful here.
	blkValid := consensus.NewDefaultBlockValidator(config.BlockTime(), config.Clock(), pvt)

	// register block validation on floodsub
	btv := net.NewBlockTopicValidator(blkValid)
	if err := network.fsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return SyncerSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// set up consensus
	nodeConsensus := consensus.NewExpected(blockstore.CborStore, blockstore.Blockstore, chn.Processor, blkValid, chn.ActorState, config.GenesisCid(), config.BlockTime(), consensus.ElectionMachine{}, consensus.TicketMachine{})
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, chn.ActorState, config.GenesisCid(), pvt)

	// setup fecher
	graphsyncNetwork := gsnet.NewFromLibp2pHost(network.host)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := net.NewGraphSyncFetcher(ctx, gsync, blockstore.Blockstore, blkValid, config.Clock(), discovery.PeerTracker)

	// only the syncer gets the storage which is online connected
	// xxx: need chain store, not just chain reader
	chainSyncer := chain.NewSyncer(nodeConsensus, nodeChainSelector, chn.ChainReader, chn.MessageStore, fetcher, chn.StatusReporter, config.Clock())
	syncerDispatcher := syncer.NewDispatcher(chainSyncer)

	return SyncerSubmodule{
		// BlockSub: nil,
		Consensus:     nodeConsensus,
		ChainSelector: nodeChainSelector,
		SyncDispatch:  syncerDispatcher,
		// cancelChainSync: nil,
		ChainSynced: moresync.NewLatch(1),
		Fetcher:     fetcher,
		validator:   blkValid,
	}, nil
}
