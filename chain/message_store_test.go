package chain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageStoreMessagesHappy(t *testing.T) {
	ctx := context.Background()
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	msgs := []*types.SignedMessage{
		mm.NewSignedMessage(alice, 0),
		mm.NewSignedMessage(alice, 1),
		mm.NewSignedMessage(bob, 0),
		mm.NewSignedMessage(alice, 2),
		mm.NewSignedMessage(alice, 3),
		mm.NewSignedMessage(bob, 1),
		mm.NewSignedMessage(alice, 4),
		mm.NewSignedMessage(bob, 2),
	}

	ms := chain.NewMessageStore(hamt.NewCborStore())
	msgsCid, err := ms.StoreMessages(ctx, msgs)
	assert.NoError(t, err)

	rtMsgs, err := ms.LoadMessages(ctx, msgsCid)
	assert.NoError(t, err)

	assert.Equal(t, msgs, rtMsgs)
}

func TestMessageStoreReceiptsHappy(t *testing.T) {
	ctx := context.Background()
	mr := NewReceiptMaker()

	receipts := []*types.MessageReceipt{
		mr.NewReceipt(),
		mr.NewReceipt(),
		mr.NewReceipt(),
	}

	ms := chain.NewMessageStore(hamt.NewCborStore())
	receiptCids, err := ms.StoreReceipts(ctx, receipts)
	assert.NoError(t, err)

	rtReceipts, err := ms.LoadReceipts(ctx, receiptCids)
	assert.NoError(t, err)

	assert.Equal(t, receipts, rtReceipts)
}

type ReceiptMaker struct {
	seq uint
}

// NewReceiptMaker creates a new receipt maker
func NewReceiptMaker() *ReceiptMaker {
	return &ReceiptMaker{0}
}

// NewReceipt creates a new distinct receipt.
func (rm *ReceiptMaker) NewReceipt() *types.MessageReceipt {
	seq := rm.seq
	rm.seq++
	return &types.MessageReceipt{Return: [][]byte{[]byte(fmt.Sprintf("%d", seq))}}
}
