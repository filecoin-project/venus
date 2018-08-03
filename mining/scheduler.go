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

	"github.com/filecoin-project/go-filecoin/consensus"
)

// Scheduler is the mining interface consumers use. When you Start() the
// scheduler it returns two channels (inCh, outCh) and a sync.WaitGroup:
//   - inCh: the caller sends Inputs to mine on to this channel.
//   - outCh: the scheduler sends Outputs to the caller on this channel.
//   - doneWg: signals that the scheduler and any goroutines it launched
//             have stopped. (Context cancelation happens async, so you
//             need some way to know when it has actually stopped.)
//
// Once Start()ed, the Scheduler can be stopped by canceling its miningCtx,
// which will signal on doneWg when it's actually done. Canceling miningCtx
// cancels any run in progress and shuts the scheduler down.
type Scheduler interface {
	Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup)
}

type timingScheduler struct {
	// base tracks the current tipset being mined on.  It is only read and
	// written from the timingScheduler's main receive loop, i.e. collect
	// and ignore.
	base Input
	// worker contains the actual mining logic.
	worker Worker
	// mineDelay is the time the scheduler blocks for collection.
	mineDelay time.Duration
}

// MineDelayConversionFactor is the constant that divides the mining block time
// to arrive and the mining delay.  TODO: get a legit value for this param.
const MineDelayConversionFactor = 30

// runWorker launches calls to worker.Mine().  Inputs to worker.Mine() are
// accepted on mineInCh.  For each new input on mineInCh, runWorker cancels the
// old call to worker.Mine() and mines on the new input.  There is only one
// outstanding call to worker.Mine() at a time.  outCh is the channel read by
// the scheduler's caller. Newly mined blocks are sent out on outCh if the
// worker is able to mine a block.  Nothing is output over outCh if the worker
// does not mine a block.  If there is a mining error it is sent over the
// outCh.
func (s *timingScheduler) runWorker(miningCtx context.Context, outCh chan<- Output, mineInCh <-chan Input, doneWg *sync.WaitGroup) {
	defer doneWg.Done()
	var currentRunCtx context.Context
	currentRunCancel := func() {}
	for {
		select {
		case <-miningCtx.Done():
			currentRunCancel()
			return
		case input, ok := <-mineInCh:
			if !ok {
				// sender closed mineCh, close and ignore
				mineInCh = nil
				continue
			}
			currentRunCancel()
			currentRunCtx, currentRunCancel = context.WithCancel(miningCtx)
			doneWg.Add(1)
			go func() {
				defer doneWg.Done()
				s.worker.Mine(currentRunCtx, input, outCh)
			}()
		}
	}
}

// collect runs for the s.mineDelay and updates the base
// tipset for mining to the latest tipset read from the input channel.
// If the scheduler should terminate collect() returns true. collect()
// initializes the next round of mining, canceling any previous mining calls
// still running. If the eager flag is set, collect starts mining right away,
// possibly starting and stopping multiple mining jobs.
func (s *timingScheduler) collect(miningCtx context.Context, inCh <-chan Input, mineInCh chan<- Input, eager bool) bool {
	delayTimer := time.NewTimer(s.mineDelay)
	for {
		select {
		case <-miningCtx.Done():
			return true
		case <-delayTimer.C:
			if !eager {
				mineInCh <- s.base
			}
			return false
		case input, ok := <-inCh:
			if !ok {
				// sender closed inCh, close and ignore
				inCh = nil
				continue
			}

			log.Infof("scheduler receiving new base %s during collect", input.TipSet.String())
			s.base = input
			if eager {
				mineInCh <- input
			}
		}
	}
}

// ignore() waits for a new heaviest tipset with a greater height. No new tipsets
// from the current base tipset's height or lower heights are accepted as the
// new mining base.  If the scheduler should terminate ignore() returns true.
// The purpose of the ignore state is to de-incentivize strategic lagging of
// block propagation part way through a proving period which can cause
// competing miners to waste work.
// Note: this strategy is limited, it does not prevent witholding tipsets of
// greater heights than the current base or from witholding tipsets from miners
// with a null block mining base.
func (s *timingScheduler) ignore(miningCtx context.Context, inCh <-chan Input) bool {
	for {
		select {
		case <-miningCtx.Done():
			return true
		case input, ok := <-inCh:
			if !ok {
				inCh = nil
				continue
			}

			// Scheduler begins in ignore state with a nil base.
			// This case handles base init on the very first input.
			if s.base.TipSet == nil {
				s.base = input
				return false
			}

			curHeight, err := s.base.TipSet.Height()
			if err != nil {
				panic("this can't be happening")
			}
			inHeight, err := input.TipSet.Height()
			if err != nil {
				panic("this can't be happening")
			}

			// Newer epoch? Loop back to collect state for new non-null epoch.
			if inHeight > curHeight {
				log.Infof("scheduler receiving new base %s during ignore, transition to collect", input.TipSet.String())
				s.base = input
				return false
			}
			log.Debugf("scheduler ignoring %s during ignore because height is not greater than height of current base", input.TipSet.String())
		}
	}
}

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
func (s *timingScheduler) Start(miningCtx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup) {
	inCh := make(chan Input)
	outCh := make(chan Output)
	mineInCh := make(chan Input)
	var doneWg sync.WaitGroup    // for internal use
	var extDoneWg sync.WaitGroup // for external use

	log.Debugf("scheduler starting 'runWorker' goroutine")
	doneWg.Add(1)
	go s.runWorker(miningCtx, outCh, mineInCh, &doneWg)

	log.Debugf("scheduler starting main receive loop")
	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		// for now keep 'eager' unset. TODO eventually pull from config or CLI flag.
		eager := false
		// The receive loop. The scheduler operates in two basic states
		//   collect  -- wait for the mining delay to listen for better base tipsets
		//   ignore -- mine on the best tipset, ignore all tipsets with height <= the height of the current base
		for {
			if end := s.ignore(miningCtx, inCh); end {
				return
			}
			if end := s.collect(miningCtx, inCh, mineInCh, eager); end {
				return
			}
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
		close(mineInCh)
	}()
	return inCh, outCh, &extDoneWg
}

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker, md time.Duration) Scheduler {
	return &timingScheduler{worker: w, mineDelay: md}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the mining scheduler.
func MineOnce(ctx context.Context, s Scheduler, ts consensus.TipSet) Output {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	inCh, outCh, _ := s.Start(subCtx)
	go func() { inCh <- NewInput(ts) }()
	return <-outCh
}
