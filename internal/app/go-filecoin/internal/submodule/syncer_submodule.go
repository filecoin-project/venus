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
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/fetcher"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
)

// SyncerSubmodule enhances the node with chain syncing capabilities
type SyncerSubmodule struct {
	BlockTopic       *pubsub.Topic
	BlockSub         pubsub.Subscription
	ChainSelector    nodeChainSelector
	Consensus        consensus.Protocol
	ChainSyncManager *chainsync.Manager

	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc
}

type syncerConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	Clock() clock.Clock
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
	if err := network.pubsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return SyncerSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// setup topic.
	topic, err := network.pubsub.Join(net.BlockTopic(network.NetworkName))
	if err != nil {
		return SyncerSubmodule{}, err
	}

	// set up consensus
	nodeConsensus := consensus.NewExpected(blockstore.CborStore, blockstore.Blockstore, chn.Processor, chn.ActorState, config.BlockTime(), consensus.ElectionMachine{}, consensus.TicketMachine{})
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, chn.ActorState, config.GenesisCid())

	// setup fecher
	graphsyncNetwork := gsnet.NewFromLibp2pHost(network.Host)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(blockstore.Blockstore)
	storer := gsstoreutil.StorerForBlockstore(blockstore.Blockstore)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)
	fetcher := fetcher.NewGraphSyncFetcher(ctx, gsync, blockstore.Blockstore, blkValid, config.Clock(), discovery.PeerTracker)

	chainSyncManager, err := chainsync.NewManager(nodeConsensus, blkValid, nodeChainSelector, chn.ChainReader, chn.MessageStore, fetcher, config.Clock())
	if err != nil {
		return SyncerSubmodule{}, err
	}

	return SyncerSubmodule{
		BlockTopic: pubsub.NewTopic(topic),
		// BlockSub: nil,
		Consensus:        nodeConsensus,
		ChainSelector:    nodeChainSelector,
		ChainSyncManager: &chainSyncManager,
		// cancelChainSync: nil,
	}, nil
}

type syncerNode interface {
}

// Start starts the syncer submodule for a node.
func (s *SyncerSubmodule) Start(ctx context.Context, _node syncerNode) error {
	return s.ChainSyncManager.Start(ctx)
}
