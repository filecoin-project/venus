package mining_test

import (
	"context"
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

func newTestUtils(t *testing.T) block.TipSet {
	baseBlock := &block.Block{StateRoot: types.CidFromString(t, "somecid")}
	ts, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)
	return ts
}

// TestMineOnce tests that the MineOnce function results in a mining job being
// scheduled and run by the mining scheduler.
func TestMineOnce(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)

	// Echoes the sent block to output.
	worker := NewTestWorkerWithDeps(MakeEchoMine(t))
	result, err := MineOnce(context.Background(), worker, MineDelayTest, ts)
	assert.NoError(t, err)
	assert.NoError(t, result.Err)
	assert.True(t, ts.ToSlice()[0].StateRoot.Equals(result.NewBlock.StateRoot))
}

// TestMineOnce10Null calls mine once off of a base tipset with a ticket that
// will win after 10 rounds and verifies that the output has 1 ticket and a
// +10 height.
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
		Clock:         clock.NewSystemClock(),
	})

	result, err := MineOnce(context.Background(), worker, MineDelayTest, baseTs)
	assert.NoError(t, err)
	assert.NoError(t, result.Err)
	block := result.NewBlock
	assert.Equal(t, uint64(10+1), uint64(block.Height))
	assert.NotEqual(t, baseBlock.Ticket, block.Ticket)
}

func TestSchedulerPassesValue(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())

	checkValsMine := func(c context.Context, inTS block.TipSet, nilBlockCount uint64, outCh chan<- Output) bool {
		assert.Equal(t, ctx, c) // individual run ctx splits off from mining ctx
		assert.Equal(t, inTS, ts)
		outCh <- Output{}
		return true
	}
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	head = ts // set head so headFunc returns correctly
	outCh, _ := scheduler.Start(ctx)
	<-outCh
	cancel()
}

func TestSchedulerErrorsOnUnsetHead(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	nothingMine := func(c context.Context, inTS block.TipSet, nbc uint64, outCh chan<- Output) bool {
		outCh <- Output{}
		return false
	}
	nilHeadFunc := func() (block.TipSet, error) {
		return block.UndefTipSet, nil
	}
	worker := NewTestWorkerWithDeps(nothingMine)
	scheduler := NewScheduler(worker, MineDelayTest, nilHeadFunc)
	outCh, doneWg := scheduler.Start(ctx)
	output := <-outCh
	assert.Error(t, output.Err)
	doneWg.Wait()
}

// If head is the same increment the nullblkcount, otherwise make it 0.
func TestSchedulerUpdatesNullBlkCount(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	//	blk2 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}

	checkNullBlocks := uint64(0)
	checkTArrMine := func(c context.Context, inTS block.TipSet, nBC uint64, outCh chan<- Output) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		assert.Equal(t, checkNullBlocks, nBC)
		outCh <- Output{}
		return false
	}
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	worker := NewTestWorkerWithDeps(checkTArrMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	head = ts
	outCh, _ := scheduler.Start(ctx)
	<-outCh
	// setting checkNullBlocks races with the mining delay timer.
	checkNullBlocks = uint64(1)
	<-outCh
	checkNullBlocks = uint64(2)
	<-outCh
	cancel()
}

// Test that we can push multiple blocks through.  This schedules tipsets
// with successively higher block heights (aka epoch).
func TestSchedulerPassesManyValues(t *testing.T) {
	tf.UnitTest(t)

	ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	var checkTS block.TipSet

	// make tipsets with progressively higher heights
	blk2 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 2}
	ts3 := th.RequireNewTipSet(t, blk3)
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}

	checkValsMine := func(c context.Context, ts block.TipSet, nbc uint64, outCh chan<- Output) bool {
		assert.Equal(t, ts, checkTS)
		outCh <- Output{}
		return false
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	checkTS = ts1
	head = ts1
	outCh, _ := scheduler.Start(ctx)
	<-outCh
	// This is testing a race (that checkTS and head are both set before
	// the headFunc is called, but the TestMine delay should be long enough
	// that it should work.  TODO: eliminate races.
	checkTS = ts2
	head = ts2
	<-outCh
	checkTS = ts3 // Same race as ^^
	head = ts3
	<-outCh
	cancel()
}

// TestSchedulerCollect tests that the scheduler collects tipsets before mining
func TestSchedulerCollect(t *testing.T) {
	tf.UnitTest(t)
	ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	blk2 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}
	ts3 := th.RequireNewTipSet(t, blk3)
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	checkValsMine := func(c context.Context, inTS block.TipSet, nbc uint64, outCh chan<- Output) bool {
		assert.Equal(t, inTS, ts3)
		outCh <- Output{}
		return false
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	head = ts1
	outCh, _ := scheduler.Start(ctx)
	// again this is racing on the assumption that mining delay is long
	// enough for all these variables to be set before the sleep finishes.
	head = ts2
	head = ts3 // the scheduler should collect the latest input
	<-outCh
	cancel()
}

func TestSchedulerCancelMiningCtx(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	// Test that canceling the mining context stops mining, cancels
	// the inner context, and closes the output channel.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	shouldCancelMine := func(c context.Context, inTS block.TipSet, nbc uint64, outCh chan<- Output) bool {
		mineTimer := time.NewTimer(th.BlockTimeTest)
		select {
		case <-mineTimer.C:
			t.Fatal("should not take whole time")
		case <-c.Done():
		}
		return false
	}
	worker := NewTestWorkerWithDeps(shouldCancelMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	head = ts
	outCh, doneWg := scheduler.Start(miningCtx)
	miningCtxCancel()
	doneWg.Wait()
	assert.Equal(t, ChannelClosed, ReceiveOutCh(outCh))
}

func TestSchedulerMultiRoundWithCollect(t *testing.T) {
	tf.UnitTest(t)
	ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	var checkTS block.TipSet
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	// make tipsets with progressively higher heights
	blk2 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 2}
	ts3 := th.RequireNewTipSet(t, blk3)

	checkValsMine := func(c context.Context, inTS block.TipSet, nbc uint64, outCh chan<- Output) bool {
		assert.Equal(t, inTS, checkTS)
		outCh <- Output{}
		return false
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	checkTS = ts1
	head = ts1
	outCh, doneWg := scheduler.Start(ctx)

	<-outCh
	head = ts2 // again we're racing :(
	checkTS = ts2

	<-outCh
	checkTS = ts3
	head = ts3

	cancel()
	doneWg.Wait()

	// drain the channel
	for range outCh {
	}

	assert.Equal(t, ChannelClosed, ReceiveOutCh(outCh))
}
