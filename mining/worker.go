package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

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
	baseBlock      *types.Block
	blockGenerator BlockGenerator
	stateTree      types.StateTree
}

// NewWorker instantiates a new Worker.
func NewWorker(baseBlock *types.Block, blockGenerator BlockGenerator, stateTree types.StateTree) *Worker {
	// @why It's either this or passing in a StateTreeGetter function.
	// Personally I like this solution but I suspect you prefer that
	// we load the statetree here....
	stateTreeCid, err := stateTree.Flush(context.Background())
	if err != nil || !stateTreeCid.Equals(baseBlock.StateRoot) {
		panic("block.stateroot != statetree")
	}
	return &Worker{baseBlock, blockGenerator, stateTree}
}

// Start is the main entrypoint for Worker. Call it to start mining. Exactly
// one Result is sent into the returned channel.
func (w *Worker) Start(ctx context.Context) <-chan Result {
	// TODO respect context
	// TODO respect new blocks
	// TODO periodicity
	// TODO Stop()
	resCh := make(chan Result)
	go mineFunc(ctx, w.baseBlock, w.stateTree, w.blockGenerator, resCh)
	return resCh
}

// mine does the actual work. mine sends exactly one result on the
// given result channel so it should be launched into a goroutine
// by the caller. mine is broken out into a separate function to
// be make it easier to test Worker.
func mine(ctx context.Context, baseBlock *types.Block, stateTree types.StateTree, blockGenerator BlockGenerator, resCh chan<- Result) {
	next, err := blockGenerator.Generate(ctx, baseBlock, stateTree)
	if err == nil {
		resCh <- NewResult(next, nil)
	} else {
		resCh <- NewResult(nil, err)
	}
}

var mineFunc = mine
