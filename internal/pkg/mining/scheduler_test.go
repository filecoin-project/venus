package mining_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// TestMineOnce10Null calls mine once off of a base tipset with a ticket that
// will win after 10 rounds and verifies that the output has 1 ticket and a
// +10 height.
func TestMineOnce10Null(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Dragons: fake proofs")

	mockSigner, kis := types.NewMockSignersAndKeyInfo(5)
	ki := &(kis[0])
	addr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[addr] = addr
	totalPower := uint64(10000)
	numSectors := uint64(1)
	sectorSize := uint64(100)
	rnd := consensus.SeedFirstWinnerInNRounds(t, 10, addr, ki, totalPower, numSectors, sectorSize)
	baseBlock := &block.Block{
		StateRoot: e.NewCid(types.CidFromString(t, "somecid")),
		Height:    0,
	}
	baseTs, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)

	st, pool, _, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	messages := chain.NewMessageStore(bs)

	// Dragons: need to give the miners in the fake state actual power for these tests to work.
	api := th.NewFakeWorkerPorcelainAPI(rnd, 100, minerToWorker)
	genTime := time.Now()
	fc := th.NewFakeClock(genTime)
	chainClock := clock.NewChainClockFromClock(uint64(genTime.Unix()), 15*time.Second, fc)

	worker := NewDefaultWorker(WorkerParameters{
		API: api,

		MinerAddr:      addr,
		MinerOwnerAddr: addr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       consensus.NewElectionMachine(rnd),
		TicketGen:      consensus.NewTicketMachine(rnd),

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         chainClock,
		Poster:        &proofs.ElectionPoster{},
	})

	result, err := MineOnce(context.Background(), *worker, baseTs, chainClock)
	assert.NoError(t, err)
	assert.NoError(t, result.Err)
	block := result.NewBlock
	assert.Equal(t, uint64(10+1), block.Height)
	assert.NotEqual(t, baseBlock.Ticket, block.Ticket)
}

// This test makes use of the MineOneEpoch call to
// exercise the mining code without races or long blocking
func TestMineOneEpoch10Null(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Dragons: fake proofs")

	mockSigner, kis := types.NewMockSignersAndKeyInfo(5)
	ki := &(kis[0])
	addr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[addr] = addr
	totalPower := uint64(10000)
	numSectors := uint64(1)
	sectorSize := uint64(100)
	rnd := consensus.SeedFirstWinnerInNRounds(t, 10, addr, ki, totalPower, numSectors, sectorSize)
	baseBlock := &block.Block{
		StateRoot: e.NewCid(types.CidFromString(t, "somecid")),
		Height:    0,
	}
	baseTs, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)

	st, pool, _, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	messages := chain.NewMessageStore(bs)

	api := th.NewFakeWorkerPorcelainAPI(rnd, 100, minerToWorker)
	genTime := time.Now()
	fc := th.NewFakeClock(genTime)
	chainClock := clock.NewChainClockFromClock(uint64(genTime.Unix()), 15*time.Second, fc)

	worker := NewDefaultWorker(WorkerParameters{
		API: api,

		MinerAddr:      addr,
		MinerOwnerAddr: addr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       consensus.NewElectionMachine(rnd),
		TicketGen:      consensus.NewTicketMachine(rnd),

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         chainClock,
		Poster:        &proofs.ElectionPoster{},
	})

	for i := 0; i < 10; i++ {
		// with null count < 10 we see no errors and get no wins
		blk, err := MineOneEpoch(context.Background(), *worker, baseTs, uint64(i), chainClock)
		assert.NoError(t, err)
		assert.Nil(t, blk)
	}
	blk, err := MineOneEpoch(context.Background(), *worker, baseTs, 10, chainClock)
	assert.NoError(t, err)
	require.NotNil(t, blk)
	assert.Equal(t, uint64(10+1), blk.Height)
	assert.Equal(t, chainClock.EpochAtTime(time.Unix(int64(blk.Timestamp), 0)), blk.Height)
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
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime)

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
		assert.Equal(t, uint64(h+19), nullCount)
		wg.Done()
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)
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
	waitGroupDoneCh := make(chan struct{})
	go func() {
		wg.Wait()
		waitGroupDoneCh <- struct{}{}
	}()
	w := NewTestWorker(t, func(_ context.Context, workHead block.TipSet, _ uint64, _ chan<- Output) bool {
		// This doesn't get called until the clock has advanced one blocktime
		assert.Equal(t, genTime.Add(blockTime), chainClock.Now())
		wg.Done()
		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime / time.Duration(2)) // advance half a blocktime
	// Test relies on race, that this sleep would be enough time for the mining job
	// to hit wg.Done() if it was triggered partway through the epoch
	time.Sleep(300 * time.Millisecond)
	// assert that waitgroup is not done and hence mining job is not yet run.
	select {
	case <-waitGroupDoneCh:
		t.Fatal()
	default:
	}

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
	w := NewTestWorker(t, func(workCtx context.Context, _ block.TipSet, nullCount uint64, _ chan<- Output) bool {
		if nullCount != 0 { // only first job blocks
			return true
		}
		select {
		case <-workCtx.Done():
			wg.Done()
			return true
		}
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime) // schedule first work item
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime) // enter next epoch, should cancel first work item

	wg.Wait()
}

func TestShutdownWaitgroup(t *testing.T) {
	// waitgroup waits for all mining jobs to shut down properly
	tf.IntegrationTest(t)
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
	_, wg := scheduler.Start(ctx)
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
		}
		wg.Done()

		return true
	})

	scheduler := NewScheduler(w, headFunc(ts), chainClock)
	scheduler.Pause()
	scheduler.Start(ctx)
	fakeClock.BlockUntil(1)
	fakeClock.Advance(blockTime)
	fakeClock.BlockUntil(1)
	scheduler.Continue()
	fakeClock.Advance(blockTime)
	wg.Wait()
}

// Helper functions

func testHead(t *testing.T) block.TipSet {
	baseBlock := &block.Block{StateRoot: e.NewCid(types.CidFromString(t, "somecid"))}
	ts, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)
	return ts
}

func testClock(t *testing.T) (th.FakeClock, clock.ChainEpochClock, time.Duration) {
	// return a fake clock for running tests a ChainEpochClock for
	// using in the scheduler, and the testing blocktime
	gt := time.Unix(1234567890, 1234567890%1000000000)
	fc := th.NewFakeClock(gt)
	DefaultEpochDurationTest := 1 * time.Second
	chainClock := clock.NewChainClockFromClock(uint64(gt.Unix()), DefaultEpochDurationTest, fc)

	return fc, chainClock, DefaultEpochDurationTest
}

func headFunc(ts block.TipSet) func() (block.TipSet, error) {
	return func() (block.TipSet, error) {
		return ts, nil
	}
}
