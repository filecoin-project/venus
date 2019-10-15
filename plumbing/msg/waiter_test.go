package msg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
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
	d := requiredCommonDeps(t, th.DefaultGenesis)
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
	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}})
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
	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m3, m4}})

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
	chain := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}}, smsgsSet{smsgs{m3, m4}})
	// set the head without putting the ancestor block in the chainStore.
	err = chainStore.SetHead(ctx, chain[len(chain)-1])
	assert.Nil(t, err)

	testWaitHelp(nil, t, waiter, m2, true, nil)
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

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func newChainWithMessages(store *hamt.CborIpldStore, msgStore *chain.MessageStore, root types.TipSet, msgSets ...[][]*types.SignedMessage) []types.TipSet {
	var tipSets []types.TipSet
	parents := root
	height := uint64(0)
	stateRootCidGetter := types.NewCidForTestGetter()

	// only add root to the chain if it is not the zero-valued-tipset
	if parents.Defined() {
		for i := 0; i < parents.Len(); i++ {
			mustPut(store, parents.At(i))
		}
		tipSets = append(tipSets, parents)
		height, _ = parents.Height()
		height++
	}
	emptyMessagesCid, err := msgStore.StoreMessages(context.Background(), []*types.SignedMessage{}, []*types.SignedMessage{})
	if err != nil {
		panic(err)
	}
	emptyReceiptsCid, err := msgStore.StoreReceipts(context.Background(), []*types.MessageReceipt{})
	if err != nil {
		panic(err)
	}

	for _, tsMsgs := range msgSets {
		var blocks []*types.Block
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &types.Block{
				Height:          types.Uint64(height),
				Parents:         parents.Key(),
				Messages:        emptyMessagesCid,
				MessageReceipts: emptyReceiptsCid,
			}
			mustPut(store, child)
			blocks = append(blocks, child)
		}
		for _, msgs := range tsMsgs {
			msgsCid, err := msgStore.StoreMessages(context.Background(), msgs, []*types.SignedMessage{})
			if err != nil {
				panic(err)
			}

			child := &types.Block{
				Messages:        msgsCid,
				Parents:         parents.Key(),
				Height:          types.Uint64(height),
				StateRoot:       stateRootCidGetter(), // Differentiate all blocks
				MessageReceipts: emptyReceiptsCid,
			}
			mustPut(store, child)
			blocks = append(blocks, child)
		}
		ts, err := types.NewTipSet(blocks...)
		if err != nil {
			panic(err)
		}
		tipSets = append(tipSets, ts)
		parents = ts
		height++
	}

	return tipSets
}

// mustPut stores the thingy in the store or panics if it cannot.
func mustPut(store *hamt.CborIpldStore, thingy interface{}) cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}
