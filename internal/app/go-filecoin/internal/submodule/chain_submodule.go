package submodule

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/venus/internal/pkg/beacon"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/fork"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	"github.com/filecoin-project/venus/internal/pkg/slashing"
	appstate "github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/vm/register"
	"github.com/filecoin-project/venus/internal/pkg/vmsupport"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct {
	ChainReader  *chain.Store
	MessageStore *chain.MessageStore
	State        *cst.ChainStateReadWriter

	Sampler    *chain.Sampler
	ActorState *appstate.TipSetStateViewer
	Processor  *consensus.DefaultProcessor

	StatusReporter *chain.StatusReporter

	Fork fork.IFork
}

// xxx go back to using an interface here
/*type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey block.TipSetKey) (state.state, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	GetTipSetReceiptsRoot(tsKey block.TipSetKey) (cid.Cid, error)
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
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(config chainConfig, repo chainRepo, blockstore *BlockstoreSubmodule, verifier *ProofVerificationSubmodule, checkPoint block.TipSetKey, drand beacon.Schedule) (ChainSubmodule, error) {
	// initialize chain store
	chainStatusReporter := chain.NewStatusReporter()
	chainStore := chain.NewStore(repo.ChainDatastore(), blockstore.CborStore, blockstore.Blockstore, chainStatusReporter, checkPoint, config.GenesisCid())

	actorState := appstate.NewTipSetStateViewer(chainStore, blockstore.CborStore)
	messageStore := chain.NewMessageStore(blockstore.Blockstore)
	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.Blockstore, register.DefaultActors, drand)
	faultChecker := slashing.NewFaultChecker(chainState)
	syscalls := vmsupport.NewSyscalls(faultChecker, verifier.ProofVerifier)
	fork, err := fork.NewChainFork(chainState, blockstore.CborStore, blockstore.Blockstore)
	if err != nil {
		return ChainSubmodule{}, err
	}
	processor := consensus.NewDefaultProcessor(syscalls, chainState)

	return ChainSubmodule{
		ChainReader:    chainStore,
		MessageStore:   messageStore,
		ActorState:     actorState,
		State:          chainState,
		Processor:      processor,
		StatusReporter: chainStatusReporter,
		Fork:           fork,
	}, nil
}

type chainNode interface {
	Chain() ChainSubmodule
}

// Start loads the chain from disk.
func (c *ChainSubmodule) Start(ctx context.Context, node chainNode) error {
	return node.Chain().ChainReader.Load(ctx)
}
