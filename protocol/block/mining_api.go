package block

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/types"
)

type miningChainReader interface {
	GetHead() types.SortedCidSet
	GetTipSet(tsKey types.SortedCidSet) (*types.TipSet, error)
}

// MiningAPI provides an interface to the block mining protocol.
type MiningAPI struct {
	addNewBlockFunc  func(context.Context, *types.Block) (err error)
	chainReader      miningChainReader
	mineDelay        time.Duration
	startMiningFunc  func(context.Context) error
	stopMiningFunc   func(context.Context)
	createWorkerFunc func(ctx context.Context) (mining.Worker, error)
}

// New creates a new MiningAPI instance with the provided deps
func New(
	addNewBlockFunc func(context.Context, *types.Block) (err error),
	chainReader miningChainReader,
	blockMineDelay time.Duration,
	startMiningFunc func(context.Context) error,
	stopMiningfunc func(context.Context),
	createWorkerFunc func(ctx context.Context) (mining.Worker, error),
) MiningAPI {
	return MiningAPI{
		addNewBlockFunc:  addNewBlockFunc,
		chainReader:      chainReader,
		mineDelay:        blockMineDelay,
		startMiningFunc:  startMiningFunc,
		stopMiningFunc:   stopMiningfunc,
		createWorkerFunc: createWorkerFunc,
	}
}

// MiningOnce mines a single block in the given context, and returns the new block.
func (a *MiningAPI) MiningOnce(ctx context.Context) (*types.Block, error) {
	ts, err := a.chainReader.GetTipSet(a.chainReader.GetHead())
	if err != nil {
		return nil, err
	}

	miningWorker, err := a.createWorkerFunc(ctx)
	if err != nil {
		return nil, err
	}

	res, err := mining.MineOnce(ctx, miningWorker, a.mineDelay, *ts)
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

// MiningStart calls the node's StartMining function
func (a *MiningAPI) MiningStart(ctx context.Context) error {
	return a.startMiningFunc(ctx)
}

// MiningStop calls the node's StopMining function
func (a *MiningAPI) MiningStop(ctx context.Context) {
	a.stopMiningFunc(ctx)
}
