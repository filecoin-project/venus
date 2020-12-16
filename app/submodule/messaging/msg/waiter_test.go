package msg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	e "github.com/filecoin-project/venus/pkg/enccid"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func testWaitHelp(wg *sync.WaitGroup, t *testing.T, waiter *Waiter, expectMsg *types.SignedMessage, expectError bool) {
	_, err := expectMsg.Cid()
	assert.NoError(t, err)

	chainMessage, err := waiter.Wait(context.Background(), expectMsg, constants.DefaultConfidence, constants.DefaultMessageWaitLookback)
	assert.Equal(t, expectError, err != nil)
	assert.NotNil(t, chainMessage)
}

type smsgs []*types.SignedMessage
type smsgsSet [][]*types.SignedMessage

type chainReader interface {
	chain.TipSetProvider
	GetHead() block.TipSetKey
	GetTipSetReceiptsRoot(block.TipSetKey) (cid.Cid, error)
	GetTipSetStateRoot(block.TipSetKey) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*chain.HeadChange
	SubscribeHeadChanges(chain.ReorgNotifee)
}
type stateReader interface {
	ResolveAddressAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (address.Address, error)
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*types.Actor, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
}

func setupTest(t *testing.T) (cbor.IpldStore, *chain.Store, *chain.MessageStore, *Waiter) {
	d := requiredCommonDeps(t, gengen.DefaultGenesis)

	combineChainReader := struct {
		stateReader
		chainReader
	}{
		d.chainState,
		d.chainStore,
	}
	return d.cst, d.chainStore, d.messages, NewWaiter(combineChainReader, d.messages, d.blockstore, d.cst)
}

func TestWait(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst, chainStore, msgStore, waiter := setupTest(t)

	testWaitExisting(ctx, t, cst, chainStore, msgStore, waiter)
	testWaitNew(ctx, t, cst, chainStore, msgStore, waiter)
}

func testWaitExisting(ctx context.Context, t *testing.T, cst cbor.IpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	m1, m2 := newSignedMessage(0), newSignedMessage(1)
	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}})
	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(t, 1, ts.Len())
	require.NoError(t, chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].ParentStateRoot.Cid,
		TipSetReceipts:  ts.ToSlice()[0].ParentMessageReceipts.Cid,
	}))
	require.NoError(t, chainStore.SetHead(ctx, ts))

	testWaitHelp(nil, t, waiter, m1, false)
	testWaitHelp(nil, t, waiter, m2, false)
}

func testWaitNew(ctx context.Context, t *testing.T, cst cbor.IpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	var wg sync.WaitGroup

	_, _ = newSignedMessage(0), newSignedMessage(1) // flush out so we get distinct messages from testWaitExisting
	m3, m4 := newSignedMessage(0), newSignedMessage(1)

	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m3, m4}})

	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(t, 1, ts.Len())
	require.NoError(t, chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].ParentStateRoot.Cid,
		TipSetReceipts:  ts.ToSlice()[0].ParentMessageReceipts.Cid,
	}))
	require.NoError(t, chainStore.SetHead(ctx, ts))

	time.Sleep(5 * time.Millisecond)

	wg.Add(2)
	go testWaitHelp(&wg, t, waiter, m3, false)
	go testWaitHelp(&wg, t, waiter, m4, false)
	wg.Wait()
}

func TestWaitError(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst, chainStore, msgStore, waiter := setupTest(t)

	testWaitError(ctx, t, cst, chainStore, msgStore, waiter)
}

func testWaitError(ctx context.Context, t *testing.T, cst cbor.IpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
	m1, m2, m3, m4 := newSignedMessage(1), newSignedMessage(2), newSignedMessage(3), newSignedMessage(4)
	head := chainStore.GetHead()
	headTipSet, err := chainStore.GetTipSet(head)
	require.NoError(t, err)
	tss := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}}, smsgsSet{smsgs{m3, m4}})
	// set the head without putting the ancestor block in the chainStore.
	err = chainStore.SetHead(ctx, tss[len(tss)-1])
	assert.Nil(t, err)

	testWaitHelp(nil, t, waiter, m2, true)
}

func TestWaitRespectsContextCancel(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	_, _, _, waiter := setupTest(t)

	var err error
	var chainMessage *ChainMessage
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		chainMessage, err = waiter.Wait(ctx, newSignedMessage(0), constants.DefaultConfidence, constants.DefaultMessageWaitLookback)
	}()

	cancel()

	select {
	case <-doneCh:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		assert.Fail(t, "Wait should have returned when context was canceled")
	}
	assert.Nil(t, chainMessage)
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func newChainWithMessages(store cbor.IpldStore, msgStore *chain.MessageStore, root *block.TipSet, msgSets ...[][]*types.SignedMessage) []*block.TipSet {
	var tipSets []*block.TipSet
	parents := root
	height := abi.ChainEpoch(0)
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
	emptyTxMeta, err := msgStore.StoreMessages(context.Background(), []*types.SignedMessage{}, []*types.UnsignedMessage{})
	if err != nil {
		panic(err)
	}
	emptyReceiptsCid, err := msgStore.StoreReceipts(context.Background(), []types.MessageReceipt{})
	if err != nil {
		panic(err)
	}

	for _, tsMsgs := range msgSets {
		var blocks []*block.Block
		var receipts []types.MessageReceipt
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &block.Block{
				Height:                height,
				Parents:               parents.Key(),
				Messages:              e.NewCid(emptyTxMeta),
				ParentMessageReceipts: e.NewCid(emptyReceiptsCid),
			}
			mustPut(store, child)
			blocks = append(blocks, child)
		}
		for _, msgs := range tsMsgs {
			for _, msg := range msgs {
				c, err := msg.Cid()
				if err != nil {
					panic(err)
				}
				receipts = append(receipts, types.MessageReceipt{ExitCode: 0, ReturnValue: c.Bytes(), GasUsed: types.Zero})
			}
			txMeta, err := msgStore.StoreMessages(context.Background(), msgs, []*types.UnsignedMessage{})
			if err != nil {
				panic(err)
			}

			child := &block.Block{
				Messages:        e.NewCid(txMeta),
				Parents:         parents.Key(),
				Height:          height,
				ParentStateRoot: e.NewCid(stateRootCidGetter()), // Differentiate all blocks
			}
			blocks = append(blocks, child)
		}
		receiptCid, err := msgStore.StoreReceipts(context.TODO(), receipts)
		if err != nil {
			panic(err)
		}

		for _, blk := range blocks {
			blk.ParentMessageReceipts = e.NewCid(receiptCid)
			mustPut(store, blk)
		}

		ts, err := block.NewTipSet(blocks...)
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
func mustPut(store cbor.IpldStore, thingy interface{}) cid.Cid {
	c, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return c
}
