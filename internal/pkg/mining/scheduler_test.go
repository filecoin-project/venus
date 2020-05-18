package mining_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

const epochDuration = builtin.EpochDurationSeconds
const propDelay = 6 * time.Second

// Mining loop unit tests

func TestWorkerCalled(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)

	called := make(chan struct{}, 1)
	w := NewTestWorker(t, func(_ context.Context, workHead block.TipSet, _ uint64, out chan<- Output) bool {
		assert.True(t, workHead.Equals(ts))
		out <- NewOutputEmpty()
		called <- struct{}{}
		return true
	})

	fakeClock, chainClock := clock.NewFakeChain(1234567890, epochDuration, propDelay, 1234567890)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(epochDuration)
	fakeClock.Advance(propDelay)

	<-called
}

func TestCorrectNullBlocksGivenEpoch(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)
	h, err := ts.Height()
	require.NoError(t, err)

	fakeClock, chainClock := clock.NewFakeChain(1234567890, epochDuration, propDelay, 1234567890)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Move forward 20 epochs
	for i := 0; i < 19; i++ {
		fakeClock.Advance(epochDuration)
	}

	called := make(chan struct{}, 20)
	w := NewTestWorker(t, func(_ context.Context, _ block.TipSet, nullCount uint64, out chan<- Output) bool {
		assert.Equal(t, uint64(h+20), nullCount)
		out <- NewOutputEmpty()
		called <- struct{}{}
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	// Move forward 1 epoch for a total of 21
	fakeClock.Advance(epochDuration)
	fakeClock.Advance(propDelay)

	<-called
}

func TestWaitsForEpochStart(t *testing.T) {
	// If the scheduler starts partway through an epoch it will wait to mine
	// until there is a new epoch boundary
	tf.UnitTest(t)
	ts := testHead(t)

	fakeClock, chainClock := clock.NewFakeChain(1234567890, epochDuration, propDelay, 1234567890)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	genTime := chainClock.Now()

	var wg sync.WaitGroup
	wg.Add(1)
	waitGroupDoneCh := make(chan struct{})
	go func() {
		wg.Wait()
		waitGroupDoneCh <- struct{}{}
	}()

	called := make(chan struct{}, 1)
	expectMiningCall := false
	w := NewTestWorker(t, func(_ context.Context, workHead block.TipSet, _ uint64, _ chan<- Output) bool {
		if !expectMiningCall {
			t.Fatal("mining worker called too early")
			return true
		}
		// This doesn't get called until the clock has advanced to prop delay past epoch
		assert.Equal(t, genTime.Add(epochDuration).Add(propDelay), chainClock.Now())
		called <- struct{}{}
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)

	fakeClock.BlockUntil(1)
	expectMiningCall = false
	fakeClock.Advance(epochDuration) // advance to epoch start
	fakeClock.Advance(propDelay / 2) // advance halfway into prop delay

	// advance past propagation delay in next block and expect worker to be called
	fakeClock.BlockUntil(1)
	expectMiningCall = true
	fakeClock.Advance(propDelay / 2)
	<-called
}

func TestSkips(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)

	fakeClock, chainClock := clock.NewFakeChain(1234567890, epochDuration, propDelay, 1234567890)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	w := NewTestWorker(t, func(_ context.Context, _ block.TipSet, nullCount uint64, _ chan<- Output) bool {
		// This should never be reached as the first epoch should skip mining
		if nullCount == 0 {
			t.Fail()
			return true
		}
		wg.Done()

		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Pause()
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(epochDuration + propDelay)
	fakeClock.BlockUntil(1)
	scheduler.Continue()
	fakeClock.Advance(epochDuration)
	wg.Wait()
}

// Helper functions

func testHead(t *testing.T) block.TipSet {
	baseBlock := &block.Block{StateRoot: e.NewCid(types.CidFromString(t, "somecid"))}
	ts, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)
	return ts
}

func headFunc(ts block.TipSet) func() (block.TipSet, error) {
	return func() (block.TipSet, error) {
		return ts, nil
	}
}
