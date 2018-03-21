package mining

import (
	"context"
	"sync"
	"time"

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

// AsyncWorker implements the plumbing that drives mining.
type AsyncWorker struct {
	blockGenerator BlockGenerator
	// doSomeWork is a function like Sleep() that we call to simulate mining.
	doSomeWork DoSomeWorkFunc
	mine       mineFunc
}

// Worker is the mining interface consumers use. When you Start() a worker
// it returns two channels (inCh, outCh) and a sync.WaitGroup:
//   - inCh: callers send new blocks to mine on top of to this channel
//   - outCh: the worker sends Results to the caller on this channel
//   - doneWg: signals that the worker and any mining runs it launched
//             have stopped. (Context cancelation happens async, so you
//             need some way to know when it has actually stopped.)
//
// Once Start()ed, the Worker can be stopped by canceling its context which
// will signal on doneWg when it's actually done.
type Worker interface {
	Start(ctx context.Context) (chan<- *types.Block, <-chan Result, *sync.WaitGroup)
}

// NewWorker instantiates a new Worker.
func NewWorker(blockGenerator BlockGenerator) Worker {
	return NewWorkerWithMineAndWork(blockGenerator, Mine, func() {})
}

// NewWorkerWithMineAndWork instantiates a new Worker with custom functions.
func NewWorkerWithMineAndWork(blockGenerator BlockGenerator, mine mineFunc, doSomeWork DoSomeWorkFunc) Worker {
	return &AsyncWorker{
		blockGenerator: blockGenerator,
		doSomeWork:     doSomeWork,
		mine:           mine,
	}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the worker.
func MineOnce(ctx context.Context, w Worker, baseBlock *types.Block) Result {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := w.Start(subCtx)
	go func() { inCh <- baseBlock }()
	return <-outCh
}

// BlockGetterFunc gets a block.
type BlockGetterFunc func() *types.Block

// MineEvery is a convenience function to mine by pulling a block from
// a getter periodically. (This as opposed to Start() which runs on
// demand, whenever a block is pushed to it through the input channel).
func MineEvery(ctx context.Context, w Worker, period time.Duration, getBlock BlockGetterFunc) (chan<- *types.Block, <-chan Result, *sync.WaitGroup) {
	inCh, outCh, doneWg := w.Start(ctx)

	doneWg.Add(1)
	go func() {
		defer func() {
			doneWg.Done()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(period):
				go func() { inCh <- getBlock() }()
			}
		}
	}()

	return inCh, outCh, doneWg
}

// Start is the main entrypoint for Worker. Call it to start mining. It returns
// two channels: an input channel for blocks and an output channel for results.
// It also returns a waitgroup that will signal that all mining runs have
// completed. Each block that is received on the input channel causes the
// worker to cancel the context of the previous mining run if any and start
// mining on the new block. Any results are sent into its output channel.
// Closing the input channel does not cause the worker to stop; cancel its
// context to stop the worker.
func (w *AsyncWorker) Start(ctx context.Context) (chan<- *types.Block, <-chan Result, *sync.WaitGroup) {
	inCh := make(chan *types.Block)
	outCh := make(chan Result)
	var doneWg sync.WaitGroup

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		var currentRunCtx context.Context
		var currentRunCancel = func() {}
		var currentBlock *types.Block
		for {
			select {
			case <-ctx.Done():
				currentRunCancel()
				close(outCh)
				return
			case newBaseBlock := <-inCh:
				if currentBlock == nil || currentBlock.Score() <= newBaseBlock.Score() {
					currentRunCancel()
					currentRunCtx, currentRunCancel = context.WithCancel(ctx)
					doneWg.Add(1)
					go func() {
						w.mine(currentRunCtx, newBaseBlock, w.blockGenerator, w.doSomeWork, outCh)
						doneWg.Done()
					}()
					currentBlock = newBaseBlock
				}
			}
		}
	}()
	return inCh, outCh, &doneWg
}

// DoSomeWorkFunc is a dummy function that mimics doing something time-consuming
// in the mining loop. Pass a function that calls Sleep() is a good idea.
// TODO This should take at least take a context. Ideally, it would take a context
// and a block, and return a block (with a random nonce or something set on it).
// The operation in most blockchains is called 'seal' but we have other uses
// for that name.
type DoSomeWorkFunc func()

type mineFunc func(context.Context, *types.Block, BlockGenerator, DoSomeWorkFunc, chan<- Result)

// Mine does the actual work. It's the implementation of worker.mine.
func Mine(ctx context.Context, baseBlock *types.Block, blockGenerator BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Result) {
	next, err := blockGenerator.Generate(ctx, baseBlock)
	if err == nil {
		doSomeWork()
	}
	outCh <- NewResult(next, err)
}
