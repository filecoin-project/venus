package chain

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/proofverification"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/slashing"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/vm/register"
	"github.com/filecoin-project/venus/pkg/vmsupport"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct { //nolint
	ChainReader  *chain.Store
	MessageStore *chain.MessageStore
	State        *cst.ChainStateReadWriter

	Sampler    *chain.Sampler
	ActorState *appstate.TipSetStateViewer
	Processor  *consensus.DefaultProcessor

	StatusReporter *chain.StatusReporter

	Fork fork.IFork

	CheckPoint block.TipSetKey
	Drand      beacon.Schedule

	config chainConfig
}

// xxx go back to using an interface here
type chainRepo interface {
	ChainDatastore() repo.Datastore
}

type chainConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(config chainConfig,
	repo chainRepo,
	blockstore *blockstore.BlockstoreSubmodule,
	verifier *proofverification.ProofVerificationSubmodule,
) (*ChainSubmodule, error) {
	// initialize chain store
	chainStatusReporter := chain.NewStatusReporter()
	chainStore := chain.NewStore(repo.ChainDatastore(), blockstore.CborStore, blockstore.Blockstore, chainStatusReporter, config.GenesisCid())
	//drand
	genBlk, err := chainStore.GetGenesisBlock(context.TODO())
	if err != nil {
		return nil, err
	}
	drand, err := beacon.DefaultDrandIfaceFromConfig(genBlk.Timestamp)
	if err != nil {
		return nil, err
	}
	actorState := appstate.NewTipSetStateViewer(chainStore, blockstore.CborStore)
	messageStore := chain.NewMessageStore(blockstore.Blockstore)
	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.Blockstore, register.DefaultActors, drand)
	faultChecker := slashing.NewFaultChecker(chainState)
	syscalls := vmsupport.NewSyscalls(faultChecker, verifier.ProofVerifier)
	fork, err := fork.NewChainFork(chainState, blockstore.CborStore, blockstore.Blockstore)
	if err != nil {
		return nil, err
	}
	processor := consensus.NewDefaultProcessor(syscalls, chainState)

	return &ChainSubmodule{
		ChainReader:    chainStore,
		MessageStore:   messageStore,
		ActorState:     actorState,
		State:          chainState,
		Processor:      processor,
		StatusReporter: chainStatusReporter,
		Fork:           fork,
		Drand:          drand,
		config:         config,
		CheckPoint:     chainStore.GetCheckPoint(),
	}, nil
}

// Start loads the chain from disk.
func (chain *ChainSubmodule) Start(ctx context.Context) error {
	return chain.ChainReader.Load(ctx)
}

// StateView loads the state view for a tipset, i.e. the state *after* the application of the tipset's messages.
func (chain *ChainSubmodule) StateView(baseKey block.TipSetKey) (*appstate.View, error) {
	view, err := chain.State.StateView(baseKey)
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (chain *ChainSubmodule) API() *ChainAPI {
	return &ChainAPI{chain: chain}
}
