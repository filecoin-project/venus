package mining

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"

	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
)

var (
	ticketDomain *big.Int
)

func init() {
	ticketDomain = &big.Int{}
	ticketDomain.Exp(big.NewInt(2), big.NewInt(256), nil)
	ticketDomain.Sub(ticketDomain, big.NewInt(1))
}

// Input is the TipSets the worker should mine on, the address
// to accrue rewards to, and a context that the caller can use
// to cancel this mining run.
type Input struct {
	Ctx    context.Context
	TipSet core.TipSet
	// TODO: should this contain both Mining and Reward Addresses?
	MiningAddress types.Address
	RewardAddress types.Address
}

// NewInput instantiates a new Input.
func NewInput(ctx context.Context, ts core.TipSet, a types.Address, m types.Address) Input {
	return Input{Ctx: ctx, TipSet: ts, RewardAddress: a, MiningAddress: m}
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
	createPoST     DoSomeWorkFunc // TODO: rename createPoSTFunc?
	mine           mineFunc
	nullBlockTimer NullBlockTimerFunc
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
	return NewWorkerWithDeps(blockGenerator, Mine, createPoST, nullBlockTimer)
}

// NewWorkerWithDeps instantiates a new Worker with custom functions.
func NewWorkerWithDeps(blockGenerator BlockGenerator, mine mineFunc, createPoST DoSomeWorkFunc, nullBlockTimer NullBlockTimerFunc) Worker {
	return &AsyncWorker{
		blockGenerator: blockGenerator,
		createPoST:     createPoST,
		mine:           mine,
		nullBlockTimer: nullBlockTimer,
	}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the worker.
func MineOnce(ctx context.Context, w Worker, ts core.TipSet, rewardAddress, miningAddress types.Address) Output {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := w.Start(subCtx)
	go func() { inCh <- NewInput(subCtx, ts, rewardAddress, miningAddress) }()
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
		for {
			select {
			case <-miningCtx.Done():
				currentRunCancel()
				close(outCh)
				return
			case input, ok := <-inCh:
				if ok {
					// TODO(EC): implement the mining logic described in the spec here:
					//   https://github.com/filecoin-project/specs/pull/71/files#diff-a7e9cad7bc42c664eb72d7042276a22fR83
					//   specifically:
					currentRunCancel()
					currentRunCtx, currentRunCancel = context.WithCancel(input.Ctx)
					doneWg.Add(1)
					go func() {
						w.mine(currentRunCtx, input, w.nullBlockTimer, w.blockGenerator, w.createPoST, outCh)
						doneWg.Done()
					}()
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

type mineFunc func(ctx context.Context, input Input, nullBlockTimer NullBlockTimerFunc, bg BlockGenerator, createPoST DoSomeWorkFunc, out chan<- Output)

// NullBlockTimerFunc blocks until it is time to add a null block.
type NullBlockTimerFunc func()

// Mine does the actual work. It's the implementation of worker.mine.
func Mine(ctx context.Context, input Input, nullBlockTimer NullBlockTimerFunc, blockGenerator BlockGenerator, createPoST DoSomeWorkFunc, outCh chan<- Output) {
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)

	// TODO: derive these from actual storage power.
	// This means broadening the scope of the State function
	// and powerTableView from the generator to the worker.
	const myPower = 1
	const totalPower = 5

	for nullBlockCount := uint64(0); ; nullBlockCount++ {
		if ctx.Err() != nil {
			break
		}

		challenge := createChallenge(input.TipSet, nullBlockCount)
		proof := createProof(challenge, createPoST)
		ticket := createTicket(proof)

		// TODO: Test the interplay of isWinningTicket() and createPoST()
		if isWinningTicket(ticket, myPower, totalPower) {
			next, err := blockGenerator.Generate(ctx, input.TipSet, ticket, nullBlockCount, input.RewardAddress, input.MiningAddress)
			if err == nil {
				log.SetTag(ctx, "block", next)
			}

			// TODO(EC): Consider what to do if we have found a winning ticket and are mining with
			// it and a new tipset comes in with greater height. Currently Worker.Start() will cancel us.
			// We should instead let the successful run proceed unless the context is explicitly canceled.
			if ctx.Err() == nil {
				outCh <- NewOutput(next, err)
			} else {
				log.Warningf("Abandoning successfully mined block without publishing: %s", input.TipSet.String())
			}

			break
		}

		nullBlockTimer()
	}
}

func createChallenge(parents core.TipSet, nullBlockCount uint64) []byte {
	// Find the smallest ticket from parent set
	var smallest types.Signature
	for _, v := range parents {
		if smallest == nil || bytes.Compare(v.Ticket, smallest) < 0 {
			smallest = v.Ticket
		}
	}

	buf := make([]byte, 4)
	n := binary.PutUvarint(buf, nullBlockCount)
	buf = append(smallest, buf[:n]...)

	h := sha256.Sum256(buf)
	return h[:]
}

func createProof(challenge []byte, createPoST DoSomeWorkFunc) []byte {
	// TODO: Actually use the results of the PoST once it is implemented.
	createPoST()
	return challenge
}

func createTicket(proof []byte) []byte {
	h := sha256.Sum256(proof)
	// TODO: sign h once we have keys.
	return h[:]
}

var isWinningTicket = func(ticket []byte, myPower, totalPower int64) bool {
	// See https://github.com/filecoin-project/aq/issues/70 for an explanation of the math here.
	lhs := &big.Int{}
	lhs.SetBytes(ticket)
	lhs.Mul(lhs, big.NewInt(totalPower))

	rhs := &big.Int{}
	rhs.Mul(big.NewInt(myPower), ticketDomain)

	return lhs.Cmp(rhs) < 0
}

// How long the node's mining Worker should sleep to simulate mining.
const mineSleepTime = time.Millisecond * 10

// createPoST is the default implementation of DoSomeWorkFunc. Contrary to the
// advertisement, it doesn't do anything yet.
func createPoST() {
	time.Sleep(mineSleepTime)
}

// nullBlockTimer is the default implementation of NullBlockTimerFunc.
func nullBlockTimer() {
	time.Sleep(mineSleepTime)
}
