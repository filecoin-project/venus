package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new blocks mined by the Worker.  This is the
// interface to implement if you want to explore an alternate mining strategy.
//
// The default Scheduler implementation, timingScheduler, is an attempt to
// prevent miners from getting interrupted by attackers strategically releasing
// better base tipsets midway through a proving period. Such attacks are bad
// because they causes miners to waste work.  Note that the term 'base tipset',
// or 'mining base' is used to denote the tipset that the miner uses as the
// parent of the block it attempts to generate during mining.
//
// The timingScheduler operates in two states, 'collect', where the scheduler
// listens for new heaviest tipsets to use as the best mining base, and 'ignore',
// where mining proceeds uninterrupted.  The scheduler enters the 'collect' state
// each time a new heaviest tipset arrives with a greater height.  The
// scheduler finishes the collect state after the mining delay time, a protocol
// parameter, has passed.  The scheduler then enters the 'ignore' state.  Here
// the scheduler mines, ignoring all inputs with the most recent and lower
// heights.  'ignore' concludes when the scheduler receives an input tipset,
// possibly the tipset consisting of the block the miner just mined, with
// a greater height, and transitions back to collect.  It is in miners'
// best interest to wait for the collection period so that they can wait to
// work on a base tipset made up of all blocks mined at the new height.
//
// The current approach is limited. It does not prevent wasted work from all
// strategic block witholding attacks.  This is also going to be effected by
// current unknowns surrounding the specifics of the mining protocol (i.e. how
// do VDFs and PoSTs fit into mining, and what is the lookback parameter for
// challenge sampling.  For more details see:
// https://gist.github.com/whyrusleeping/4c05fd902f7123bdd1c729e3fffed797

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// Scheduler is the mining interface consumers use. When you Start() the
// scheduler it returns a channel and a sync.WaitGroup:
//   - channel: the scheduler emits outputs (mined blocks) to the caller on this channel
//   - waitgroup: signals that the scheduler and any goroutines it launched
//             have stopped. (Context cancelation happens async, so you
//             need some way to know when it has actually stopped.)
//
// Once started, a scheduler can be stopped by canceling the context provided to `Start()`,
// which will signal on the waitgroup when it's actually done.
type Scheduler interface {
	Start(miningCtx context.Context) (<-chan Output, *sync.WaitGroup)
	IsStarted() bool
}

type timingScheduler struct {
	// worker contains the actual mining logic.
	worker Worker
	// mineDelay is the time the scheduler blocks for collection.
	mineDelay time.Duration
	// pollHeadFunc is the function the scheduler uses to poll for the
	// current heaviest tipset
	pollHeadFunc func() (types.TipSet, error)

	isStarted bool
}

// MineDelayConversionFactor is the constant that divides the mining block time
// to arrive and the mining delay.  TODO: get a legit value for this param.
const MineDelayConversionFactor = 30

// Start is the main entrypoint for the timingScheduler. Call it to start
// mining. It returns two channels: an input channel for tipsets and an output
// channel for newly mined blocks. It also returns a waitgroup that will signal
// that all mining runs and auxiliary goroutines have completed. Each tipset
// that is received on the input channel is procssesed by the scheduler.
// Depending on the tipset height and the time the input is received, a new
// input may cancel the context of the previous mining run, or be ignorned.
// Any successfully mined blocks are sent into the output channel. Closing the
// input channel does not cause the scheduler to stop; cancel the miningCtx to
// stop all mining and shut down the scheduler.
func (s *timingScheduler) Start(miningCtx context.Context) (<-chan Output, *sync.WaitGroup) {
	// we buffer 1 to make sure we do not get blocked when shutting down
	outCh := make(chan Output, 1)
	var doneWg sync.WaitGroup    // for internal use
	var extDoneWg sync.WaitGroup // for external use

	log.Debugf("Scheduler starting main receive loop")
	doneWg.Add(1)

	s.isStarted = true
	go func() {
		defer doneWg.Done()
		nullBlkCount := 0
		var prevBase types.TipSet
		var prevWon bool
		for {
			select {
			case <-miningCtx.Done():
				s.isStarted = false
				return
			default:
			}
			// This is the sleep during which we collect. TODO: maybe this should vary?
			time.Sleep(s.mineDelay)
			// Ask for the heaviest tipset.
			base, _ := s.pollHeadFunc()
			if !base.Defined() { // Don't try to mine on an unset head.
				outCh <- NewOutput(nil, errors.New("cannot mine on unset (nil) head"))
				return
			}
			if prevWon && prevBase.Equals(base) {
				// Skip this round, this likely means that the new head has not propagated yet through the system.
				// TODO: investigate if there is a better way to handle this situation.
				continue
			}

			// Determine how many null blocks we should mine with.
			nullBlkCount = nextNullBlkCount(nullBlkCount, prevBase, base)

			// Mine synchronously! Ignore all new tipsets.
			prevWon = s.worker.Mine(miningCtx, base, nullBlkCount, outCh)
			prevBase = base
		}
	}()

	// This tear down goroutine waits for all work to be done before closing
	// channels.  When this goroutine is complete, external code can
	// consider the scheduler to be done.
	extDoneWg.Add(1)
	go func() {
		defer extDoneWg.Done()
		doneWg.Wait()
		close(outCh)
	}()
	return outCh, &extDoneWg
}

// IsStarted is called when starting mining to tell whether the scheduler should be
// started
func (s *timingScheduler) IsStarted() bool {
	return s.isStarted
}

// nextNullBlkCount determines how many null blocks should be mined on top of
// the current base tipset, currBase, given the previous base, prevBase and the
// previous number of null blocks mined on the previous base, prevNullBlkCount.
func nextNullBlkCount(prevNullBlkCount int, prevBase, currBase types.TipSet) int {
	// We haven't mined on this base before, start with 0 null blocks.
	if !prevBase.Defined() {
		return 0
	}
	if prevBase.String() != currBase.String() {
		return 0
	}
	// We just mined this with prevNullBlkCount, increment.
	return prevNullBlkCount + 1
}

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker, md time.Duration, f func() (types.TipSet, error)) Scheduler {
	return &timingScheduler{worker: w, mineDelay: md, pollHeadFunc: f}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the mining scheduler.  The worker will mine as many null blocks
// on top of the input tipset as necessary and output the winning block.
// It makes a polling function that simply returns the provided tipset.
// Then the scheduler takes this polling function, and the worker and the
// mining duration
func MineOnce(ctx context.Context, w Worker, md time.Duration, ts types.TipSet) (Output, error) {
	pollHeadFunc := func() (types.TipSet, error) {
		return ts, nil
	}
	s := NewScheduler(w, md, pollHeadFunc)
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	outCh, _ := s.Start(subCtx)
	block, ok := <-outCh
	if !ok {
		return Output{}, errors.New("Mining completed without returning block")
	}
	return block, nil
}
