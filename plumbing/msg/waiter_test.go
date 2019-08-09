package msg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func testWaitHelp(wg *sync.WaitGroup, t *testing.T, waiter *Waiter, expectMsg *types.SignedMessage, expectError bool, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) {
	expectCid, err := expectMsg.Cid()
	if cb == nil {
		cb = func(b *types.Block, msg *types.SignedMessage,
			rcp *types.MessageReceipt) error {
			assert.True(t, types.SmsgCidsEqual(expectMsg, msg))
			if wg != nil {
				wg.Done()
			}

			return nil
		}
	}
	assert.NoError(t, err)

	err = waiter.Wait(context.Background(), expectCid, cb)
	assert.Equal(t, expectError, err != nil)
}

type smsgs []*types.SignedMessage
type smsgsSet [][]*types.SignedMessage

func setupTest(t *testing.T) (*hamt.CborIpldStore, *chain.Store, *chain.MessageStore, *Waiter) {
	d := requiredCommonDeps(t, consensus.DefaultGenesis)
	return d.cst, d.chainStore, d.messages, NewWaiter(d.chainStore, d.messages, d.blockstore, d.cst)
}

func setupTestWithGif(t *testing.T, gif consensus.GenesisInitFunc) (*hamt.CborIpldStore, *chain.Store, *chain.MessageStore, *Waiter) {
	d := requiredCommonDeps(t, gif)
	return d.cst, d.chainStore, d.messages, NewWaiter(d.chainStore, d.messages, d.blockstore, d.cst)
}

func TestWait(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst, chainStore, msgStore, waiter := setupTest(t)

	testWaitExisting(ctx, t, cst, chainStore, msgStore, waiter)
	testWaitNew(ctx, t, cst, chainStore, msgStore, waiter)
}

func testWaitExisting(ctx context.Context, t *testing.T, cst *hamt.CborIpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	m1, m2 := newSignedMessage(), newSignedMessage()
	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	chainWithMsgs := core.NewChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}})
	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(t, 1, ts.Len())
	require.NoError(t, chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	}))
	require.NoError(t, chainStore.SetHead(ctx, ts))

	testWaitHelp(nil, t, waiter, m1, false, nil)
	testWaitHelp(nil, t, waiter, m2, false, nil)
}

func testWaitNew(ctx context.Context, t *testing.T, cst *hamt.CborIpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	var wg sync.WaitGroup

	_, _ = newSignedMessage(), newSignedMessage() // flush out so we get distinct messages from testWaitExisting
	m3, m4 := newSignedMessage(), newSignedMessage()
	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	chainWithMsgs := core.NewChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m3, m4}})

	wg.Add(2)
	go testWaitHelp(&wg, t, waiter, m3, false, nil)
	go testWaitHelp(&wg, t, waiter, m4, false, nil)
	time.Sleep(10 * time.Millisecond)

	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(t, 1, ts.Len())
	require.NoError(t, chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	}))
	require.NoError(t, chainStore.SetHead(ctx, ts))

	wg.Wait()
}

func TestWaitError(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst, chainStore, msgStore, waiter := setupTest(t)

	testWaitError(ctx, t, cst, chainStore, msgStore, waiter)
}

func testWaitError(ctx context.Context, t *testing.T, cst *hamt.CborIpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	m1, m2, m3, m4 := newSignedMessage(), newSignedMessage(), newSignedMessage(), newSignedMessage()
	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	chain := core.NewChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}}, smsgsSet{smsgs{m3, m4}})
	// set the head without putting the ancestor block in the chainStore.
	err = chainStore.SetHead(ctx, chain[len(chain)-1])
	assert.Nil(t, err)

	testWaitHelp(nil, t, waiter, m2, true, nil)
}

func TestWaitConflicting(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
	worker1, worker2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	// create a valid miner
	minerAddr := mockSigner.Addresses[3]

	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(addr1, types.NewAttoFILFromFIL(10000)),
		consensus.ActorAccount(addr2, types.NewAttoFILFromFIL(0)),
		consensus.ActorAccount(addr3, types.NewAttoFILFromFIL(0)),
		consensus.MinerActor(minerAddr, addr3, th.RequireRandomPeerID(t), types.ZeroAttoFIL, types.OneKiBSectorSize),
	)
	cst, chainStore, msgStore, waiter := setupTestWithGif(t, testGen)

	// Create conflicting messages
	m1 := types.NewMessage(addr1, addr3, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm1, err := types.NewSignedMessage(*m1, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	m2 := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm2, err := types.NewSignedMessage(*m2, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	baseTS := headTipSet
	require.Equal(t, 1, baseTS.Len())
	baseBlock := baseTS.ToSlice()[0]

	require.NotNil(t, chainStore.GenesisCid())

	emptyReceiptsCid, err := msgStore.StoreReceipts(ctx, []*types.MessageReceipt{})
	require.NoError(t, err)

	b1 := th.RequireMkFakeChild(t,
		th.FakeChildParams{
			MinerAddr:   minerAddr,
			Parent:      baseTS,
			GenesisCid:  chainStore.GenesisCid(),
			StateRoot:   baseBlock.StateRoot,
			Signer:      mockSigner,
			MinerWorker: worker1,
		})
	sm1Cid, err := msgStore.StoreMessages(ctx, []*types.SignedMessage{sm1})
	require.NoError(t, err)
	b1.Messages = sm1Cid
	b1.MessageReceipts = emptyReceiptsCid
	b1.Ticket = []byte{0} // block 1 comes first in message application
	core.MustPut(cst, b1)

	b2 := th.RequireMkFakeChild(t,
		th.FakeChildParams{
			MinerAddr:   minerAddr,
			Parent:      baseTS,
			GenesisCid:  chainStore.GenesisCid(),
			StateRoot:   baseBlock.StateRoot,
			Signer:      mockSigner,
			MinerWorker: worker2,
			Nonce:       uint64(1),
		})
	sm2Cid, err := msgStore.StoreMessages(ctx, []*types.SignedMessage{sm2})
	require.NoError(t, err)
	b2.Messages = sm2Cid
	b2.MessageReceipts = emptyReceiptsCid
	b2.Ticket = []byte{1}
	core.MustPut(cst, b2)

	ts := th.RequireNewTipSet(t, b1, b2)
	require.NoError(t, chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: baseBlock.StateRoot,
	}))
	assert.NoError(t, chainStore.SetHead(ctx, ts))
	msgApplySucc := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.NotNil(t, rcp)
		return nil
	}
	msgApplyFail := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.Nil(t, rcp)
		return nil
	}

	testWaitHelp(nil, t, waiter, sm1, false, msgApplySucc)
	testWaitHelp(nil, t, waiter, sm2, false, msgApplyFail)
}

func TestWaitRespectsContextCancel(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	_, _, _, waiter := setupTest(t)

	failIfCalledCb := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.Fail(t, "Should not be called -- message doesnt exist")
		return nil
	}

	var err error
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		err = waiter.Wait(ctx, types.CidFromString(t, "somecid"), failIfCalledCb)
	}()

	cancel()

	select {
	case <-doneCh:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		assert.Fail(t, "Wait should have returned when context was canceled")
	}
}
