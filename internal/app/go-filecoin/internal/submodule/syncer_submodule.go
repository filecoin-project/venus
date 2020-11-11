package submodule

import (
	"context"
	"time"

	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/beacon"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/chainsync"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/fetcher"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/net/blocksub"
	"github.com/filecoin-project/venus/internal/pkg/net/pubsub"
	"github.com/filecoin-project/venus/internal/pkg/slashing"
	"github.com/filecoin-project/venus/internal/pkg/state"
)

// SyncerSubmodule enhances the node with chain syncing capabilities
type SyncerSubmodule struct {
	BlockTopic       *pubsub.Topic
	BlockSub         pubsub.Subscription
	ChainSelector    nodeChainSelector
	Consensus        consensus.Protocol
	FaultDetector    slashing.ConsensusFaultDetector
	ChainSyncManager *chainsync.Manager
	Drand            beacon.Schedule

	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	CancelChainSync context.CancelFunc
	// faultCh receives detected consensus faults
	faultCh chan slashing.ConsensusFault
}

type syncerConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	ChainClock() clock.ChainEpochClock
	Drand() beacon.Schedule
}

type nodeChainSelector interface {
	Weight(context.Context, *block.TipSet) (fbig.Int, error)
	IsHeavier(ctx context.Context, a, b *block.TipSet) (bool, error)
}

// NewSyncerSubmodule creates a new chain submodule.
func NewSyncerSubmodule(ctx context.Context,
	config syncerConfig,
	blockstore *BlockstoreSubmodule,
	network *NetworkSubmodule,
	discovery *DiscoverySubmodule,
	chn *ChainSubmodule,
	postVerifier consensus.ProofVerifier,
	checkPoint block.TipSetKey) (SyncerSubmodule, error) {
	// setup validation
	blkValid := consensus.NewDefaultBlockValidator(config.ChainClock(), chn.MessageStore, chn.State)
	msgValid := consensus.NewMessageSyntaxValidator()
	syntax := consensus.WrappedSyntaxValidator{
		BlockSyntaxValidator:   blkValid,
		MessageSyntaxValidator: msgValid,
	}

	// register block validation on pubsub
	btv := blocksub.NewBlockTopicValidator(blkValid)
	if err := network.pubsub.RegisterTopicValidator(btv.Topic(network.NetworkName), btv.Validator(), btv.Opts()...); err != nil {
		return SyncerSubmodule{}, errors.Wrap(err, "failed to register block validator")
	}

	// setup topic.
	topic, err := network.pubsub.Join(blocksub.Topic(network.NetworkName))
	if err != nil {
		return SyncerSubmodule{}, err
	}

	genBlk, err := chn.ChainReader.GetGenesisBlock(ctx)
	if err != nil {
		return SyncerSubmodule{}, errors.Wrap(err, "failed to locate genesis block during node build")
	}

	// setup default drand
	d := config.Drand()

	// set up consensus
	//	elections := consensus.NewElectionMachine(chn.state)
	sampler := chain.NewSampler(chn.ChainReader, genBlk.Ticket)
	tickets := consensus.NewTicketMachine(sampler)
	stateViewer := consensus.AsDefaultStateViewer(state.NewViewer(blockstore.CborStore))

	nodeConsensus := consensus.NewExpected(blockstore.CborStore, blockstore.Blockstore, chn.Processor, &stateViewer,
		config.BlockTime(), tickets, postVerifier, chn.ChainReader, config.ChainClock(), d, chn.State, chn.MessageStore, chn.Fork)
	nodeChainSelector := consensus.NewChainSelector(blockstore.CborStore, &stateViewer)

	// setup fecher
	network.GraphExchange.RegisterIncomingRequestHook(func(p peer.ID, requestData graphsync.RequestData, hookActions graphsync.IncomingRequestHookActions) {
		_, has := requestData.Extension(fetcher.ChainsyncProtocolExtension)
		if has {
			// TODO: Don't just validate every request with the extension -- support only known selectors
			// TODO: use separate block store for the chain (supported in GraphSync)
			hookActions.ValidateRequest()
		}
	})
	fetcher := fetcher.NewGraphSyncFetcher(ctx, network.GraphExchange, blockstore.Blockstore, syntax, config.ChainClock(), discovery.PeerTracker)
	exchangeClient := exchange.NewClient(network.Host, network.PeerMgr)
	faultCh := make(chan slashing.ConsensusFault)
	faultDetector := slashing.NewConsensusFaultDetector(faultCh)

	chainSyncManager, err := chainsync.NewManager(nodeConsensus, blkValid, nodeChainSelector, chn.ChainReader, chn.MessageStore, blockstore.Blockstore, fetcher, exchangeClient, config.ChainClock(), checkPoint, faultDetector, chn.Fork)
	if err != nil {
		return SyncerSubmodule{}, err
	}

	return SyncerSubmodule{
		BlockTopic: pubsub.NewTopic(topic),
		// BlockSub: nil,
		Consensus:        nodeConsensus,
		ChainSelector:    nodeChainSelector,
		ChainSyncManager: &chainSyncManager,
		Drand:            d,
		// cancelChainSync: nil,
		faultCh: faultCh,
	}, nil
}

type syncerNode interface {
}

// Start starts the syncer submodule for a node.
func (s *SyncerSubmodule) Start(ctx context.Context, _node syncerNode) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.faultCh:
				// TODO #3690 connect this up to a slasher that sends messages
				// to outbound queue to carry out penalization
			}
		}
	}()
	return s.ChainSyncManager.Start(ctx)
}
