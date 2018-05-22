package mining

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMineOnce(t *testing.T) {
	assert := assert.New(t)
	mockBg := &MockBlockGenerator{}
	baseBlock := &types.Block{StateRoot: types.SomeCid()}
	tipSet := core.TipSet{baseBlock.Cid().String(): baseBlock}
	tipSets := []core.TipSet{tipSet}
	rewardAddr := types.NewAddressForTestGetter()()

	var mineCtx context.Context
	// Echoes the sent block to output.
	echoMine := func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		mineCtx = c
		b := core.BaseBlockFromTipSets(i.TipSets)
		outCh <- Output{NewBlock: b}
	}
	worker := NewWorkerWithDeps(mockBg, echoMine, func() {})
	result := MineOnce(context.Background(), worker, tipSets, rewardAddr)
	assert.NoError(result.Err)
	assert.True(baseBlock.StateRoot.Equals(result.NewBlock.StateRoot))
	assert.Error(mineCtx.Err())
}

func TestWorker_Start(t *testing.T) {
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	baseBlock := &types.Block{StateRoot: newCid()}
	tipSet := core.TipSet{baseBlock.Cid().String(): baseBlock}
	tipSets := []core.TipSet{tipSet}
	mockBg := &MockBlockGenerator{}
	rewardAddr := types.NewAddressForTestGetter()()

	// Test that values are passed faithfully.
	ctx, cancel := context.WithCancel(context.Background())
	doSomeWorkCalled := false
	doSomeWork := func() { doSomeWorkCalled = true }
	mineCalled := false
	fakeMine := func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		mineCalled = true
		assert.NotEqual(ctx, c)
		b := core.BaseBlockFromTipSets(i.TipSets)
		assert.True(baseBlock.StateRoot.Equals(b.StateRoot))
		assert.Equal(mockBg, bg)
		doSomeWork()
		outCh <- Output{}
	}
	worker := NewWorkerWithDeps(mockBg, fakeMine, doSomeWork)
	inCh, outCh, _ := worker.Start(ctx)
	inCh <- NewInput(context.Background(), tipSets, rewardAddr)
	<-outCh
	assert.True(mineCalled)
	assert.True(doSomeWorkCalled)
	cancel()

	// Test that we can push multiple blocks through. There was an actual bug
	// where multiply queued inputs were not all processed.
	ctx, cancel = context.WithCancel(context.Background())
	fakeMine = func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {})
	inCh, outCh, _ = worker.Start(ctx)
	// Note: inputs have to pass whatever check on newly arriving tipsets
	// are in place in Start().
	inCh <- NewInput(context.Background(), tipSets, rewardAddr)
	inCh <- NewInput(context.Background(), tipSets, rewardAddr)
	inCh <- NewInput(context.Background(), tipSets, rewardAddr)
	<-outCh
	<-outCh
	<-outCh
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel() // Makes vet happy.

	// Test that it ignores blocks with lower score.
	ctx, cancel = context.WithCancel(context.Background())
	b1 := &types.Block{Height: 1}
	b1Ts := []core.TipSet{{b1.Cid().String(): b1}}
	bWorseScore := &types.Block{Height: 0}
	bWorseScoreTs := []core.TipSet{{bWorseScore.Cid().String(): bWorseScore}}
	fakeMine = func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		gotBlock := core.BaseBlockFromTipSets(i.TipSets)
		assert.True(b1.Cid().Equals(gotBlock.Cid()))
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {})
	inCh, outCh, _ = worker.Start(ctx)
	inCh <- NewInput(context.Background(), b1Ts, rewardAddr)
	<-outCh
	inCh <- NewInput(context.Background(), bWorseScoreTs, rewardAddr)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel() // Makes vet happy.

	// Test that canceling the Input.Ctx cancels that input's mining run.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	inputCtx, inputCtxCancel := context.WithCancel(context.Background())
	var gotMineCtx context.Context
	fakeMine = func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {})
	inCh, outCh, _ = worker.Start(miningCtx)
	inCh <- NewInput(inputCtx, tipSets, rewardAddr)
	<-outCh
	inputCtxCancel()
	assert.Error(gotMineCtx.Err()) // Same context as miningRunCtx.
	assert.NoError(miningCtx.Err())
	miningCtxCancel() // Make vet happy.

	// Test that canceling the mining context stops mining, cancels
	// the inner context, and closes the output channel.
	miningCtx, miningCtxCancel = context.WithCancel(context.Background())
	inputCtx, inputCtxCancel = context.WithCancel(context.Background())
	gotMineCtx = context.Background()
	fakeMine = func(c context.Context, i Input, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {})
	inCh, outCh, doneWg := worker.Start(miningCtx)
	inCh <- NewInput(inputCtx, tipSets, rewardAddr)
	<-outCh
	miningCtxCancel()
	doneWg.Wait()
	assert.Equal(ChannelClosed, ReceiveOutCh(outCh))
	assert.Error(gotMineCtx.Err())
	inputCtxCancel() // Make vet happy.
}

func Test_mine(t *testing.T) {
	assert := assert.New(t)
	baseBlock := &types.Block{Height: 2}
	tipSet := core.TipSet{baseBlock.Cid().String(): baseBlock}
	tipSets := []core.TipSet{tipSet}
	addr := types.NewAddressForTestGetter()()
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success.
	mockBg := &MockBlockGenerator{}
	outCh := make(chan Output)
	mockBg.On("Generate", mock.Anything, baseBlock, addr).Return(next, nil)
	doSomeWorkCalled := false
	input := NewInput(ctx, tipSets, addr)
	go Mine(ctx, input, mockBg, func() { doSomeWorkCalled = true }, outCh)
	r := <-outCh
	assert.NoError(r.Err)
	assert.True(doSomeWorkCalled)
	assert.True(r.NewBlock.Cid().Equals(next.Cid()))
	mockBg.AssertExpectations(t)

	// Block generation fails.
	mockBg = &MockBlockGenerator{}
	outCh = make(chan Output)
	mockBg.On("Generate", mock.Anything, baseBlock, addr).Return(nil, errors.New("boom"))
	doSomeWorkCalled = false
	input = NewInput(ctx, tipSets, addr)
	go Mine(ctx, input, mockBg, func() { doSomeWorkCalled = true }, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.False(doSomeWorkCalled)
	mockBg.AssertExpectations(t)
}
