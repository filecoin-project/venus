package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new blocks mined by the Worker.  This is the
// interface to implement if you want to explore an alternate mining strategy.

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
)

// Scheduler is the mining interface consumers use.
type Scheduler interface {
	Start(miningCtx context.Context) (<-chan Output, *sync.WaitGroup)
	IsStarted() bool
	Pause()
	Continue()
}

type timingScheduler struct {
	// worker contains the actual mining logic.
	worker Worker
	// pollHeadFunc is the function the scheduler uses to poll for the
	// current heaviest tipset
	pollHeadFunc func() (block.TipSet, error)
	// chainClock measures time and tracks the epoch-time relationship
	chainClock clock.ChainEpochClock

	// mu protects skipping
	mu sync.Mutex
	// skipping tracks whether we should skip mining
	skipping bool

	isStarted bool
}

// Start starts mining taking in a context.
// It returns a channel for reading mining outputs and a waitgroup for teardown.
// It is the callers responsibility to close the out channel only after waiting
// on the waitgroup.
func (s *timingScheduler) Start(miningCtx context.Context) (<-chan Output, *sync.WaitGroup) {
	var doneWg sync.WaitGroup
	outCh := make(chan Output, 1)

	// loop mining work
	doneWg.Add(1)
	go func() {
		s.mineLoop(miningCtx, outCh, &doneWg)
		doneWg.Done()
	}()
	s.isStarted = true

	return outCh, &doneWg
}

func (s *timingScheduler) mineLoop(miningCtx context.Context, outCh chan Output, doneWg *sync.WaitGroup) {
	// mineLoop is the main event loop for the timing scheduler.  It waits for
	// a new epoch to start, polls the heaviest head, includes the correct number
	// of null blocks and starts a mining job async.
	//
	// If the previous epoch's mining job is not finished it is canceled via the context
	//
	// The scheduler will skip mining jobs if the skipping flag is set
	workContext, workCancel := context.WithCancel(miningCtx) // nolint:staticcheck
	for {
		currEpoch := s.chainClock.WaitNextEpoch(miningCtx)
		select { // check for interrupt during waiting
		case <-miningCtx.Done():
			s.isStarted = false
			return // nolint:govet
		default:
		}
		workCancel() // cancel any late work from last epoch

		// check if we are skipping and don't mine if so
		if s.isSkipping() {
			continue
		}

		workContext, workCancel = context.WithCancel(miningCtx) // nolint: govet
		base, err := s.pollHeadFunc()
		if err != nil {
			log.Errorf("error polling head from mining scheduler %s", err)
		}
		h, err := base.Height()
		if err != nil {
			log.Errorf("error getting height from base", err)
		}
		nullBlkCount := uint64(currEpoch-h) - 1
		doneWg.Add(1)
		go func(ctx context.Context) {
			// Mine is not intended to be re-entrant, but using a go-routine here makes it impossible to enforce
			// single-threaded access.
			// Some context in https://github.com/filecoin-project/go-filecoin/issues/4065
			s.worker.Mine(ctx, base, nullBlkCount, outCh)
			doneWg.Done()
		}(workContext)
	}
}

func (s *timingScheduler) isSkipping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.skipping
}

// IsStarted is called when starting mining to tell whether the scheduler should be
// started
func (s *timingScheduler) IsStarted() bool {
	return s.isStarted
}

// Pause is called to pause the scheduler from running mining work
func (s *timingScheduler) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipping = true
}

// Continue is called to unpause the scheduler's mining work
func (s *timingScheduler) Continue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipping = false
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

// MineOnce mines on a given base until it finds a winner.
func MineOnce(ctx context.Context, w DefaultWorker, ts block.TipSet) Output {
	var nullCount uint64
	for {
		out := MineOneEpoch(ctx, w, ts, nullCount)
		if out.Err != nil || out.Header != nil {
			return out
		}
		nullCount++
	}
}

// MineOneEpoch attempts to mine a block in an epoch and returns the mined block,
// or nil if no block could be mined
func MineOneEpoch(ctx context.Context, w DefaultWorker, ts block.TipSet, nullCount uint64) Output {
	workCtx, workCancel := context.WithCancel(ctx)
	defer workCancel()
	outCh := make(chan Output, 1)

	won := w.Mine(workCtx, ts, nullCount, outCh)
	if !won {
		return NewOutputEmpty()
	}
	out, ok := <-outCh
	if !ok {
		return NewOutputErr(errors.New("Mining completed without returning block"))
	}
	return out
}
