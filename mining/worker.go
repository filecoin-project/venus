package mining

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/types"
)

// Input is the block the worker should mine on, the address
// to accrue rewards to, and a context that the caller can use
// to cancel this mining run.
type Input struct {
	Ctx           context.Context
	MineOn        *types.Block
	RewardAddress types.Address
}

// NewInput instantiates a new Input.
func NewInput(ctx context.Context, b *types.Block, a types.Address) Input {
	return Input{Ctx: ctx, MineOn: b, RewardAddress: a}
}

// Output is the result of a single mining run. It has either a new
// block or an error, mimicing the golang (retVal, error) pattern.
// If a mining run's context is canceled there is no output.
type Output struct {
	NewBlock *types.Block
	Err      error
}

// NewOutput instantiates a new Output.
func NewOutput(b *types.Block, e error) Output {
	return Output{NewBlock: b, Err: e}
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
//   - inCh: caller	 send Inputs to mine on to this channel
//   - outCh: the worker sends Outputs to the caller on this channel
//   - doneWg: signals that the worker and any mining runs it launched
//             have stopped. (Context cancelation happens async, so you
//             need some way to know when it has actually stopped.)
//
// Once Start()ed, the Worker can be stopped by canceling its miningCtx, which
// will signal on doneWg when it's actually done. Canceling an Input.Ctx
// just cancels the run for that input. Canceling miningCtx cancels any run
// in progress and shuts the worker down.
type Worker interface {
	Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup)
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
func MineOnce(ctx context.Context, w Worker, baseBlock *types.Block, rewardAddress types.Address) Output {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := w.Start(subCtx)
	go func() { inCh <- NewInput(subCtx, baseBlock, rewardAddress) }()
	return <-outCh
}

// BlockGetterFunc gets a block.
type BlockGetterFunc func() *types.Block

// MineEvery is a convenience function to mine by pulling a block from
// a getter periodically. (This as opposed to Start() which runs on
// demand, whenever a block is pushed to it through the input channel).
func MineEvery(ctx context.Context, w Worker, period time.Duration, getBlock BlockGetterFunc, rewardAddress types.Address) (chan<- Input, <-chan Output, *sync.WaitGroup) {
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
				go func() { inCh <- NewInput(ctx, getBlock(), rewardAddress) }()
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
// Closing the input channel does not cause the worker to stop; cancel
// the Input.Ctx to cancel an individual mining run or the mininCtx to
// stop all mining and shut down the worker.
func (w *AsyncWorker) Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup) {
	inCh := make(chan Input)
	outCh := make(chan Output)
	var doneWg sync.WaitGroup

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		var currentRunCtx context.Context
		var currentRunCancel = func() {}
		var currentBlock *types.Block
		for {
			select {
			case <-miningCtx.Done():
				currentRunCancel()
				close(outCh)
				return
			case input, ok := <-inCh:
				if ok {
					newBaseBlock := input.MineOn
					if currentBlock == nil || currentBlock.Score() <= newBaseBlock.Score() {
						currentRunCancel()
						currentRunCtx, currentRunCancel = context.WithCancel(input.Ctx)
						doneWg.Add(1)
						go func() {
							w.mine(currentRunCtx, input, w.blockGenerator, w.doSomeWork, outCh)
							doneWg.Done()
						}()
						currentBlock = newBaseBlock
					}
				} else {
					// Sender closed the channel. Set it to nil to ignore it.
					inCh = nil
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

type mineFunc func(context.Context, Input, BlockGenerator, DoSomeWorkFunc, chan<- Output)

// Mine does the actual work. It's the implementation of worker.mine.
func Mine(ctx context.Context, input Input, blockGenerator BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)
	next, err := blockGenerator.Generate(ctx, input.MineOn, input.RewardAddress)
	if err == nil {
		// TODO whatever happens here, respect the context.
		log.SetTag(ctx, "block", next)
		doSomeWork()
	}
	if ctx.Err() == nil {
		outCh <- NewOutput(next, err)
	}
}
