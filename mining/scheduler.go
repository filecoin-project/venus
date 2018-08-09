package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new successful blocks.  This is the interface to
// implement if you want to explore an alternate mining strategy.
//
// The default Scheduler implementation, timingScheduler, operates in two
// states, 'collect', where the scheduler listens for new heaviest tipsets to
// replace the mining base, and 'ignore', where mining proceeds uninterrupted.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/core"
)

// Scheduler is the mining interface consumers use. When you Start() the
// scheduler it returns two channels (inCh, outCh) and a sync.WaitGroup:
//   - inCh: the caller sends Inputs to mine on to this channel.
//   - outCh: the worker sends Outputs to the caller on this channel.
//   - doneWg: signals that the scheduler and any goroutines it launched
//             have stopped. (Context cancelation happens async, so you
//             need some way to know when it has actually stopped.)
//
// Once Start()ed, the Scheduler can be stopped by canceling its miningCtx, which
// will signal on doneWg when it's actually done. Canceling an Input.Ctx
// just cancels the run for that input. Canceling miningCtx cancels any run
// in progress and shuts the worker down.
type Scheduler interface {
	Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup)
}

type timingScheduler struct {
	miningAt uint64
	base     Input
	state    receiveState

	worker Worker
}

type receiveState int

const (
	delay = iota
	grace
	ignore
	end
)

// mineGrace is the protocol grace period
const mineGrace = time.Second

// mineSleepTime is the protocol's estimated mining time
const mineSleepTime = mineGrace * 30

// mineDelay is the protocol mining delay at the beginning of an epoch
const mineDelay = mineGrace * 2

// runMine runs the miner, inputs to the miner's goroutine are accepted on
// mineInCh.  For each new input on mineInCh, runMine cancels the old mining
// run to run the new input.  There is only one outstanding call to the 'mine'
// function at a time.  Newly mined valid blocks are sent out on outCh. A
// signal indicating that a run of 'mine' was completed without interruption,
// even if it did not result in a valid block, is sent on mineOutCh.
func (s *timingScheduler) runMine(miningCtx context.Context, outCh chan<- Output, mineInCh <-chan Input, mineOutCh chan<- struct{}, doneWg sync.WaitGroup) {
	defer doneWg.Done()
	currentRunCancel := func() {}
	var currentRunCtx context.Context
	for {
		select {
		case <-miningCtx.Done():
			fmt.Printf("runMine: miningCtx.Done()\n")
			currentRunCancel()
			return
		case input, ok := <-mineInCh:
			fmt.Printf("runMine: mineInCh: %v\n", input)
			if ok {
				currentRunCancel()
				currentRunCtx, currentRunCancel = context.WithCancel(input.Ctx)
				doneWg.Add(1)
				go func() {
					defer doneWg.Done()
					fmt.Printf("mine: running 'mine'\n")
					s.worker.Mine(currentRunCtx, input, outCh)
					mineOutCh <- struct{}{}
				}()
			} else {
				// sender closed mineCh, close and ignore
				mineInCh = nil
			}
		}
	}
}

// delay runs for the protocol mining delay "mineDelay" and updates the base
// tipset for mining to the latest tipset read from the input channel.
// delay initializes the next round of mining, canceling any previous mining
// calls still running. If mining work from the previous period is not
// completed delay logs a warning as this indicates someone is speeding up PoST
// generation somehow.
func (s *timingScheduler) delay(miningCtx context.Context, inCh <-chan Input, mineInCh chan<- Input, mineOutCh <-chan struct{}) {
	epoch, err := s.base.TipSet.Height()
	if err != nil {
		panic("this can't be happening")
	}
	epoch += s.base.NullBlks
	// No blocks found last epoch? Add a null block for this epoch's input.
	if s.miningAt > epoch {
		input := NewInput(s.base.Ctx, s.base.TipSet)
		input.NullBlks = s.base.NullBlks + uint64(1)
		s.base = input
		epoch++
	}

	// This is unexpected and means someone is generating
	// PoSTs too fast.
	defer func() {
		if s.miningAt != epoch {
			// log.Warningf("Not enough time to mine on epochs %d to %d", s.miningAt, epoch - 1)
			s.miningAt = epoch
		}
	}()
	delayTimer := time.NewTimer(mineDelay)
	for {
		select {
		case <-miningCtx.Done():
			s.state = end
			return
		case <-delayTimer.C:
			s.state = grace
			mineInCh <- s.base
			return
		case input, ok := <-inCh:
			if !ok {
				inCh = nil
				continue
			}
			inEpoch, err := input.TipSet.Height()
			if err != nil {
				panic("this can't be happening")
			}
			// Older epoch? Do nothing. TODO: is it secure that old heavier tipsets are ignored?
			// Current epoch? Replace base.
			// Newer epoch? Loop back to DELAY state for new epoch.
			if inEpoch == epoch {
				s.base = input
			}
			if inEpoch > epoch {
				// log.Warning("miner preempted during mining delay")
				s.base = input
				epoch = inEpoch
				return
			}
		case _, ok := <-mineOutCh:
			if ok {
				s.miningAt++
			} else {
				mineOutCh = nil
			}
		}
	}
}

// grace waits for the protocol grace period "mineGrace".  During the grace
// period mining is always running.  However grace restarts mining when
// receiving new heavier base tipsets.
func (s *timingScheduler) grace(miningCtx context.Context, inCh <-chan Input, mineInCh chan<- Input) {
	epoch, err := s.base.TipSet.Height()
	if err != nil {
		panic("this can't be happening")
	}
	epoch += s.base.NullBlks

	graceTimer := time.NewTimer(mineGrace)
	for {
		select {
		case <-miningCtx.Done():
			s.state = end
			return
		case <-graceTimer.C:
			s.state = ignore
			return
		case input, ok := <-inCh:
			if !ok {
				inCh = nil
				continue
			}
			inEpoch, err := input.TipSet.Height()
			if err != nil {
				panic("this can't be happening")
			}
			// Older epoch? Do nothing. TODO: is it secure that old heavier tipsets are ignored?
			// Current epoch? Restart mining.
			// Newer epoch? Loop back to DELAY state for new epoch.
			if inEpoch == epoch {
				s.base = input
				mineInCh <- input
			}
			if inEpoch > epoch {
				// log.Warning("miner preempted during grace period")
				s.base = input
				s.state = delay
				return
			}
		}
	}
}

// ignore waits for the mining period.  During this time no new tipsets of this
// epoch are accepted as new mining bases.  ignore is finished when the mining
// period is over, or a tipset of the next epoch is input.
func (s *timingScheduler) ignore(miningCtx context.Context, inCh <-chan Input, mineOutCh <-chan struct{}) {
	epoch, err := s.base.TipSet.Height()
	if err != nil {
		panic("this can't be happening")
	}
	epoch += s.base.NullBlks

	mineTimer := time.NewTimer(mineSleepTime)
	for {
		select {
		case <-miningCtx.Done():
			s.state = end
			return
		case <-mineTimer.C:
			s.state = delay
			return
		case input, ok := <-inCh:
			if !ok {
				inCh = nil
				continue
			}
			inEpoch, err := input.TipSet.Height()
			if err != nil {
				panic("this can't be happening")
			}
			// Older epoch? Current epoch? Do nothing.
			// Newer epoch? Loop back to DELAY state for new epoch.
			if inEpoch > epoch {
				s.base = input
				s.state = delay
				return
			}
		case _, ok := <-mineOutCh:
			if ok {
				s.miningAt++
			} else {
				mineOutCh = nil
			}
		}
	}
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
func (s *timingScheduler) Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup) {
	inCh := make(chan Input)
	outCh := make(chan Output)
	mineInCh := make(chan Input)
	mineOutCh := make(chan struct{})
	var doneWg sync.WaitGroup    // for internal use
	var extDoneWg sync.WaitGroup // for external use

	doneWg.Add(1)
	go s.runMine(miningCtx, outCh, mineInCh, mineOutCh, doneWg)

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()

		// Wait for an initial value
		input := <-inCh
		s.base = input
		fmt.Printf("Start: init\n")

		// The receive loop.  The worker operates in three basic receiveStates:
		//   delay  -- wait for the mining delay to receive the best tipset for this epoch
		//   grace  -- start mining but restart off of better tipsets that arrive
		//   ignore -- ignore all tipsets from the current epoch and finish mining
		for {
			switch s.state {
			case delay:
				fmt.Printf("Start: delay\n")
				s.delay(miningCtx, inCh, mineInCh, mineOutCh)
			case grace:
				fmt.Printf("Start: grace\n")
				s.grace(miningCtx, inCh, mineInCh)
			case ignore:
				fmt.Printf("Start: ignore\n")
				s.ignore(miningCtx, inCh, mineOutCh)
			case end:
				fmt.Printf("Start: end\n")
				return
			default:
				panic("worker should never reach here")
			}
		}
	}()

	// This tear down goroutine waits for all work to be done before closing
	// channels.  When this goroutine is complete, external code can
	// consider the worker to be done.
	extDoneWg.Add(1)
	go func() {
		doneWg.Wait()
		close(outCh)
		close(mineInCh)
		close(mineOutCh)
		extDoneWg.Done()
		fmt.Printf("tear down all done\n")
	}()
	return inCh, outCh, &extDoneWg
}

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker) Scheduler {
	return &timingScheduler{worker: w}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the mining scheduler.
func MineOnce(ctx context.Context, s Scheduler, ts core.TipSet) Output {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := s.Start(subCtx)
	go func() { inCh <- NewInput(subCtx, ts) }()
	return <-outCh
}
