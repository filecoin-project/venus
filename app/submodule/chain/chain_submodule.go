package chain

import (
	"context"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/client/apiface/v0api"
	"github.com/filecoin-project/venus/pkg/vm"
	cbor "github.com/ipfs/go-ipld-cbor"
	"time"

	chainv0api "github.com/filecoin-project/venus/app/submodule/chain/v0api"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/vmsupport"
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
	processor := consensus.NewDefaultProcessor(syscalls)

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
func (chain *ChainSubmodule) API() apiface.IChain {
	return &chainAPI{
		IAccount:    NewAccountAPI(chain),
		IActor:      NewActorAPI(chain),
		IBeacon:     NewBeaconAPI(chain),
		IChainInfo:  NewChainInfoAPI(chain),
		IMinerState: NewMinerStateAPI(chain),
	}
}

func (chain *ChainSubmodule) V0API() v0api.IChain {
	return &chainv0api.WrapperV1IChain{IChain: chain.API()}
}
