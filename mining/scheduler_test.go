package mining_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/block"
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	. "github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
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
// will win after 10 rounds and verifies that the output has 10 tickets and a
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
		Tickets:   []block.Ticket{baseTicket},
	}
	baseTs, err := block.NewTipSet(baseBlock)
	require.NoError(t, err)

	st, pool, _, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}
	messages := chain.NewMessageStore(bs)

	api := th.NewFakeWorkerPorcelainAPI(addr, 10, minerToWorker)

	worker := NewDefaultWorker(WorkerParameters{
		API: api,

		MinerAddr:      addr,
		MinerOwnerAddr: addr,
		WorkerSigner:   mockSigner,

		GetStateTree: getStateTree,
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,
		Election:     &consensus.ElectionMachine{},
		TicketGen:    &consensus.TicketMachine{},

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
	assert.Equal(t, uint64(10), uint64(block.Height))
	assert.Equal(t, 10, len(block.Tickets))
}

func TestSchedulerPassesValue(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())

	checkValsMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		assert.Equal(t, ctx, c) // individual run ctx splits off from mining ctx
		assert.Equal(t, inTS, ts)
		outCh <- Output{}
		return true, block.Ticket{}
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

	nothingMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		outCh <- Output{}
		return false, block.Ticket{}
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

// If head is the same append the previous ticket to the the ticket array,
// otherwise use an empty ticket array.
func TestSchedulerUpdatesTicketArray(t *testing.T) {
	tf.UnitTest(t)

	ts := newTestUtils(t)
	ctx, cancel := context.WithCancel(context.Background())
	blk2 := &block.Block{StateRoot: types.CidFromString(t, "somecid"), Height: 1}
	ts2 := th.RequireNewTipSet(t, blk2)

	expectedTArr := []block.Ticket{}
	checkTArrMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		select {
		case <-ctx.Done():
			return false, block.Ticket{}
		default:
		}
		assert.Equal(t, expectedTArr, tArr)
		outCh <- Output{}
		return false, NthTicket(uint8(len(tArr)))
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
	expectedTArr = []block.Ticket{NthTicket(0)}
	<-outCh
	expectedTArr = []block.Ticket{NthTicket(0), NthTicket(1)}
	<-outCh
	expectedTArr = []block.Ticket{NthTicket(0), NthTicket(1), NthTicket(2)}
	<-outCh
	expectedTArr = []block.Ticket{NthTicket(0), NthTicket(1), NthTicket(2), NthTicket(3)}
	<-outCh
	head = ts2
	expectedTArr = []block.Ticket{}
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

	checkValsMine := func(c context.Context, ts block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		assert.Equal(t, ts, checkTS)
		outCh <- Output{}
		return false, block.Ticket{}
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
	checkValsMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		assert.Equal(t, inTS, ts3)
		outCh <- Output{}
		return false, block.Ticket{}
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
	blk2 := &block.Block{StateRoot: types.SomeCid(), Height: 0}
	ts2 := consensus.RequireNewTipSet(require, blk2)
	blockingMine := func(c context.Context, ts block.TipSet, tArr []block.Ticket, outCh chan<- Output) {
		time.Sleep(th.BlockTimeTest)
		assert.Equal(ts, ts1)
		outCh <- Output{NewBlock: blk1}
	}
	var head block.TipSet
	headFunc := func() block.TipSet {
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
	var head block.TipSet
	headFunc := func() (block.TipSet, error) {
		return head, nil
	}
	shouldCancelMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		mineTimer := time.NewTimer(th.BlockTimeTest)
		select {
		case <-mineTimer.C:
			t.Fatal("should not take whole time")
		case <-c.Done():
		}
		return false, block.Ticket{}
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

	checkValsMine := func(c context.Context, inTS block.TipSet, tArr []block.Ticket, outCh chan<- Output) (bool, block.Ticket) {
		assert.Equal(t, inTS, checkTS)
		outCh <- Output{}
		return false, block.Ticket{}
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
