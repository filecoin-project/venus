package mining

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// Input is the TipSets the worker should mine on, the address
// to accrue rewards to, and a context that the caller can use
// to cancel this mining run.
type Input struct {
	Ctx     context.Context
	TipSets []core.TipSet
	// TODO This should apparently be a miner actor address.
	RewardAddress types.Address
}

// NewInput instantiates a new Input.
func NewInput(ctx context.Context, tipSets []core.TipSet, a types.Address) Input {
	return Input{Ctx: ctx, TipSets: tipSets, RewardAddress: a}
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
	return NewWorkerWithDeps(blockGenerator, Mine, func() {})
}

// NewWorkerWithDeps instantiates a new Worker with custom functions.
func NewWorkerWithDeps(blockGenerator BlockGenerator, mine mineFunc, doSomeWork DoSomeWorkFunc) Worker {
	return &AsyncWorker{
		blockGenerator: blockGenerator,
		doSomeWork:     doSomeWork,
		mine:           mine,
	}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the worker.
func MineOnce(ctx context.Context, w Worker, tipSets []core.TipSet, rewardAddress types.Address) Output {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := w.Start(subCtx)
	go func() { inCh <- NewInput(subCtx, tipSets, rewardAddress) }()
	return <-outCh
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
//
// TODO A potentially simpler interface here would be for the worker to
// take the input channel from the caller and then shut everything down
// when the input channel is closed.
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
					// TODO(aa) select the heaviest tipset
					// TODO(aa) implement the mining logic described in the spec here:
					//   https://github.com/filecoin-project/specs/pull/71/files#diff-a7e9cad7bc42c664eb72d7042276a22fR83
					//   specifically:
					//     - the spec suggests to "wait a little bit" when we see a tipset at a greater
					//       height than the one we're working on. However it's probably just easier to
					//       starting mining as soon as we see a tipset from a greater height and then
					//       cancel it and start over when we see a tipset at that height with greater
					//       weight. So replace the score check below that cancels the mining run
					//       with one that cancels and starts a new run if we are currently not running
					//       (below as currentBlock == nil), if we see a tipset from a greater height
					//       (replaces score check below), or if we see a tipset from the same height
					//       but with greater weight. I say ignore for now rational miner strategies that
					//       wouldn't abandon a lesser-weight mining run that wins the lottery.
					newBaseBlock := core.BaseBlockFromTipSets(input.TipSets)
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
// in the mining loop such as computing proofs. Pass a function that calls Sleep()
// is a good idea for now.
type DoSomeWorkFunc func()

type mineFunc func(context.Context, Input, BlockGenerator, DoSomeWorkFunc, chan<- Output)

// Mine does the actual work. It's the implementation of worker.mine.
func Mine(ctx context.Context, input Input, blockGenerator BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)

	// TODO(aa) Check ticket. If not a winner then wait (how long?!) and add a null block
	// and try again. Keep doing that until we get canceled or a winner is found. When
	// a winner is found proceed with block creation:
	// https://github.com/filecoin-project/specs/pull/71/files#diff-a7e9cad7bc42c664eb72d7042276a22fR87
	// DoSomeWorkFunc is a placeholder for the thing that computes proofs.

	// TODO(aa) Generate()'s signature might want to change. In order to limit the scope
	// of changes I'm just pulling a block out here and passing it in.
	newBaseBlock := core.BaseBlockFromTipSets(input.TipSets)
	next, err := blockGenerator.Generate(ctx, newBaseBlock, input.RewardAddress)
	if err == nil {
		log.SetTag(ctx, "block", next)
		// TODO whatever happens here should respect the context, but see caveat below.
		doSomeWork()
	}
	// TODO(aa) Consider what to do if we have found a winning ticket and are mining with
	// it and a new tipset comes in with greater height. Unless we change the logic the
	// successful mining run will be canceled if it is still in flight. We should probably
	// let the successful run proceed unless the context is explicitly canceled.
	if ctx.Err() == nil {
		outCh <- NewOutput(next, err)
	}
}
