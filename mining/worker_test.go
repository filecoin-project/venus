package mining

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/swap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
)

func TestMineOnce(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	mockBg := &MockBlockGenerator{}
	baseBlock := &types.Block{StateRoot: types.SomeCid()}
	tipSet := core.TipSet{baseBlock.Cid().String(): baseBlock}
	newAddr := types.NewAddressForTestGetter()
	rewardAddr := newAddr()
	miningAddr := newAddr()

	var mineCtx context.Context
	// Echoes the sent block to output.
	echoMine := func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		mineCtx = c
		require.Equal(1, len(i.TipSet))
		b := i.TipSet.ToSlice()[0]
		outCh <- Output{NewBlock: b}
	}
	worker := NewWorkerWithDeps(mockBg, echoMine, func() {}, nullBlockImmediately)
	result := MineOnce(context.Background(), worker, tipSet, rewardAddr, miningAddr)
	assert.NoError(result.Err)
	assert.True(baseBlock.StateRoot.Equals(result.NewBlock.StateRoot))
	assert.Error(mineCtx.Err())
}

func TestWorker_Start(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newCid := types.NewCidForTestGetter()
	baseBlock := &types.Block{StateRoot: newCid()}
	tipSet := core.TipSet{baseBlock.Cid().String(): baseBlock}
	mockBg := &MockBlockGenerator{}
	newAddr := types.NewAddressForTestGetter()
	rewardAddr := newAddr()
	miningAddr := newAddr()

	// Test that values are passed faithfully.
	ctx, cancel := context.WithCancel(context.Background())
	doSomeWorkCalled := false
	doSomeWork := func() { doSomeWorkCalled = true }
	mineCalled := false
	fakeMine := func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		mineCalled = true
		assert.NotEqual(ctx, c)
		require.Equal(1, len(i.TipSet))
		b := i.TipSet.ToSlice()[0]
		assert.True(baseBlock.StateRoot.Equals(b.StateRoot))
		assert.Equal(mockBg, bg)
		doSomeWork()
		outCh <- Output{}
	}
	worker := NewWorkerWithDeps(mockBg, fakeMine, doSomeWork, nullBlockImmediately)
	inCh, outCh, _ := worker.Start(ctx)
	inCh <- NewInput(context.Background(), tipSet, rewardAddr, miningAddr)
	<-outCh
	assert.True(mineCalled)
	assert.True(doSomeWorkCalled)
	cancel()

	// Test that multi-block tipsets are passed faithfully
	mineCalled = false
	ctx, cancel = context.WithCancel(context.Background())
	tipSet = core.RequireNewTipSet(require, []*types.Block{{StateRoot: newCid()}, {StateRoot: newCid()}}...)
	fakeMine = func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		mineCalled = true
		require.Equal(2, len(i.TipSet))
		tipSetMined := i.TipSet
		assert.Equal(tipSet, tipSetMined)
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {}, nullBlockImmediately)
	inCh, outCh, _ = worker.Start(ctx)
	inCh <- NewInput(context.Background(), tipSet, rewardAddr, miningAddr)
	<-outCh
	assert.True(mineCalled)
	cancel()

	// Test that we can push multiple blocks through. There was an actual bug
	// where multiply queued inputs were not all processed.
	ctx, cancel = context.WithCancel(context.Background())
	fakeMine = func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {}, nullBlockImmediately)
	inCh, outCh, _ = worker.Start(ctx)
	// Note: inputs have to pass whatever check on newly arriving tipsets
	// are in place in Start().
	inCh <- NewInput(context.Background(), tipSet, rewardAddr, miningAddr)
	inCh <- NewInput(context.Background(), tipSet, rewardAddr, miningAddr)
	inCh <- NewInput(context.Background(), tipSet, rewardAddr, miningAddr)
	<-outCh
	<-outCh
	<-outCh
	assert.Equal(ChannelEmpty, ReceiveOutCh(outCh))
	cancel() // Makes vet happy.

	// Test that canceling the Input.Ctx cancels that input's mining run.
	miningCtx, miningCtxCancel := context.WithCancel(context.Background())
	inputCtx, inputCtxCancel := context.WithCancel(context.Background())
	var gotMineCtx context.Context
	fakeMine = func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {}, nullBlockImmediately)
	inCh, outCh, _ = worker.Start(miningCtx)
	inCh <- NewInput(inputCtx, tipSet, rewardAddr, miningAddr)
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
	fakeMine = func(c context.Context, i Input, _ NullBlockTimerFunc, bg BlockGenerator, doSomeWork DoSomeWorkFunc, outCh chan<- Output) {
		gotMineCtx = c
		outCh <- Output{}
	}
	worker = NewWorkerWithDeps(mockBg, fakeMine, func() {}, nullBlockImmediately)
	inCh, outCh, doneWg := worker.Start(miningCtx)
	inCh <- NewInput(inputCtx, tipSet, rewardAddr, miningAddr)
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
	newAddr := types.NewAddressForTestGetter()
	addr := newAddr()
	miningAddr := newAddr()
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success.
	mockBg := &MockBlockGenerator{}
	outCh := make(chan Output)
	mockBg.On("Generate", mock.Anything, tipSet, uint64(0), addr, miningAddr).Return(next, nil)
	doSomeWorkCalled := false
	input := NewInput(ctx, tipSet, addr, miningAddr)
	go Mine(ctx, input, nullBlockImmediately, mockBg, func() { doSomeWorkCalled = true }, outCh)
	r := <-outCh
	assert.NoError(r.Err)
	assert.True(doSomeWorkCalled)
	assert.True(r.NewBlock.Cid().Equals(next.Cid()))
	mockBg.AssertExpectations(t)

	// Block generation fails.
	mockBg = &MockBlockGenerator{}
	outCh = make(chan Output)
	mockBg.On("Generate", mock.Anything, tipSet, uint64(0), addr, miningAddr).Return(nil, errors.New("boom"))
	doSomeWorkCalled = false
	input = NewInput(ctx, tipSet, addr, miningAddr)
	go Mine(ctx, input, nullBlockImmediately, mockBg, func() { doSomeWorkCalled = true }, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.True(doSomeWorkCalled)
	mockBg.AssertExpectations(t)

	// Null block count is increased until we find a winning ticket.
	mockBg = &MockBlockGenerator{}
	outCh = make(chan Output)
	mockBg.On("Generate", mock.Anything, tipSet, uint64(2), addr, miningAddr).Return(next, nil)
	workCount := 0
	input = NewInput(ctx, tipSet, addr, miningAddr)
	(func() {
		defer swap.Swap(&isWinningTicket, everyThirdWinningTicket())()
		go Mine(ctx, input, nullBlockImmediately, mockBg, func() { workCount++ }, outCh)
		r = <-outCh
	})()
	assert.NoError(r.Err)
	assert.Equal(3, workCount)
	assert.True(r.NewBlock.Cid().Equals(next.Cid()))
	mockBg.AssertExpectations(t)
}

func TestIsWinningTicket(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		ticket     byte
		myPower    int64
		totalPower int64
		wins       bool
	}{
		{0x00, 1, 5, true},
		{0x30, 1, 5, true},
		{0x40, 1, 5, false},
		{0xF0, 1, 5, false},
		{0x00, 5, 5, true},
		{0x33, 5, 5, true},
		{0x44, 5, 5, true},
		{0xFF, 5, 5, true},
		{0x00, 0, 5, false},
		{0x33, 0, 5, false},
		{0x44, 0, 5, false},
		{0xFF, 0, 5, false},
	}

	for _, c := range cases {
		ticket := [sha256.Size]byte{}
		ticket[0] = c.ticket
		r := isWinningTicket(ticket[:], c.myPower, c.totalPower)
		assert.Equal(c.wins, r, "%+v", c)
	}
}

func TestCreateChallenge(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		parentTickets  [][]byte
		nullBlockCount uint64
		challenge      string
	}{
		// From https://www.di-mgt.com.au/sha_testvectors.html
		{[][]byte{[]byte("ac"), []byte("ab"), []byte("xx")}, uint64('c'),
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{[][]byte{[]byte("z"), []byte("x"), []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnop")},
			uint64('q'), "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		{[][]byte{[]byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrst"), []byte("z"), []byte("x")},
			uint64('u'), "cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}

	for _, c := range cases {
		decoded, err := hex.DecodeString(c.challenge)
		assert.NoError(err)

		parents := core.TipSet{}
		for _, t := range c.parentTickets {
			b := types.Block{Ticket: t}
			parents[b.Cid().String()] = &b
		}
		r := createChallenge(parents, c.nullBlockCount)

		assert.Equal(decoded, r)
	}
}

// Implements NullBlockTimerFunc with the policy that it is always a good time
// to create a null block.
func nullBlockImmediately() {
}

// Returns a ticket checking function that return true every third time
func everyThirdWinningTicket() func(_ []byte, _, _ int64) bool {
	count := 0
	return func(_ []byte, _, _ int64) bool {
		count++
		return count%3 == 0
	}
}
