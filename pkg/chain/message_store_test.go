// stm: #unit
package chain_test

import (
	"context"
	"io"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cborString string

func (cs cborString) MarshalCBOR(w io.Writer) error {
	cw := cbg.NewCborWriter(w)
	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(cs))); err != nil {
		return err
	}
	_, err := io.WriteString(w, string(cs))
	return err
}

func (cs *cborString) UnmarshalCBOR(r io.Reader) error {
	sval, err := cbg.ReadString(cbg.NewCborReader(r))
	if err != nil {
		return err
	}
	*cs = cborString(sval)
	return nil
}

//func TestCborString(t *testing.T) {
//	rawString := "hello world"
//	cborString := cborString(rawString)
//	buf := bytes.NewBuffer(nil)
//	assert.NoError(t, cborString.MarshalCBOR(buf))
//	assert.NoError(t, cborString.UnmarshalCBOR(buf))
//	assert.Equal(t, string(cborString), rawString)
//}

func TestMessageStoreMessagesHappy(t *testing.T) {
	testflags.UnitTest(t)
	ctx := context.Background()
	keys := testhelpers.MustGenerateKeyInfo(2, 42)
	mm := testhelpers.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	signedMsgs := []*types.SignedMessage{
		mm.NewSignedMessage(alice, 0),
		mm.NewSignedMessage(alice, 1),
		mm.NewSignedMessage(bob, 0),
		mm.NewSignedMessage(alice, 2),
		mm.NewSignedMessage(alice, 3),
		mm.NewSignedMessage(bob, 1),
		mm.NewSignedMessage(alice, 4),
		mm.NewSignedMessage(bob, 2),
	}

	unsignedMsgs := []*types.Message{
		mm.NewUnsignedMessage(alice, 4),
	}

	bs := blockstoreutil.Adapt(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	ms := chain.NewMessageStore(bs, config.DefaultForkUpgradeParam)
	// stm: @CHAIN_MESSAGE_STORE_MESSAGES_001
	msgsCid, err := ms.StoreMessages(ctx, signedMsgs, unsignedMsgs)
	assert.NoError(t, err)

	// stm: @CHAIN_MESSAGE_STORE_LOAD_META_001, @CHAIN_MESSAGE_LOAD_SIGNED_FROM_CIDS_001, @CHAIN_MESSAGE_LOAD_UNSIGNED_FROM_CIDS_001
	rtMsgs, _, err := ms.LoadMetaMessages(ctx, msgsCid)
	assert.NoError(t, err)
	assert.Equal(t, signedMsgs, rtMsgs)

	// stm: @CHAIN_MESSAGE_READ_META_CID_001
	_, _, err = ms.ReadMsgMetaCids(ctx, msgsCid)
	assert.NoError(t, err)

	var notFoundCID cid.Cid
	testutil.Provide(t, &notFoundCID)

	{
		meta, err := ms.LoadTxMeta(ctx, msgsCid)
		require.NoError(t, err)

		as := cbor.NewCborStore(bs)

		var goodCid = signedMsgs[0].Cid()

		secpMsgArr := adt.MakeEmptyArray(adt.WrapStore(ctx, as))
		assert.NoError(t, secpMsgArr.Set(0, (*cbg.CborCid)(&goodCid)))
		blsMsgArr := adt.MakeEmptyArray(adt.WrapStore(ctx, as))
		assert.NoError(t, blsMsgArr.Set(0, (*cbg.CborCid)(&notFoundCID)))
		meta.SecpkRoot, err = secpMsgArr.Root()
		assert.NoError(t, err)
		meta.BlsRoot, err = blsMsgArr.Root()
		assert.NoError(t, err)

		// store a 'bad' message meta with bls(unsigned) message can't be found.
		metaCid, err := ms.StoreTxMeta(ctx, meta)
		require.NoError(t, err)

		// error occurs while load unsigned messages of cid array
		// stm: @CHAIN_MESSAGE_STORE_LOAD_META_005, @CHAIN_MESSAGE_STORE_LOAD_META_002
		_, _, err = ms.LoadMetaMessages(ctx, metaCid)
		require.Error(t, err)

		// store a 'bad' message meta with bad root of 'AMTCIDs'
		meta.BlsRoot = notFoundCID
		metaCid, err = ms.StoreTxMeta(ctx, meta)
		require.NoError(t, err)

		// stm: @CHAIN_MESSAGE_STORE_LOAD_META_003
		_, _, err = ms.LoadMetaMessages(ctx, metaCid)
		assert.Error(t, err)

		// stm: @CHAIN_MESSAGE_READ_META_CID_002, @CHAIN_MESSAGE_READ_META_CID_003
		_, _, err = ms.ReadMsgMetaCids(ctx, metaCid)
		require.Error(t, err)

		// store a 'bad' message meta with secp(signed) message can't be found.
		assert.NoError(t, secpMsgArr.Set(uint64(1), (*cbg.CborCid)(&notFoundCID)))
		meta.SecpkRoot, err = secpMsgArr.Root()
		require.NoError(t, err)
		metaCid, err = ms.StoreTxMeta(ctx, meta)
		require.NoError(t, err)

		// error occurs while loading signed messages of cir array
		// stm: @CHAIN_MESSAGE_STORE_LOAD_META_004, @CHAIN_MESSAGE_LOAD_SIGNED_003, @CHAIN_MESSAGE_LOAD_SIGNED_FROM_CIDS_002
		_, _, err = ms.LoadMetaMessages(ctx, metaCid)
		assert.Error(t, err)
	}
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

	// stm: @CHAIN_MESSAGE_LOAD_RECEIPTS_001
	rtReceipts, err := ms.LoadReceipts(ctx, receiptCids)
	assert.NoError(t, err)
	assert.Equal(t, receipts, rtReceipts)

	var badReceiptID cid.Cid
	// stm: CHAIN_MESSAGE_LOAD_RECEIPTS_002
	_, err = ms.LoadReceipts(ctx, badReceiptID)
	assert.Error(t, err)

	rectArr := adt.MakeEmptyArray(adt.WrapStore(ctx, cbor.NewCborStore(bs)))
	assert.NoError(t, rectArr.Set(0, cborString("invalid receipt data")))

	badReceiptID, err = rectArr.Root()
	assert.NoError(t, err)

	// expect unmarshal to receipt failed
	// stm: @CHAIN_MESSAGE_LOAD_RECEIPTS_003
	_, err = ms.LoadReceipts(ctx, badReceiptID)
	assert.Error(t, err)
}

func TestMessageStoreLoadMessage(t *testing.T) {
	testflags.UnitTest(t)
	ctx := context.Background()
	keys := testhelpers.MustGenerateKeyInfo(2, 42)
	mm := testhelpers.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	signedMsgs := []*types.SignedMessage{
		mm.NewSignedMessage(alice, 0),
		mm.NewSignedMessage(bob, 0),
	}

	unsignedMsgs := []*types.Message{
		mm.NewUnsignedMessage(alice, 4),
	}

	bs := blockstoreutil.Adapt(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	ms := chain.NewMessageStore(bs, config.DefaultForkUpgradeParam)
	_, err := ms.StoreMessages(ctx, signedMsgs, unsignedMsgs)
	assert.NoError(t, err)

	var notFoundCID cid.Cid
	testutil.Provide(t, &notFoundCID)

	// stm @CHAIN_MESSAGE_LOAD_MESSAGE_001, @CHAIN_MESSAGE_LOAD_SIGNED_001
	_, err = ms.LoadMessage(ctx, signedMsgs[0].Cid())
	assert.NoError(t, err)

	// stm: @CHAIN_MESSAGE_LOAD_UNSIGNED_001
	_, err = ms.LoadUnsignedMessage(ctx, unsignedMsgs[0].Cid())
	assert.NoError(t, err)

	// put a message with un-cbor-Unmarshal-able message data.
	badMsgCID, err := cbor.NewCborStore(bs).Put(ctx, (*cbg.CborCid)(&notFoundCID))
	assert.NoError(t, err)

	// Unmarshal to signed message failed error.
	// stm: @CHAIN_MESSAGE_LOAD_SIGNED_003
	_, err = ms.LoadMessage(ctx, badMsgCID)
	assert.Error(t, err)

	// If message is not found, return error; also getting message from blockstore failed in 'LoadUnsignedMessage'
	// stm: @CHAIN_MESSAGE_LOAD_MESSAGE_003, @CHAIN_MESSAGE_LOAD_SIGNED_002, @CHAIN_MESSAGE_LOAD_UNSIGNED_002
	_, err = ms.LoadMessage(ctx, notFoundCID)
	assert.Error(t, err)

	// Unmarshal to unsigned message failed error.
	// stm: @CHAIN_MESSAGE_LOAD_UNSIGNED_003, @CHAIN_MESSAGE_LOAD_UNSIGNED_FROM_CIDS_002
	_, err = ms.LoadUnsignedMessagesFromCids(ctx, []cid.Cid{badMsgCID})
	assert.Error(t, err)
}
