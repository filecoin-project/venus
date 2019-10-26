package chain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	ms := chain.NewMessageStore(bs)
	msgsCid, err := ms.StoreMessages(ctx, msgs, []*types.UnsignedMessage{})
	assert.NoError(t, err)

	rtMsgs, _, err := ms.LoadMessages(ctx, msgsCid)
	assert.NoError(t, err)

	assert.Equal(t, msgs, rtMsgs)
}

func TestMessageStoreReceiptsHappy(t *testing.T) {
	ctx := context.Background()
	mr := types.NewReceiptMaker()

	receipts := []*types.MessageReceipt{
		mr.NewReceipt(),
		mr.NewReceipt(),
		mr.NewReceipt(),
	}

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	ms := chain.NewMessageStore(bs)
	receiptCids, err := ms.StoreReceipts(ctx, receipts)
	assert.NoError(t, err)

	rtReceipts, err := ms.LoadReceipts(ctx, receiptCids)
	assert.NoError(t, err)

	assert.Equal(t, receipts, rtReceipts)
}
