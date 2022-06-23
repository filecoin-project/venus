package chain

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	apiwrapper "github.com/filecoin-project/venus/app/submodule/chain/v0api"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vmsupport"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct { //nolint
	ChainReader  *chain.Store
	MessageStore *chain.MessageStore
	Processor    *consensus.DefaultProcessor
	Fork         fork.IFork
	SystemCall   vm.SyscallsImpl

	CheckPoint types.TipSetKey
	Drand      beacon.Schedule

	config chainConfig

	Stmgr *statemanger.Stmgr
	// Wait for confirm message
	Waiter *chain.Waiter
}

type chainConfig interface {
	GenesisCid() cid.Cid
	BlockTime() time.Duration
	Repo() repo.Repo
	Verifier() ffiwrapper.Verifier
}

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(ctx context.Context,
	config chainConfig,
	circulatiingSupplyCalculator chain.ICirculatingSupplyCalcualtor,
) (*ChainSubmodule, error) {
	repo := config.Repo()
	// initialize chain store
	chainStore := chain.NewStore(repo.ChainDatastore(), repo.Datastore(), config.GenesisCid(), circulatiingSupplyCalculator)
	//drand
	genBlk, err := chainStore.GetGenesisBlock(context.TODO())
	if err != nil {
		return nil, err
	}

	drand, err := beacon.DrandConfigSchedule(genBlk.Timestamp, repo.Config().NetworkParams.BlockDelay, repo.Config().NetworkParams.DrandSchedule)
	if err != nil {
		return nil, err
	}

	messageStore := chain.NewMessageStore(config.Repo().Datastore(), repo.Config().NetworkParams.ForkUpgradeParam)
	fork, err := fork.NewChainFork(ctx, chainStore, cbor.NewCborStore(config.Repo().Datastore()), config.Repo().Datastore(), repo.Config().NetworkParams)
	if err != nil {
		return nil, err
	}
	faultChecker := consensusfault.NewFaultChecker(chainStore, fork)
	syscalls := vmsupport.NewSyscalls(faultChecker, config.Verifier())
	processor := consensus.NewDefaultProcessor(syscalls, circulatiingSupplyCalculator)

	waiter := chain.NewWaiter(chainStore, messageStore, config.Repo().Datastore(), cbor.NewCborStore(config.Repo().Datastore()))

	store := &ChainSubmodule{
		ChainReader:  chainStore,
		MessageStore: messageStore,
		Processor:    processor,
		SystemCall:   syscalls,
		Fork:         fork,
		Drand:        drand,
		config:       config,
		Waiter:       waiter,
		CheckPoint:   chainStore.GetCheckPoint(),
	}
	err = store.ChainReader.Load(context.TODO())
	if err != nil {
		return nil, err
	}
	return store, nil
}

// Start loads the chain from disk.
func (chain *ChainSubmodule) Start(ctx context.Context) error {
	return chain.Fork.Start(ctx)
}

//Stop stop the chain head event
func (chain *ChainSubmodule) Stop(ctx context.Context) {
	chain.ChainReader.Stop()
}

//API chain module api implement
func (chain *ChainSubmodule) API() v1api.IChain {
	return &chainAPI{
		IAccount:    NewAccountAPI(chain),
		IActor:      NewActorAPI(chain),
		IChainInfo:  NewChainInfoAPI(chain),
		IMinerState: NewMinerStateAPI(chain),
	}
}

func (chain *ChainSubmodule) V0API() v0api.IChain {
	return &apiwrapper.WrapperV1IChain{IChain: chain.API()}
}
