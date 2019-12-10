package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new blocks mined by the Worker.  This is the
// interface to implement if you want to explore an alternate mining strategy.

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/pkg/errors"
)

// Scheduler is the mining interface consumers use.
type Scheduler interface {
	Start(miningCtx context.Context, skipCh chan bool) (<-chan Output, *sync.WaitGroup)
	IsStarted() bool
}

type timingScheduler struct {
	// worker contains the actual mining logic.
	worker Worker
	// pollHeadFunc is the function the scheduler uses to poll for the
	// current heaviest tipset
	pollHeadFunc func() (block.TipSet, error)
	// chainClock measures time and tracks the epoch-time relationship
	chainClock clock.ChainEpochClock

	// mu guards the skipping flag
	mu       sync.Mutex
	skipping bool

	isStarted bool
}

// Start starts mining. It takes in a channel signaling when to skip mining --
// intended to be wired to the syncer's catchup notifications -- and a context.
// It returns a channel for reading mining outputs and a waitgroup for teardown.
func (s *timingScheduler) Start(miningCtx context.Context, skipCh chan bool) (<-chan Output, *sync.WaitGroup) {
	var doneWg sync.WaitGroup
	outCh := make(chan Output, 1)

	// loop mining work
	doneWg.Add(1)
	go func() {
		s.mineLoop(miningCtx, outCh, doneWg)
		doneWg.Done()
	}()

	// loop receive skip and unskip commands
	doneWg.Add(1)
	go func() {
		for {
			select {
			case skip, ok := <-skipCh:
				if !ok {
					return
				}
				s.setSkip(skip)
			case <-miningCtx.Done():
				return
			}
		}
		doneWg.Done()
	}()

	return outCh, &doneWg
}

func (s *timingScheduler) setSkip(skip bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipping = skip
}

func (s *timingScheduler) getSkip() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.skipping
}

func (s *timingScheduler) mineLoop(miningCtx context.Context, outCh chan Output, doneWg sync.WaitGroup) {
	// mineLoop is the main event loop for the timing scheduler.  It waits for
	// a new epoch to start, polls the heaviest head, includes the correct number
	// of null blocks and starts a mining job async.
	//
	// If the previous epoch's mining job is not finished it is canceled via the context
	//
	// The scheduler will skip mining jobs if the skipping flag is set
	workContext, workCancel := context.WithCancel(miningCtx)
	for {
		s.waitForEpochStart(miningCtx)
		workCancel()     // cancel any previous mining work
		if s.getSkip() { // don't mine if we are skipping
			continue
		}

		workContext, workCancel = context.WithCancel(miningCtx)
		base, err := s.pollHeadFunc()
		if err != nil {
			log.Errorf("error polling head from mining scheduler %s", err)
		}
		h, err := base.Height()
		if err != nil {
			log.Errorf("error getting height from base", err)
		}
		nullBlkCount := s.calcNullBlks(h)
		doneWg.Add(1)
		go func() {
			s.worker.Mine(workContext, base, nullBlkCount, outCh)
			doneWg.Done()
		}()
	}
}

func (s *timingScheduler) waitForEpochStart(miningCtx context.Context) {
	// waitForEpochStart blocks until the next epoch begins
	currEpoch := uint64(s.chainClock.EpochAtTime(s.chainClock.Now()))
	nextEpoch := currEpoch + 1
	nextEpochStart := s.chainClock.StartTimeOfEpoch(nextEpoch)
	waitDur := nextEpochStart.Sub(s.chainClock.Now())
	newEpochCh := s.chainClock.After(waitDur)
	select {
	case <-newEpochCh:
		return
	case <-miningCtx.Done():
		s.isStarted = false
		return
	}
}

func (s *timingScheduler) calcNullBlks(baseHeight uint64) uint64 {
	currEpoch := uint64(s.chainClock.EpochAtTime(s.chainClock.Now()))
	return currEpoch - baseHeight - 1
}

// IsStarted is called when starting mining to tell whether the scheduler should be
// started
func (s *timingScheduler) IsStarted() bool {
	return s.isStarted
}

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker, f func() (block.TipSet, error), c clock.ChainEpochClock) Scheduler {
	return &timingScheduler{
		worker:       w,
		pollHeadFunc: f,
		chainClock:   c,
	}
}

// MineOnce is a convenience function that presents a synchronous blocking
// interface to the mining scheduler.  The worker will mine as many null blocks
// on top of the input tipset as necessary and output the winning block.
// It makes a polling function that simply returns the provided tipset.
// Then the scheduler takes this polling function, and the worker and the
// mining duration
func MineOnce(ctx context.Context, w Worker, ts block.TipSet, c clock.ChainEpochClock) (Output, error) {
	pollHeadFunc := func() (block.TipSet, error) {
		return ts, nil
	}
	s := NewScheduler(w, pollHeadFunc, c)
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	outCh, _ := s.Start(subCtx, nil)
	block, ok := <-outCh
	if !ok {
		return Output{}, errors.New("Mining completed without returning block")
	}
	return block, nil
}
