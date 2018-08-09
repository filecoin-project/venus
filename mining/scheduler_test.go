package mining

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUtils(t *testing.T) (*assert.Assertions, *require.Assertions, core.TipSet) {
	assert := assert.New(t)
	require := require.New(t)
	baseBlock := &types.Block{StateRoot: types.SomeCid()}

	ts := core.TipSet{baseBlock.Cid().String(): baseBlock}
	return assert, require, ts
}

// TestMineOnce tests that the MineOnce function results in a mining job being
// scheduled and run by the mining scheduler.
func TestMineOnce(t *testing.T) {
	assert, require, ts := newTestUtils(t)

	// Echoes the sent block to output.
	worker := newTestWorkerWithDeps(makeEchoMine(require))
	scheduler := NewScheduler(worker)
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
	worker := newTestWorkerWithDeps(checkValsMine)
	scheduler := NewScheduler(worker)
	inCh, outCh, _ := scheduler.Start(ctx)
	inCh <- NewInput(context.Background(), ts)
	<-outCh
	cancel()
	return
}

func TestSchedulerPassValue(t *testing.T) {
	assert, require, ts := newTestUtils(t)
	// Test that we can push multiple blocks through. There was an actual bug
	// where multiply queued inputs were not all processed.
	// Another scheduler test.  Now we need to have timing involved...
	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorkerWithDeps(makeEchoMine(require))
	scheduler := NewScheduler(worker)
	inCh, outCh, _ := scheduler.Start(ctx)
	// Note: inputs have to pass whatever check on newly arriving tipsets
	// are in place in Start().
	inCh <- NewInput(context.Background(), ts)
	inCh <- NewInput(context.Background(), ts)
	inCh <- NewInput(context.Background(), ts)
	<-outCh
	<-outCh
	<-outCh
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel()
}

func TestSchedulerCancelInputCtx(t *testing.T) {
	assert, _, ts := newTestUtils(t)
	// Test that canceling the Input.Ctx cancels that input's mining run.
	// scheduler test.  Again state / timing considerations need to be respected now
	// TODO: this test might need to go away with new scheduler patterns, at very least
	// it should be reevaluated after we settle on a solid new pattern for canceling input runs.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	inputCtx, inputCtxCancel := context.WithCancel(context.Background())
	var gotMineCtx context.Context
	fakeMine := func(c context.Context, i Input, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker := newTestWorkerWithDeps(fakeMine)
	scheduler := NewScheduler(worker)
	inCh, outCh, _ := scheduler.Start(miningCtx)
	inCh <- NewInput(inputCtx, ts)
	<-outCh
	inputCtxCancel()
	assert.Error(gotMineCtx.Err()) // Same context as miningRunCtx.
	assert.NoError(miningCtx.Err())
	miningCtxCancel()
}

func TestSchedulerCancelMiningCtx(t *testing.T) {
	assert, _, ts := newTestUtils(t)
	// Test that canceling the mining context stops mining, cancels
	// the inner context, and closes the output channel.
	// scheduler test
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	inputCtx, inputCtxCancel := context.WithCancel(context.Background())
	gotMineCtx := context.Background()
	fakeMine := func(c context.Context, i Input, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker := newTestWorkerWithDeps(fakeMine)
	scheduler := NewScheduler(worker)
	inCh, outCh, doneWg := scheduler.Start(miningCtx)
	inCh <- NewInput(inputCtx, ts)
	<-outCh
	miningCtxCancel()
	doneWg.Wait()
	assert.Equal(ChannelClosed, ReceiveOutCh(outCh))
	assert.Error(gotMineCtx.Err())
	inputCtxCancel()
}
