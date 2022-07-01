package chain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func TestMessageStoreMessagesHappy(t *testing.T) {
	testflags.UnitTest(t)
	ctx := context.Background()
	keys := testhelpers.MustGenerateKeyInfo(2, 42)
	mm := testhelpers.NewMessageMaker(t, keys)

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

	bs := blockstoreutil.Adapt(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	ms := chain.NewMessageStore(bs, config.DefaultForkUpgradeParam)
	msgsCid, err := ms.StoreMessages(ctx, msgs, []*types.Message{})
	assert.NoError(t, err)

	rtMsgs, _, err := ms.LoadMetaMessages(ctx, msgsCid)
	assert.NoError(t, err)

	assert.Equal(t, msgs, rtMsgs)
}

func TestMessageStoreReceiptsHappy(t *testing.T) {
	ctx := context.Background()
	mr := testhelpers.NewReceiptMaker()

	receipts := []types.MessageReceipt{
		mr.NewReceipt(),
		mr.NewReceipt(),
		mr.NewReceipt(),
	}

	bs := blockstoreutil.Adapt(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	ms := chain.NewMessageStore(bs, config.DefaultForkUpgradeParam)
	receiptCids, err := ms.StoreReceipts(ctx, receipts)
	assert.NoError(t, err)

	rtReceipts, err := ms.LoadReceipts(ctx, receiptCids)
	assert.NoError(t, err)
	assert.Equal(t, receipts, rtReceipts)
}
