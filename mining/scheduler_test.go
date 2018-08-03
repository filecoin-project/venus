package mining

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUtils(t *testing.T) (*assert.Assertions, *require.Assertions, consensus.TipSet) {
	assert := assert.New(t)
	require := require.New(t)
	baseBlock := &types.Block{StateRoot: types.SomeCid()}

	ts := consensus.TipSet{baseBlock.Cid().String(): baseBlock}
	return assert, require, ts
}

// TestMineOnce tests that the MineOnce function results in a mining job being
// scheduled and run by the mining scheduler.
func TestMineOnce(t *testing.T) {
	assert, require, ts := newTestUtils(t)

	// Echoes the sent block to output.
	worker := NewTestWorkerWithDeps(MakeEchoMine(require))
	scheduler := NewScheduler(worker, MineDelayTest)
	result := MineOnce(context.Background(), scheduler, ts)
	assert.NoError(result.Err)
	assert.True(ts.ToSlice()[0].StateRoot.Equals(result.NewBlock.StateRoot))
}

func TestSchedulerPassesValue(t *testing.T) {
	assert, _, ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())

	checkValsMine := func(c context.Context, i Input, outCh chan<- Output) {
		assert.NotEqual(ctx, c) // individual run ctx splits off from mining ctx
		assert.Equal(i.TipSet, ts)
		outCh <- Output{}
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, _ := scheduler.Start(ctx)
	inCh <- NewInput(ts)
	<-outCh
	cancel()
}

// Test that we can push multiple blocks through.  This schedules tipsets
// with successively higher block heights (aka epoch).
func TestSchedulerPassesManyValues(t *testing.T) {
	assert, require, ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	var checkTS consensus.TipSet
	// make tipsets with progressively higher heights
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 2}
	ts3 := consensus.RequireNewTipSet(require, blk3)

	checkValsMine := func(c context.Context, i Input, outCh chan<- Output) {
		assert.Equal(i.TipSet, checkTS)
		outCh <- Output{}
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, _ := scheduler.Start(ctx)
	// Note: inputs have to pass whatever check on newly arriving tipsets
	// are in place in Start().  For the (default) timingScheduler tipsets
	// need increasing heights.
	checkTS = ts1
	inCh <- NewInput(ts1)
	<-outCh
	checkTS = ts2
	inCh <- NewInput(ts2)
	<-outCh
	checkTS = ts3
	inCh <- NewInput(ts3)
	<-outCh
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel()
}

// TestSchedulerCollect tests that the scheduler collects tipsets before mining
func TestSchedulerCollect(t *testing.T) {
	assert, require, ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts3 := consensus.RequireNewTipSet(require, blk3)
	checkValsMine := func(c context.Context, i Input, outCh chan<- Output) {
		assert.Equal(i.TipSet, ts3)
		outCh <- Output{}
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, _ := scheduler.Start(ctx)
	inCh <- NewInput(ts1)
	inCh <- NewInput(ts2)
	inCh <- NewInput(ts3) // the scheduler will collect the latest input
	<-outCh
	cancel()
}

// TestCannotInterruptMiner tests that a tipset from the same epoch, i.e. with
// the same height, does not affect the base tipset for mining once the
// mining delay has finished.
func TestCannotInterruptMiner(t *testing.T) {
	assert, require, ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	blk1 := ts1.ToSlice()[0]
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 0}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blockingMine := func(c context.Context, i Input, outCh chan<- Output) {
		time.Sleep(BlockTimeTest)
		assert.Equal(i.TipSet, ts1)
		outCh <- Output{NewBlock: blk1}
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
}

func TestSchedulerCancelMiningCtx(t *testing.T) {
	assert, _, ts := newTestUtils(t)
	// Test that canceling the mining context stops mining, cancels
	// the inner context, and closes the output channel.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	shouldCancelMine := func(c context.Context, i Input, outCh chan<- Output) {
		mineTimer := time.NewTimer(BlockTimeTest)
		select {
		case <-mineTimer.C:
			t.Fatal("should not take whole time")
		case <-c.Done():
		}
	}
	worker := NewTestWorkerWithDeps(shouldCancelMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, doneWg := scheduler.Start(miningCtx)
	inCh <- NewInput(ts)
	miningCtxCancel()
	doneWg.Wait()
	assert.Equal(ChannelClosed, ReceiveOutCh(outCh))
}

func TestSchedulerMultiRoundWithCollect(t *testing.T) {
	assert, require, ts1 := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	var checkTS consensus.TipSet
	// make tipsets with progressively higher heights
	blk2 := &types.Block{StateRoot: types.SomeCid(), Height: 1}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blk3 := &types.Block{StateRoot: types.SomeCid(), Height: 2}
	ts3 := consensus.RequireNewTipSet(require, blk3)

	checkValsMine := func(c context.Context, i Input, outCh chan<- Output) {
		assert.Equal(i.TipSet, checkTS)
		outCh <- Output{}
	}
	worker := NewTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker, MineDelayTest)
	inCh, outCh, doneWg := scheduler.Start(ctx)
	// Note: inputs have to pass whatever check on newly arriving tipsets
	// are in place in Start().  For the (default) timingScheduler tipsets
	// need increasing heights.
	checkTS = ts1
	inCh <- NewInput(ts3)
	inCh <- NewInput(ts2)
	inCh <- NewInput(ts1)
	<-outCh
	checkTS = ts2
	inCh <- NewInput(ts2)
	inCh <- NewInput(ts1)
	inCh <- NewInput(ts3)
	inCh <- NewInput(ts2)
	inCh <- NewInput(ts2)
	<-outCh
	checkTS = ts3
	inCh <- NewInput(ts3)
	<-outCh
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel()
	doneWg.Wait()
	assert.Equal(ChannelClosed, ReceiveOutCh(outCh))
}
