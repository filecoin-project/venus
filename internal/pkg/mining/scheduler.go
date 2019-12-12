package mining

// The Scheduler listens for new heaviest TipSets and schedules mining work on
// these TipSets.  The scheduler is ultimately responsible for informing the
// rest of the system about new blocks mined by the Worker.  This is the
// interface to implement if you want to explore an alternate mining strategy.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
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

	// skipping tracks whether we should skip mining
	skipping bool

	isStarted bool
}

// Start starts mining. It takes in a channel signaling when to skip mining --
// intended to be wired to the syncer's catchup notifications -- and a context.
// It returns a channel for reading mining outputs and a waitgroup for teardown.
// It is the callers responsibility to close the out channel ONLY after waiting
// on the waitgroup.
func (s *timingScheduler) Start(miningCtx context.Context, skipCh chan bool) (<-chan Output, *sync.WaitGroup) {
	var doneWg sync.WaitGroup
	outCh := make(chan Output, 1)

	// loop mining work
	doneWg.Add(1)
	go func() {
		s.mineLoop(miningCtx, outCh, skipCh, &doneWg)
		doneWg.Done()
	}()
	s.isStarted = true

	return outCh, &doneWg
}

func (s *timingScheduler) mineLoop(miningCtx context.Context, outCh chan Output, skipCh chan bool, doneWg *sync.WaitGroup) {
	// mineLoop is the main event loop for the timing scheduler.  It waits for
	// a new epoch to start, polls the heaviest head, includes the correct number
	// of null blocks and starts a mining job async.
	//
	// If the previous epoch's mining job is not finished it is canceled via the context
	//
	// The scheduler will skip mining jobs if the skipping flag is set
	workContext, workCancel := context.WithCancel(miningCtx) // nolint:staticcheck
	for {
		s.waitForEpochStart(miningCtx)
		select { // check for interrupt during waiting
		case <-miningCtx.Done():
			return // nolint:govet
		default:
		}
		workCancel() // cancel any late work from last epoch

		// check if we are skipping and don't mine if so
		select {
		case skip := <-skipCh:
			s.skipping = skip
		default:
		}
		if s.skipping { // don't mine if we are skipping
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
		nullBlkCount := s.calcNullBlks(h)
		doneWg.Add(1)
		go func(ctx context.Context) {
			s.worker.Mine(ctx, base, nullBlkCount, outCh)
			doneWg.Done()
		}(workContext)
	}
}

func (s *timingScheduler) waitForEpochStart(miningCtx context.Context) {
	// waitForEpochStart blocks until the next epoch begins
	currEpoch := uint64(s.chainClock.EpochAtTime(s.chainClock.Now()))
	nextEpoch := currEpoch + 1
	nextEpochStart := s.chainClock.StartTimeOfEpoch(nextEpoch)
	nowB4 := s.chainClock.Now()
	waitDur := nextEpochStart.Sub(nowB4)
	newEpochCh := s.chainClock.After(waitDur)
	fmt.Printf("wait for time: %s at %s/%s\n", nextEpochStart, nowB4, s.chainClock.Now())
	select {
	case currTime := <-newEpochCh:
		//	fmt.Printf("starting epoch: %d\n", nextEpoch)
		//		fmt.Printf("exp time: h%d-m%d-s%d-m%d, curr time: h%d-m%d-s%d-m%d\n", nextEpochStart.Hour(), nextEpochStart.Minute(), nextEpochStart.Second(), nextEpochStart.Nanosecond()/1000000, currTime.Hour(), currTime.Minute(), currTime.Second(), currTime.Nanosecond()/1000000)
		_ = currTime
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

// MineOnce mines on a given base until it finds a winner.
func MineOnce(ctx context.Context, w DefaultWorker, ts block.TipSet, c clock.ChainEpochClock) (Output, error) {
	var winner *block.Block
	var err error
	var nullCount uint64
	for winner == nil {
		winner, err = MineOneEpoch(ctx, w, ts, nullCount, c)
		if err != nil {
			return Output{}, err
		}
		nullCount++
	}

	return Output{NewBlock: winner}, nil
}

// MineOneEpoch attempts to mine a block in an epoch and returns the mined block
// or nil if no block could be mined
func MineOneEpoch(ctx context.Context, w DefaultWorker, ts block.TipSet, nullCount uint64, chainClock clock.ChainEpochClock) (*block.Block, error) {
	workCtx, workCancel := context.WithCancel(ctx)
	defer workCancel()
	outCh := make(chan Output, 1)

	// Control the time so that this block is always mined at a time that matches the epoch
	h, err := ts.Height()
	if err != nil {
		return nil, err
	}
	epochStartTime := chainClock.StartTimeOfEpoch(nullCount + h + 1)
	nextEpochStartTime := chainClock.StartTimeOfEpoch(nullCount + h + 2)
	epochTime := nextEpochStartTime.Sub(epochStartTime)
	halfEpochTime := epochTime / time.Duration(2) // mine blocks midway through the epoch

	w.clock = th.NewFakeClock(epochStartTime.Add(halfEpochTime))

	won := w.Mine(workCtx, ts, nullCount, outCh)
	if !won {
		return nil, nil
	}
	out, ok := <-outCh
	if !ok {
		return nil, errors.New("Mining completed without returning block")
	}
	if out.Err != nil {
		return nil, out.Err
	}
	return out.NewBlock, nil
}
