package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new blocks mined by the Worker.  This is the
// interface to implement if you want to explore an alternate mining strategy.

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/pkg/errors"
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
	pollHeadFunc func() (block.TipSet, error)

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
		nullBlkCount := uint64(0)
		var prevBase block.TipSet
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

func nextNullBlkCount(prevNullBlkCount uint64, prevBase, currBase block.TipSet) uint64 {
	if !prevBase.Defined() || !prevBase.Key().Equals(currBase.Key()) {
		return 0
	}

	return prevNullBlkCount + 1

}

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker, md time.Duration, f func() (block.TipSet, error)) Scheduler {
	return &timingScheduler{worker: w, mineDelay: md, pollHeadFunc: f}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the mining scheduler.  The worker will mine as many null blocks
// on top of the input tipset as necessary and output the winning block.
// It makes a polling function that simply returns the provided tipset.
// Then the scheduler takes this polling function, and the worker and the
// mining duration
func MineOnce(ctx context.Context, w Worker, md time.Duration, ts block.TipSet) (Output, error) {
	pollHeadFunc := func() (block.TipSet, error) {
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
