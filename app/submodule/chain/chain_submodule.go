package chain

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/app/submodule/apiface/v0api"
	chainv0api "github.com/filecoin-project/venus/app/submodule/chain/v0api"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/slashing"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/vmsupport"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct { //nolint
	ChainReader    *chain.Store
	MessageStore   *chain.MessageStore
	Sampler        *chain.Sampler
	Processor      *consensus.DefaultProcessor
	StatusReporter *chain.StatusReporter

	Fork fork.IFork

	CheckPoint types.TipSetKey
	Drand      beacon.Schedule

	config chainConfig

	// Wait for confirm message
	Waiter *chain.Waiter
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

// NewChainSubmodule creates a new chain submodule.
func NewChainSubmodule(ctx context.Context,
	config chainConfig,
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

	messageStore := chain.NewMessageStore(blockstore.Blockstore, repo.Config().NetworkParams.ForkUpgradeParam)
	fork, err := fork.NewChainFork(ctx, chainStore, blockstore.CborStore, blockstore.Blockstore, repo.Config().NetworkParams)
	if err != nil {
		return nil, err
	}
	faultChecker := slashing.NewFaultChecker(chainStore, fork)
	syscalls := vmsupport.NewSyscalls(faultChecker, verifier)

	processor := consensus.NewDefaultProcessor(syscalls)

	waiter := chain.NewWaiter(chainStore, messageStore, blockstore.Blockstore, blockstore.CborStore)

	store := &ChainSubmodule{
		ChainReader:    chainStore,
		MessageStore:   messageStore,
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
	return chain.Fork.Start(ctx)
}

func (chain *ChainSubmodule) Stop(ctx context.Context) {
	chain.ChainReader.Stop()
}

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
