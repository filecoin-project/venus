package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct {
	ChainReader  *chain.Store
	MessageStore *chain.MessageStore
	State        *cst.ChainStateReadWriter
	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	HeaviestTipSetCh chan interface{}

	ActorState *consensus.ActorStateStore
	Processor  *consensus.DefaultProcessor

	StatusReporter *chain.StatusReporter
}

// xxx go back to using an interface here
/*type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey block.TipSetKey) (state.Tree, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}
*/
type chainRepo interface {
	ChainDatastore() repo.Datastore
}

type chainConfig interface {
	GenesisCid() cid.Cid
	Rewarder() consensus.BlockRewarder
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(ctx context.Context, config chainConfig, repo chainRepo, blockstore *BlockstoreSubmodule) (ChainSubmodule, error) {
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
	actorState := consensus.NewActorStateStore(chainStore, blockstore.CborStore, blockstore.Blockstore, processor)
	messageStore := chain.NewMessageStore(blockstore.Blockstore)
	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.Blockstore, builtin.DefaultActors)

	return ChainSubmodule{
		ChainReader:  chainStore,
		MessageStore: messageStore,
		// HeaviestTipSetCh nil
		ActorState:     actorState,
		State:          chainState,
		Processor:      processor,
		StatusReporter: chainStatusReporter,
	}, nil
}

type chainNode interface {
	Chain() ChainSubmodule
}

// Start loads the chain from disk.
func (c *ChainSubmodule) Start(ctx context.Context, node chainNode) error {
	return node.Chain().ChainReader.Load(ctx)
}
