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
)

// SyncerSubmodule enhances the node with chain syncing capabilities
type SyncerSubmodule struct {
	BlockSub      pubsub.Subscription
	ChainSelector nodeChainSelector
	Consensus     consensus.Protocol
	Syncer        nodeChainSyncer
	SyncDispatch  nodeSyncDispatcher

	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc

	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher

	validator consensus.BlockValidator
}

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
	Weight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

// NewSyncerSubmodule creates a new chain submodule.
func NewSyncerSubmodule(ctx context.Context, config syncerConfig, repo chainRepo, blockstore *BlockstoreSubmodule, network *NetworkSubmodule, discovery *DiscoverySubmodule, chn *ChainSubmodule) (SyncerSubmodule, error) {
	// setup block validation
	// TODO when #2961 is resolved do the needful here.
	blkValid := consensus.NewDefaultBlockValidator(config.BlockTime(), config.Clock())

	// register block validation on floodsub
	btv := net.NewBlockTopicValidator(blkValid)
	if err := network.fsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return SyncerSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// set up consensus
	nodeConsensus := consensus.NewExpected(blockstore.CborStore, blockstore.Blockstore, chn.Processor, blkValid, chn.ActorState, config.GenesisCid(), config.BlockTime(), consensus.ElectionMachine{}, consensus.TicketMachine{})
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, chn.ActorState, config.GenesisCid())

	// setup fecher
	graphsyncNetwork := gsnet.NewFromLibp2pHost(network.host)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := net.NewGraphSyncFetcher(ctx, gsync, blockstore.Blockstore, blkValid, config.Clock(), discovery.PeerTracker)

	// only the syncer gets the storage which is online connected
	chainSyncer := chain.NewSyncer(nodeConsensus, nodeChainSelector, chn.ChainReader, chn.MessageStore, fetcher, chn.StatusReporter, config.Clock())
	syncerDispatcher := syncer.NewDispatcher(chainSyncer)

	return SyncerSubmodule{
		// BlockSub: nil,
		Consensus:     nodeConsensus,
		ChainSelector: nodeChainSelector,
		SyncDispatch:  syncerDispatcher,
		// cancelChainSync: nil,
		Fetcher:   fetcher,
		validator: blkValid,
	}, nil
}

type syncerNode interface {
	Syncer() SyncerSubmodule
}

// Start starts the syncer submodule for a node.
func (s *SyncerSubmodule) Start(ctx context.Context, node syncerNode) {
	node.Syncer().SyncDispatch.Start(ctx)
}
