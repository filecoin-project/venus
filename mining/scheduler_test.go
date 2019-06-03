package mining

import (
	"context"
	"testing"
	"time"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUtils(t *testing.T) types.TipSet {
	baseBlock := &types.Block{StateRoot: types.SomeCid()}
	ts, err := types.NewTipSet(baseBlock)
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

func TestSchedulerPassesValue(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())

	checkValsMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
		assert.Equal(t, ctx, c) // individual run ctx splits off from mining ctx
		assert.Equal(t, inTS, ts)
		outCh <- Output{}
		return true
	}
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
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

	nothingMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
		outCh <- Output{}
		return false
	}
	nilHeadFunc := func() (types.TipSet, error) {
		return types.UndefTipSet, nil
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
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)

	checkNullBlocks := 0
	checkNullBlockMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		assert.Equal(t, checkNullBlocks, nBC)
		outCh <- Output{}
		return false
	}
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
		return head, nil
	}
	worker := NewTestWorkerWithDeps(checkNullBlockMine)
	scheduler := NewScheduler(worker, MineDelayTest, headFunc)
	head = ts
	outCh, _ := scheduler.Start(ctx)
	<-outCh
	// setting checkNullBlocks races with the mining delay timer.
	checkNullBlocks = 1
	<-outCh
	checkNullBlocks = 2
	<-outCh
	head = ts2
	checkNullBlocks = 0
	<-outCh
	cancel()
}

// Test that we can push multiple blocks through.  This schedules tipsets
// with successively higher block heights (aka epoch).
func TestSchedulerPassesManyValues(t *testing.T) {
	tf.UnitTest(t)

	ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	var checkTS types.TipSet
	// make tipsets with progressively higher heights
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 2}
	ts3 := th.RequireNewTipSet(t, blk3)
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
		return head, nil
	}

	checkValsMine := func(c context.Context, ts types.TipSet, nBC int, outCh chan<- Output) bool {
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
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts3 := th.RequireNewTipSet(t, blk3)
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
		return head, nil
	}
	checkValsMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
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

// This test is no longer meaningful without mocking ticket generation winning.
// We need some way to make sure that the block being mined is still the block
// received during collect.  TODO: isWinningTicket faking and reimplementing
// in this new paradigm
/*
func TestCannotInterruptMiner(t *testing.T) {
	assert, require, ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	blk1 := ts1.ToSlice()[0]
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 0}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blockingMine := func(c context.Context, ts types.TipSet, nBC int, outCh chan<- Output) {
		time.Sleep(th.BlockTimeTest)
		assert.Equal(ts, ts1)
		outCh <- Output{NewBlock: blk1}
	}
	var head types.TipSet
	headFunc := func() types.TipSet {
		return head
	}
	worker := NewTestWorkerWithDeps(blockingMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, _ := scheduler.Start(ctx)
	inCh <- NewInput(ts1)
	// Wait until well after the mining delay, and send a new input.
	time.Sleep(4 * MineDelayTest)
	inCh <- NewInput(ts2)
	out := <-outCh
	assert.Equal(out.NewBlock, blk1)
	cancel()
}*/

func TestSchedulerCancelMiningCtx(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	// Test that canceling the mining context stops mining, cancels
	// the inner context, and closes the output channel.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
		return head, nil
	}
	shouldCancelMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
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
	var checkTS types.TipSet
	var head types.TipSet
	headFunc := func() (types.TipSet, error) {
		return head, nil
	}
	// make tipsets with progressively higher heights
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 2}
	ts3 := th.RequireNewTipSet(t, blk3)

	checkValsMine := func(c context.Context, inTS types.TipSet, nBC int, outCh chan<- Output) bool {
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
