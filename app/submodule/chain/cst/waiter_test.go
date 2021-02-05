package cst

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

//func testWaitHelp(wg *sync.WaitGroup, t *testing.T, waiter *Waiter, expectMsg *types.SignedMessage, expectError bool) {
//	_, err := expectMsg.Cid()
//	assert.NoError(t, err)
//
//	_, err = waiter.Wait(context.Background(), expectMsg, constants.DefaultConfidence, constants.DefaultMessageWaitLookback)
//	if wg != nil {
//		wg.Done()
//	}
//
//	assert.Equal(t, expectError, err != nil)
//}
//
//type smsgs []*types.SignedMessage
//type smsgsSet [][]*types.SignedMessage

type chainReader interface {
	chain.TipSetProvider
	GetHead() *types.TipSet
	GetTipSetReceiptsRoot(*types.TipSet) (cid.Cid, error)
	GetTipSetStateRoot(*types.TipSet) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*chain.HeadChange
	SubscribeHeadChanges(chain.ReorgNotifee)
}
type stateReader interface {
	ResolveAddressAt(context.Context, *types.TipSet, address.Address) (address.Address, error)
	GetActorAt(context.Context, *types.TipSet, address.Address) (*types.Actor, error)
	GetTipSetState(context.Context, *types.TipSet) (state.Tree, error)
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

//func TestWait(t *testing.T) {
//	tf.UnitTest(t)
//
//	ctx := context.Background()
//	cst, chainStore, msgStore, waiter := setupTest(t)
//
//	testWaitExisting(ctx, t, cst, chainStore, msgStore, waiter)
//	testWaitNew(ctx, t, cst, chainStore, msgStore, waiter)
//}

//func testWaitExisting(ctx context.Context, t *testing.T, cst cbor.IpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
//	m1, m2 := newSignedMessage(0), newSignedMessage(1)
//	head := chainStore.GetHead()
//	headTipSet, err := chainStore.GetTipSet(head)
//	require.NoError(t, err)
//	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m1, m2}})
//	ts := chainWithMsgs[len(chainWithMsgs)-1]
//	require.Equal(t, 1, ts.Len())
//	require.NoError(t, chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
//		TipSet:          ts,
//		TipSetStateRoot: ts.ToSlice()[0].ParentStateRoot,
//		TipSetReceipts:  ts.ToSlice()[0].ParentMessageReceipts,
//	}))
//	require.NoError(t, chainStore.SetHead(ctx, ts))
//
//	testWaitHelp(nil, t, waiter, m1, false)
//	testWaitHelp(nil, t, waiter, m2, false)
//}
//
//func testWaitNew(ctx context.Context, t *testing.T, cst cbor.IpldStore, chainStore *chain.Store, msgStore *chain.MessageStore, waiter *Waiter) {
//	var wg sync.WaitGroup
//
//	_, _ = newSignedMessage(0), newSignedMessage(1) // flush out so we get distinct messages from testWaitExisting
//	m3, m4 := newSignedMessage(0), newSignedMessage(1)
//
//	head := chainStore.GetHead()
//	headTipSet, err := chainStore.GetTipSet(head)
//	require.NoError(t, err)
//	chainWithMsgs := newChainWithMessages(cst, msgStore, headTipSet, smsgsSet{smsgs{m3, m4}})
//
//	ts := chainWithMsgs[len(chainWithMsgs)-1]
//	require.Equal(t, 1, ts.Len())
//	require.NoError(t, chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
//		TipSet:          ts,
//		TipSetStateRoot: ts.ToSlice()[0].ParentStateRoot,
//		TipSetReceipts:  ts.ToSlice()[0].ParentMessageReceipts,
//	}))
//	require.NoError(t, chainStore.SetHead(ctx, ts))
//
//	time.Sleep(5 * time.Millisecond)
//
//	wg.Add(2)
//	go testWaitHelp(&wg, t, waiter, m3, false)
//	go testWaitHelp(&wg, t, waiter, m4, false)
//	wg.Wait()
//}

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
		fmt.Println(err)
		//assert.Error(t, err)
	case <-time.After(2 * time.Second):
		assert.Fail(t, "Wait should have returned when context was canceled")
	}
	assert.Nil(t, chainMessage)
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
//func newChainWithMessages(store cbor.IpldStore, msgStore *chain.MessageStore, root *block.TipSet, msgSets ...[][]*types.SignedMessage) []*block.TipSet {
//	var tipSets []*block.TipSet
//	parents := root
//	height := abi.ChainEpoch(0)
//	stateRootCidGetter := types.NewCidForTestGetter()
//	addrGetter := types.NewForTestGetter()
//	// only add root to the chain if it is not the zero-valued-tipset
//	if parents.Defined() {
//		for i := 0; i < parents.Len(); i++ {
//			mustPut(store, parents.At(i))
//		}
//		tipSets = append(tipSets, parents)
//		height, _ = parents.Height()
//		height++
//	}
//	emptyTxMeta, err := msgStore.StoreMessages(context.Background(), []*types.SignedMessage{}, []*types.UnsignedMessage{})
//	if err != nil {
//		panic(err)
//	}
//	emptyReceiptsCid, err := msgStore.StoreReceipts(context.Background(), []types.MessageReceipt{})
//	if err != nil {
//		panic(err)
//	}
//
//	for _, tsMsgs := range msgSets {
//		var blocks []*block.BlockHeader
//		var receipts []types.MessageReceipt
//		// If a message set does not contain a slice of messages then
//		// add a tipset with no messages and a single block to the chain
//		if len(tsMsgs) == 0 {
//			child := &block.BlockHeader{
//				Miner:                 addrGetter(),
//				Height:                height,
//				Parents:               parents.Key(),
//				Messages:              emptyTxMeta,
//				ParentMessageReceipts: emptyReceiptsCid,
//				ParentStateRoot:       stateRootCidGetter(),
//			}
//			mustPut(store, child)
//			blocks = append(blocks, child)
//		}
//		for _, msgs := range tsMsgs {
//			for _, msg := range msgs {
//				c, err := msg.Cid()
//				if err != nil {
//					panic(err)
//				}
//				receipts = append(receipts, types.MessageReceipt{ExitCode: 0, ReturnValue: c.Bytes(), GasUsed: 0})
//			}
//			txMeta, err := msgStore.StoreMessages(context.Background(), msgs, []*types.UnsignedMessage{})
//			if err != nil {
//				panic(err)
//			}
//
//			child := &block.BlockHeader{
//				Miner:           addrGetter(),
//				Messages:        txMeta,
//				Parents:         parents.Key(),
//				Height:          height,
//				ParentStateRoot: stateRootCidGetter(), // Differentiate all blocks
//			}
//			blocks = append(blocks, child)
//		}
//		receiptCid, err := msgStore.StoreReceipts(context.TODO(), receipts)
//		if err != nil {
//			panic(err)
//		}
//
//		for _, blk := range blocks {
//			blk.ParentMessageReceipts = receiptCid
//			mustPut(store, blk)
//		}
//
//		ts, err := block.NewTipSet(blocks...)
//		if err != nil {
//			panic(err)
//		}
//		tipSets = append(tipSets, ts)
//		parents = ts
//		height++
//	}
//
//	return tipSets
//}

// mustPut stores the thingy in the store or panics if it cannot.
//func mustPut(store cbor.IpldStore, thingy interface{}) cid.Cid {
//	c, err := store.Put(context.Background(), thingy)
//	if err != nil {
//		panic(err)
//	}
//	return c
//}
