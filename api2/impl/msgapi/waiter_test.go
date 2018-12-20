package msgapi

import (
	"context"
	"sync"
	"testing"
	"time"

	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	return setupTestWithGif(require, consensus.InitGenesis)
}

func setupTestWithGif(require *require.Assertions, gif consensus.GenesisInitFunc) (*hamt.CborIpldStore, *chain.DefaultStore, *Waiter) {
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	chainStore, err := chain.Init(context.Background(), r, bs, cst, gif)
	require.NoError(err)
	waiter := NewWaiter(chainStore, bs, cst)
	return cst, chainStore, waiter
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
	sm1, err := types.NewSignedMessage(*m1, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	m2 := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm2, err := types.NewSignedMessage(*m2, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	baseTS := chainStore.Head()
	require.Equal(1, len(baseTS))
	baseBlock := baseTS.ToSlice()[0]

	require.NotNil(chainStore.GenesisCid())
	b1 := chain.RequireMkFakeChild(require, baseTS, chainStore.GenesisCid(), baseBlock.StateRoot, uint64(0), uint64(0))
	b1.Messages = []*types.SignedMessage{sm1}
	b1.Ticket = []byte{0} // block 1 comes first in message application
	core.MustPut(cst, b1)

	b2 := chain.RequireMkFakeChild(require, baseTS, chainStore.GenesisCid(), baseBlock.StateRoot, uint64(1), uint64(0))
	b2.Messages = []*types.SignedMessage{sm2}
	b2.Ticket = []byte{1}
	core.MustPut(cst, b2)

	ts := consensus.RequireNewTipSet(require, b1, b2)
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

// TODO ensure it returns an error
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
		assert.NoError(err)
	case <-time.After(2 * time.Second):
		assert.Fail("Wait should have returned when context was canceled")
	}
}
