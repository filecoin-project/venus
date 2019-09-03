package block

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
)

type miningChainReader interface {
	GetHead() types.TipSetKey
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
}

// MiningAPI provides an interface to the block mining protocol.
type MiningAPI struct {
	minerAddress    func() (address.Address, error)
	addNewBlockFunc func(context.Context, *types.Block) (err error)
	chainReader     miningChainReader
	isMiningFunc    func() bool
	mineDelay       time.Duration
	setupMiningFunc func(context.Context) error
	startMiningFunc func(context.Context) error
	stopMiningFunc  func(context.Context)
	getWorkerFunc   func(ctx context.Context) (mining.Worker, error)
}

// New creates a new MiningAPI instance with the provided deps
func New(
	minerAddr func() (address.Address, error),
	addNewBlockFunc func(context.Context, *types.Block) (err error),
	chainReader miningChainReader,
	isMiningFunc func() bool,
	blockMineDelay time.Duration,
	setupMiningFunc func(ctx context.Context) error,
	startMiningFunc func(context.Context) error,
	stopMiningfunc func(context.Context),
	getWorkerFunc func(ctx context.Context) (mining.Worker, error),
) MiningAPI {
	return MiningAPI{
		minerAddress:    minerAddr,
		addNewBlockFunc: addNewBlockFunc,
		chainReader:     chainReader,
		isMiningFunc:    isMiningFunc,
		mineDelay:       blockMineDelay,
		setupMiningFunc: setupMiningFunc,
		startMiningFunc: startMiningFunc,
		stopMiningFunc:  stopMiningfunc,
		getWorkerFunc:   getWorkerFunc,
	}
}

// MinerAddress returns the mining address the MiningAPI is using, an error is
// returned if the mining address is not set.
func (a *MiningAPI) MinerAddress() (address.Address, error) {
	return a.minerAddress()
}

// MiningIsActive calls the node's IsMining function
func (a *MiningAPI) MiningIsActive() bool {
	return a.isMiningFunc()
}

// MiningOnce mines a single block in the given context, and returns the new block.
func (a *MiningAPI) MiningOnce(ctx context.Context) (*types.Block, error) {
	if a.isMiningFunc() {
		return nil, errors.New("Node is already mining")
	}

	ts, err := a.chainReader.GetTipSet(a.chainReader.GetHead())
	if err != nil {
		return nil, err
	}

	miningWorker, err := a.getWorkerFunc(ctx)
	if err != nil {
		return nil, err
	}

	res, err := mining.MineOnce(ctx, miningWorker, a.mineDelay, ts)
	if err != nil {
		return nil, err
	}
	if res.Err != nil {
		return nil, res.Err
	}

	if err := a.addNewBlockFunc(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}

// MiningSetup sets up a storage miner without running repeated tasks like mining
func (a *MiningAPI) MiningSetup(ctx context.Context) error {
	return a.setupMiningFunc(ctx)
}

// MiningStart calls the node's StartMining function
func (a *MiningAPI) MiningStart(ctx context.Context) error {
	return a.startMiningFunc(ctx)
}

// MiningStop calls the node's StopMining function
func (a *MiningAPI) MiningStop(ctx context.Context) {
	a.stopMiningFunc(ctx)
}
