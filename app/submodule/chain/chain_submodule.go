package chain

import (
	"context"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/slashing"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/register"
	"github.com/filecoin-project/venus/pkg/vm/state"
	"github.com/filecoin-project/venus/pkg/vmsupport"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct { //nolint
	ChainReader    *chain.Store
	MessageStore   *chain.MessageStore
	State          *cst.ChainStateReadWriter
	Sampler        *chain.Sampler
	ActorState     *appstate.TipSetStateViewer
	Processor      *consensus.DefaultProcessor
	StatusReporter *chain.StatusReporter

	Fork fork.IFork

	CheckPoint block.TipSetKey
	Drand      beacon.Schedule

	config chainConfig

	// Wait for confirm message
	Waiter *cst.Waiter
}

// xxx go back to using an interface here
type chainRepo interface {
	ChainDatastore() repo.Datastore
	Config() *config.Config
}

type chainConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	Repo() repo.Repo
}

type chainReader interface {
	chain.TipSetProvider
	GetHead() *block.TipSet
	GetTipSetReceiptsRoot(*block.TipSet) (cid.Cid, error)
	GetTipSetStateRoot(*block.TipSet) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*chain.HeadChange
	SubscribeHeadChanges(chain.ReorgNotifee)
}
type stateReader interface {
	ResolveAddressAt(context.Context, *block.TipSet, address.Address) (address.Address, error)
	GetActorAt(context.Context, *block.TipSet, address.Address) (*types.Actor, error)
	GetTipSetState(context.Context, *block.TipSet) (state.Tree, error)
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(config chainConfig,
	repo chainRepo,
	blockstore *blockstore.BlockstoreSubmodule,
	verifier ffiwrapper.Verifier,
) (*ChainSubmodule, error) {
	// initialize chain store
	chainStatusReporter := chain.NewStatusReporter()
	chainStore := chain.NewStore(repo.ChainDatastore(), blockstore.CborStore, blockstore.Blockstore, chainStatusReporter, repo.Config().NetworkParams.ForkUpgradeParam, config.GenesisCid())
	//drand
	genBlk, err := chainStore.GetGenesisBlock(context.TODO())
	if err != nil {
		return nil, err
	}

	drand, err := beacon.DrandConfigSchedule(genBlk.Timestamp, repo.Config().NetworkParams.BlockDelay, repo.Config().NetworkParams.DrandSchedule)
	if err != nil {
		return nil, err
	}

	actorState := appstate.NewTipSetStateViewer(chainStore, blockstore.CborStore)
	messageStore := chain.NewMessageStore(blockstore.Blockstore)
	chainState := cst.NewChainStateReadWriter(chainStore, messageStore, blockstore.Blockstore, register.DefaultActors, drand)
	fork, err := fork.NewChainFork(chainState, blockstore.CborStore, blockstore.Blockstore, repo.Config().NetworkParams.ForkUpgradeParam)
	if err != nil {
		return nil, err
	}
	faultChecker := slashing.NewFaultChecker(chainState, fork)
	syscalls := vmsupport.NewSyscalls(faultChecker, verifier)

	processor := consensus.NewDefaultProcessor(syscalls)

	combineChainReader := struct {
		stateReader
		chainReader
	}{
		chainState,
		chainStore,
	}
	waiter := cst.NewWaiter(combineChainReader, messageStore, blockstore.Blockstore, blockstore.CborStore)

	store := &ChainSubmodule{
		ChainReader:    chainStore,
		MessageStore:   messageStore,
		ActorState:     actorState,
		State:          chainState,
		Processor:      processor,
		StatusReporter: chainStatusReporter,
		Fork:           fork,
		Drand:          drand,
		config:         config,
		Waiter:         waiter,
		CheckPoint:     chainStore.GetCheckPoint(),
	}
	err = store.ChainReader.Load(context.TODO())
	if err != nil {
		return nil, err
	}
	return store, nil
}

// Start loads the chain from disk.
func (chain *ChainSubmodule) Start(ctx context.Context) error {
	return nil
}

func (chain *ChainSubmodule) Stop(ctx context.Context) {
	chain.ChainReader.Stop()
}

func (chain *ChainSubmodule) API() *ChainAPI {
	return &ChainAPI{
		AccountAPI:    NewAccountAPI(chain),
		ActorAPI:      NewActorAPI(chain),
		BeaconAPI:     NewBeaconAPI(chain),
		ChainInfoAPI:  NewChainInfoAPI(chain),
		DbAPI:         NewDbAPI(chain),
		MinerStateAPI: NewMinerStateAPI(chain),
	}
}
