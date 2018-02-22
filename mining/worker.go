package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

var log = logging.Logger("mining.Worker")

// Result is the result of a single mining run. It has either a new
// block or an error, mimicing the golang (retVal, error) pattern.
type Result struct {
	NewBlock *types.Block
	Err      error
}

// NewResult instantiates a new MiningResult.
func NewResult(b *types.Block, e error) Result {
	return Result{NewBlock: b, Err: e}
}

// Worker mines. At the moment it does a single mining run.
type Worker struct {
	blockGenerator BlockGenerator
	getStateTree   GetStateTree
}

// GetStateTree is a function that gets a state tree by cid. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, *cid.Cid) (types.StateTree, error)

// NewWorker instantiates a new Worker.
func NewWorker(blockGenerator BlockGenerator, getStateTree GetStateTree) (*Worker) {
	return &Worker{blockGenerator, getStateTree}
}

// Start is the main entrypoint for Worker. Call it to start mining. Exactly
// one Result is sent into the returned channel.
func (w *Worker) Start(ctx context.Context, baseBlock *types.Block) (resCh chan Result) {
	// TODO respect context
	// TODO respect new blocks
	// TODO periodicity
	// TODO Stop()
	resCh = make(chan Result)
	stateTree, err := w.getStateTree(ctx, baseBlock.StateRoot)
	if err != nil {
		err := errors.Wrap(err, "Couldn't load state tree")
		go func() { resCh <- NewResult(nil, err) }()
		return
	}

	go mineFunc(ctx, baseBlock, stateTree, w.blockGenerator, resCh)
	return
}

// mine does the actual work. mine sends exactly one result on the
// given result channel so it should be launched into a goroutine
// by the caller. mine is broken out into a separate function to
// be make it easier to test Worker.
func mine(ctx context.Context, baseBlock *types.Block, stateTree types.StateTree, blockGenerator BlockGenerator, resCh chan<- Result) {
	next, err := blockGenerator.Generate(ctx, baseBlock, stateTree)
	resCh <- NewResult(next, err)
}

var mineFunc = mine
