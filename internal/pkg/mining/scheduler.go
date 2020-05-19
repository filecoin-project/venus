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

// NewScheduler returns a new timingScheduler to schedule mining work on the
// input worker.
func NewScheduler(w Worker, f func() (block.TipSet, error), c clock.ChainEpochClock) Scheduler {
	return &timingScheduler{
		worker:       w,
		pollHeadFunc: f,
		chainClock:   c,
	}
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
		err := s.mineLoop(miningCtx, outCh)
		if err != nil {
			outCh <- NewOutputErr(err)
		}
		doneWg.Done()
	}()
	s.isStarted = true

	return outCh, &doneWg
}

func (s *timingScheduler) mineLoop(ctx context.Context, outCh chan Output) error {
	mineCh := make(chan Output, 1)

	// start on epoch boundary
	targetEpoch := s.chainClock.WaitNextEpoch(ctx)
	if s.isDone(ctx) {
		return nil
	}

	// The main event loop for the timing scheduler.
	// Waits for a new epoch to start, polls the heaviest head, includes the correct number
	// of null blocks and starts a mining job async.
	//
	// The scheduler will skip mining jobs if the skipping flag is set
	for {
		// wait for prop delay after epoch start for parent blocks to arrive
		s.chainClock.WaitForEpochPropDelay(ctx, targetEpoch)
		if s.isDone(ctx) {
			return nil
		}

		// our target is now the next epoch
		targetEpoch++

		// continue if we are skipping
		if s.isSkipping() {
			continue
		}

		// Check for a new base tipset, and reset null count if one is found.
		base, err := s.pollHeadFunc()
		if err != nil {
			return errors.Wrap(err, "error polling head from mining scheduler")
		}

		baseHeight, err := base.Height()
		if err != nil {
			log.Errorf("error getting height from base", err)
		}

		// block time validation should prevent us from seeing a base height than we expect
		if baseHeight >= targetEpoch {
			log.Errorf("Scheduled target epoch %d is not greater than base height %d", targetEpoch, baseHeight)
		}

		// null count is the number of epochs between the one we are mining and the one now.
		nullCount := uint64(targetEpoch-baseHeight) - 1

		// mine now
		s.worker.Mine(ctx, base, nullCount, mineCh)
		out := <-mineCh

		// wait until target to send
		s.chainClock.WaitForEpoch(ctx, targetEpoch)
		if s.isDone(ctx) {
			return nil
		}

		// send output at epoch boundary
		outCh <- out
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

// check if context is done. Should be called after every wait.
func (s *timingScheduler) isDone(ctx context.Context) bool {
	select { // check for interrupt during waiting
	case <-ctx.Done():
		s.isStarted = false
		return true
	default:
	}
	return false
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
