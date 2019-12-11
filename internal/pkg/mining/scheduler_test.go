package mining_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// TestMineOnce tests that the MineOnce function results in a mining job being
// scheduled and run by the mining scheduler.
func TestMineOnce(t *testing.T) {
	tf.UnitTest(t)

	ts := testHead(t)

	// Echoes the sent block to output.
	worker := NewTestWorker(t, MakeEchoMine(t))
	chainClock := clock.NewChainClock(uint64(time.Now().Unix()), 100*time.Millisecond)
	result, err := MineOnce(context.Background(), worker, ts, chainClock)
	assert.NoError(t, err)
	assert.NoError(t, result.Err)
	assert.True(t, ts.ToSlice()[0].StateRoot.Equals(result.NewBlock.StateRoot))
}

// TestMineOnce10Null calls mine once off of a base tipset with a ticket that
// will win after 10 rounds and verifies that the output has 1 ticket and a
// +10 height.
//
// Note there is a race here.  This test can fail if the mining worker fails to
// finish its work before the epoch is over.  In practice this does not happen.
func TestMineOnce10Null(t *testing.T) {
	tf.IntegrationTest(t)

	mockSigner, kis := types.NewMockSignersAndKeyInfo(5)
	ki := &(kis[0])
	addr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[addr] = addr
	baseTicket := consensus.SeedFirstWinnerInNRounds(t, 10, ki, 100, 10000)
	baseBlock := &block.Block{
		StateRoot: types.CidFromString(t, "somecid"),
		Height:    0,
		Ticket:    baseTicket,
	}
	baseTs, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)

	st, pool, _, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return []block.TipSet{baseTs}, nil
	}
	messages := chain.NewMessageStore(bs)

	api := th.NewFakeWorkerPorcelainAPI(addr, 10, minerToWorker)
	genTime := time.Now()
	chainClock := clock.NewChainClock(uint64(genTime.Unix()), 300*time.Millisecond)

	worker := NewDefaultWorker(WorkerParameters{
		API: api,

		MinerAddr:      addr,
		MinerOwnerAddr: addr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.ElectionMachine{},
		TicketGen:      &consensus.TicketMachine{},

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         chainClock,
	})

	result, err := MineOnce(context.Background(), worker, baseTs, chainClock)
	assert.NoError(t, err)
	assert.NoError(t, result.Err)
	block := result.NewBlock
	assert.Equal(t, uint64(10+1), uint64(block.Height))
	assert.NotEqual(t, baseBlock.Ticket, block.Ticket)
}

// This test makes use of the MineOneEpoch call to
// exercise the mining code without races or long blocking
func TestMineOneEpoch10Null(t *testing.T) {
	tf.IntegrationTest(t)

	mockSigner, kis := types.NewMockSignersAndKeyInfo(5)
	ki := &(kis[0])
	addr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[addr] = addr
	baseTicket := consensus.SeedFirstWinnerInNRounds(t, 10, ki, 100, 10000)
	baseBlock := &block.Block{
		StateRoot: types.CidFromString(t, "somecid"),
		Height:    0,
		Ticket:    baseTicket,
	}
	baseTs, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)

	st, pool, _, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return []block.TipSet{baseTs}, nil
	}
	messages := chain.NewMessageStore(bs)

	api := th.NewFakeWorkerPorcelainAPI(addr, 10, minerToWorker)
	genTime := time.Now()
	chainClock := clock.NewChainClock(uint64(genTime.Unix()), 15*time.Second)

	worker := NewDefaultWorker(WorkerParameters{
		API: api,

		MinerAddr:      addr,
		MinerOwnerAddr: addr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.ElectionMachine{},
		TicketGen:      &consensus.TicketMachine{},

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         chainClock,
	})

	for i := 0; i < 10; i++ {
		// with null count < 10 we see no errors and get no wins
		blk, err := MineOneEpoch(context.Background(), *worker, baseTs, uint64(i), chainClock)
		assert.NoError(t, err)
		assert.Nil(t, blk)
	}
	blk, err := MineOneEpoch(context.Background(), *worker, baseTs, 10, chainClock)
	assert.NoError(t, err)
	assert.NotNil(t, blk)
	assert.Equal(t, uint64(10+1), uint64(blk.Height))
	assert.Equal(t, chainClock.EpochAtTime(time.Unix(int64(blk.Timestamp), int64(blk.Timestamp%1000000000))), int64(blk.Height))
}

// Mining loop unit tests

func TestWorkerCalled(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)

	called := false
	var wg sync.WaitGroup
	wg.Add(1)
	w := NewTestWorker(t, func(_ context.Context, workHead block.TipSet, _ uint64, _ chan<- Output) bool {
		called = true
		assert.True(t, workHead.Equals(ts))
		wg.Done()
		return true
	})

	fakeClock, chainClock, blockTime := testClock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx, nil)
	fmt.Printf("current time %s\n", chainClock.Now())
	fakeClock.BlockUntil(1)
	fmt.Printf("got a sleeper\n")
	fakeClock.Advance(blockTime)
	fmt.Printf("current time %s\n", chainClock.Now())

	wg.Wait()
	assert.True(t, called)
}

func TestCorrectNullBlocksGivenEpoch(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)
	h, err := ts.Height()
	require.NoError(t, err)

	fakeClock, chainClock, blockTime := testClock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Move forward 20 epochs
	for i := 0; i < 19; i++ {
		fakeClock.Advance(blockTime)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	w := NewTestWorker(t, func(_ context.Context, _ block.TipSet, nullCount uint64, _ chan<- Output) bool {
		assert.Equal(t, h+19, nullCount)
		wg.Done()
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx, nil)
	fakeClock.BlockUntil(1)
	// Move forward 1 epoch for a total of 21
	fakeClock.Advance(blockTime)

	wg.Wait()
}

func TestWaitsForEpochStart(t *testing.T) {
	// If the scheduler starts partway through an epoch it will wait to mine
	// until there is a new epoch boundary
	tf.UnitTest(t)
	ts := testHead(t)

	fakeClock, chainClock, blockTime := testClock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	genTime := chainClock.Now()

	var wg sync.WaitGroup
	wg.Add(1)
	w := NewTestWorker(t, func(_ context.Context, workHead block.TipSet, _ uint64, _ chan<- Output) bool {
		// This doesn't get called until the clock has advanced one blocktime
		assert.Equal(t, genTime.Add(blockTime), chainClock.Now())
		wg.Done()
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx, nil)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime / time.Duration(2)) // advance half a blocktime
	time.Sleep(300 * time.Millisecond)              // Need to race.  Ok to delete this test if we prefer not to.
	fakeClock.Advance(blockTime / time.Duration(2))
	wg.Wait()
}

func TestCancelsLateWork(t *testing.T) {
	// Test will hang if work is not cancelled
	tf.UnitTest(t)
	ts := testHead(t)

	fakeClock, chainClock, blockTime := testClock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	var count int
	w := NewTestWorker(t, func(workCtx context.Context, _ block.TipSet, _ uint64, _ chan<- Output) bool {
		if count != 0 { // only first job blocks
			return true
		}
		count++
		select {
		case <-workCtx.Done():
			wg.Done()
			return true
		}
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx, nil)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime) // schedule first work item
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime) // enter next epoch, should cancel first work item

	wg.Wait()
}

func TestShutdownWaitgroup(t *testing.T) {
	// waitgroup waits for all mining jobs to shut down properly
	genTime := time.Now()
	chainClock := clock.NewChainClock(uint64(genTime.Unix()), 100*time.Millisecond)
	ts := testHead(t)
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	jobs := make(map[uint64]bool)
	w := NewTestWorker(t, func(workContext context.Context, _ block.TipSet, null uint64, _ chan<- Output) bool {
		mu.Lock()
		jobs[null] = false
		mu.Unlock()
		select {
		case <-workContext.Done():
			mu.Lock()
			jobs[null] = true
			mu.Unlock()
			return true
		}
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	_, wg := scheduler.Start(ctx, nil)
	time.Sleep(600 * time.Millisecond) // run through some epochs
	cancel()
	wg.Wait()

	// After passing barrier all jobs should be finished
	mu.Lock()
	defer mu.Unlock()
	for _, waitedForFin := range jobs {
		assert.True(t, waitedForFin)
	}
}

func TestSkips(t *testing.T) {
	tf.UnitTest(t)
	ts := testHead(t)

	fakeClock, chainClock, blockTime := testClock(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	w := NewTestWorker(t, func(_ context.Context, _ block.TipSet, nullCount uint64, _ chan<- Output) bool {
		// This should never be reached as the first epoch should skip mining
		if nullCount == 0 {
			t.Fail()
			return true
		} else {
			wg.Done()
		}
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	skipCh := make(chan bool)
	go func() { skipCh <- true }()
	scheduler.Start(ctx, skipCh)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime)
	go func() { skipCh <- false }()
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime)
	wg.Wait()
}

// Helper functions

func testHead(t *testing.T) block.TipSet {
	baseBlock := &block.Block{StateRoot: types.CidFromString(t, "somecid")}
	ts, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)
	return ts
}

func testClock(t *testing.T) (th.FakeClock, clock.ChainEpochClock, time.Duration) {
	// return a fake clock for running tests a ChainEpochClock for
	// using in the scheduler, and the testing blocktime
	gt := time.Unix(1234567890, 1234567890%1000000000)
	fmt.Printf("test clock time: h%d-m%d-s%d-m%d\n", gt.Hour(), gt.Minute(), gt.Second(), gt.Nanosecond()/1000000)
	fc := th.NewFakeClock(gt)
	defaultBlockTimeTest := 1 * time.Second
	chainClock := clock.NewChainClockFromClock(uint64(gt.Unix()), defaultBlockTimeTest, fc)

	return fc, chainClock, defaultBlockTimeTest
}

func headFunc(ts block.TipSet) func() (block.TipSet, error) {
	return func() (block.TipSet, error) {
		return ts, nil
	}
}
