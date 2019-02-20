package msg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)
var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func testWaitHelp(wg *sync.WaitGroup, assert *assert.Assertions, waiter *Waiter, expectMsg *types.SignedMessage, expectError bool, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) {
	expectCid, err := expectMsg.Cid()
	if cb == nil {
		cb = func(b *types.Block, msg *types.SignedMessage,
			rcp *types.MessageReceipt) error {
			assert.True(types.SmsgCidsEqual(expectMsg, msg))
			if wg != nil {
				wg.Done()
			}

			return nil
		}
	}
	assert.NoError(err)

	err = waiter.Wait(context.Background(), expectCid, cb)
	assert.Equal(expectError, err != nil)
}

type smsgs []*types.SignedMessage
type smsgsSet [][]*types.SignedMessage

func setupTest(require *require.Assertions) (*hamt.CborIpldStore, *chain.DefaultStore, *Waiter) {
	d := requireCommonDeps(require)
	return d.cst, d.chainStore, NewWaiter(d.chainStore, d.blockstore, d.cst)
}

func setupTestWithGif(require *require.Assertions, gif consensus.GenesisInitFunc) (*hamt.CborIpldStore, *chain.DefaultStore, *Waiter) {
	d := requireCommonDepsWithGif(require, gif)
	return d.cst, d.chainStore, NewWaiter(d.chainStore, d.blockstore, d.cst)
}

func TestWait(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	cst, chainStore, waiter := setupTest(require)

	testWaitExisting(ctx, assert, require, cst, chainStore, waiter)
	testWaitNew(ctx, assert, require, cst, chainStore, waiter)
}

func testWaitExisting(ctx context.Context, assert *assert.Assertions, require *require.Assertions, cst *hamt.CborIpldStore, chainStore *chain.DefaultStore, waiter *Waiter) {
	m1, m2 := newSignedMessage(), newSignedMessage()
	chainWithMsgs := core.NewChainWithMessages(cst, chainStore.Head(), smsgsSet{smsgs{m1, m2}})
	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(1, len(ts))
	chain.RequirePutTsas(ctx, require, chainStore, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	})
	require.NoError(chainStore.SetHead(ctx, ts))

	testWaitHelp(nil, assert, waiter, m1, false, nil)
	testWaitHelp(nil, assert, waiter, m2, false, nil)
}

func testWaitNew(ctx context.Context, assert *assert.Assertions, require *require.Assertions, cst *hamt.CborIpldStore, chainStore *chain.DefaultStore, waiter *Waiter) {
	var wg sync.WaitGroup

	_, _ = newSignedMessage(), newSignedMessage() // flush out so we get distinct messages from testWaitExisting
	m3, m4 := newSignedMessage(), newSignedMessage()
	chainWithMsgs := core.NewChainWithMessages(cst, chainStore.Head(), smsgsSet{smsgs{m3, m4}})

	wg.Add(2)
	go testWaitHelp(&wg, assert, waiter, m3, false, nil)
	go testWaitHelp(&wg, assert, waiter, m4, false, nil)
	time.Sleep(10 * time.Millisecond)

	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(1, len(ts))
	chain.RequirePutTsas(ctx, require, chainStore, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	})
	require.NoError(chainStore.SetHead(ctx, ts))

	wg.Wait()
}

func TestWaitError(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	cst, chainStore, waiter := setupTest(require)

	testWaitError(ctx, assert, require, cst, chainStore, waiter)
}

func testWaitError(ctx context.Context, assert *assert.Assertions, require *require.Assertions, cst *hamt.CborIpldStore, chainStore *chain.DefaultStore, waiter *Waiter) {
	m1, m2, m3, m4 := newSignedMessage(), newSignedMessage(), newSignedMessage(), newSignedMessage()
	chain := core.NewChainWithMessages(cst, chainStore.Head(), smsgsSet{smsgs{m1, m2}}, smsgsSet{smsgs{m3, m4}})
	// set the head without putting the ancestor block in the chainStore.
	err := chainStore.SetHead(ctx, chain[len(chain)-1])
	assert.Nil(err)

	testWaitHelp(nil, assert, waiter, m2, true, nil)
}

func TestWaitConflicting(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(addr1, types.NewAttoFILFromFIL(10000)),
		consensus.ActorAccount(addr2, types.NewAttoFILFromFIL(0)),
		consensus.ActorAccount(addr3, types.NewAttoFILFromFIL(0)),
	)
	cst, chainStore, waiter := setupTestWithGif(require, testGen)

	// Create conflicting messages
	m1 := types.NewMessage(addr1, addr3, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm1, err := types.NewSignedMessage(*m1, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	m2 := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm2, err := types.NewSignedMessage(*m2, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	baseTS := chainStore.Head()
	require.Equal(1, len(baseTS))
	baseBlock := baseTS.ToSlice()[0]

	require.NotNil(chainStore.GenesisCid())

	b1 := chain.RequireMkFakeChild(require,
		chain.FakeChildParams{
			Parent: baseTS, GenesisCid: chainStore.GenesisCid(), StateRoot: baseBlock.StateRoot})
	b1.Messages = []*types.SignedMessage{sm1}
	b1.Ticket = []byte{0} // block 1 comes first in message application
	core.MustPut(cst, b1)

	b2 := chain.RequireMkFakeChild(require,
		chain.FakeChildParams{
			Parent: baseTS, GenesisCid: chainStore.GenesisCid(),
			StateRoot: baseBlock.StateRoot, Nonce: uint64(1)})
	b2.Messages = []*types.SignedMessage{sm2}
	b2.Ticket = []byte{1}
	core.MustPut(cst, b2)

	ts := testhelpers.RequireNewTipSet(require, b1, b2)
	chain.RequirePutTsas(ctx, require, chainStore, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: baseBlock.StateRoot,
	})
	chainStore.SetHead(ctx, ts)
	msgApplySucc := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.NotNil(rcp)
		return nil
	}
	msgApplyFail := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.Nil(rcp)
		return nil
	}

	testWaitHelp(nil, assert, waiter, sm1, false, msgApplySucc)
	testWaitHelp(nil, assert, waiter, sm2, false, msgApplyFail)
}

func TestWaitRespectsContextCancel(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	_, _, waiter := setupTest(require)

	failIfCalledCb := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.Fail("Should not be called -- message doesnt exist")
		return nil
	}

	var err error
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		err = waiter.Wait(ctx, types.SomeCid(), failIfCalledCb)
	}()

	cancel()

	select {
	case <-doneCh:
		assert.Error(err)
	case <-time.After(2 * time.Second):
		assert.Fail("Wait should have returned when context was canceled")
	}
}
