// stm: #unit
package chain_test

import (
	"context"
	"io"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	blockstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
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

		goodCid := signedMsgs[0].Cid()

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

// Test randomized algorithm by trying all permutations
type AllPermutations struct {
	t *testing.T
	// current level
	level int
	// known domain (n) at current level
	domain []int
	// current iteration positions (i < n) in the domain
	dfs []int
}

func (p *AllPermutations) Reset() {
	// same outputs should result in same inputs
	assert.Equal(p.t, p.level, len(p.domain), "nondeterminism detected (fewer queries)")

	for len(p.dfs) > 0 && p.dfs[len(p.dfs)-1]+1 == p.domain[len(p.domain)-1] {
		// pop finished domains
		p.domain = p.domain[:len(p.domain)-1]
		p.dfs = p.dfs[:len(p.dfs)-1]
	}
	if len(p.dfs) > 0 {
		p.dfs[len(p.dfs)-1]++
	}
	// next iteration in the permutation
	p.level = 0
}

func (p *AllPermutations) Done() bool {
	return len(p.domain) == 0
}

func (p *AllPermutations) Intn(n int) int {
	assert.True(p.t, n > 0, "Intn(0)")
	if p.level < len(p.domain) {
		// same outputs should result in same inputs
		assert.Equal(p.t, p.domain[p.level], n, "nondeterminism detected (different queries)")
	} else {
		// expand search domain
		p.domain = append(p.domain, n)
		p.dfs = append(p.dfs, 0)
	}
	p.level++
	return p.dfs[p.level-1]
}

// TestWeightedQuickSelect tests the tipset gas percentile cases.
// BlockGasLimit = 10_000_000_000, P = 20.
func TestWeightedQuickSelect(t *testing.T) {
	tests := []struct {
		premiums []int64
		limits   []int64
		expected int64
	}{
		{[]int64{}, []int64{}, 0},
		{[]int64{123, 100}, []int64{5_999_999_999, 2_000_000_000}, 0},
		{[]int64{123, 0}, []int64{5_999_999_999, 2_000_000_001}, 0},
		{[]int64{123, 100}, []int64{5_999_999_999, 2_000_000_001}, 100},
		{[]int64{123, 100}, []int64{7_999_999_999, 2_000_000_001}, 100},
		{[]int64{123, 100}, []int64{8_000_000_000, 2_000_000_000}, 123},
		{[]int64{123, 100}, []int64{8_000_000_000, 9_000_000_000}, 123},
		{[]int64{100, 200, 300, 400, 500, 600, 700}, []int64{4_000_000_000, 1_000_000_000, 2_000_000_000, 1_000_000_000, 2_000_000_000, 2_000_000_000, 3_000_000_000}, 400},
	}
	for _, tc := range tests {
		premiums := make([]abi.TokenAmount, len(tc.premiums))
		for i, p := range tc.premiums {
			premiums[i] = big.NewInt(p)
		}
		rand := &AllPermutations{}
		rand.t = t
		for {
			got := chain.WeightedQuickSelectInternal(premiums, tc.limits, constants.BlockGasTargetIndex, rand)
			assert.Equal(t, big.NewInt(tc.expected).String(), got.String(),
				"premiums=%v limits=%v", tc.premiums, tc.limits)
			rand.Reset()
			if rand.Done() {
				break
			}
		}
	}
}

// TestNextBaseFeeFromPremium tests the BaseFee_next formula:
// MaxAdj = ceil(BaseFee / 8)
// BaseFee_next = Max(MinBaseFee, BaseFee + Min(MaxAdj, Premium_P - MaxAdj))
func TestNextBaseFeeFromPremium(t *testing.T) {
	tests := []struct {
		baseFee  int64
		premiumP int64
		expected int64
	}{
		{100, 0, 100},
		{100, 13, 100},
		{100, 14, 101},
		{100, 26, 113},
		{801, 0, 700},
		{801, 20, 720},
		{801, 40, 740},
		{801, 60, 760},
		{801, 80, 780},
		{801, 100, 800},
		{801, 120, 820},
		{801, 140, 840},
		{801, 160, 860},
		{801, 180, 880},
		{801, 200, 900},
		{801, 201, 901},
		{808, 0, 707},
		{808, 1, 708},
		{808, 201, 908},
		{808, 202, 909},
		{808, 203, 909},
	}
	for _, tc := range tests {
		got := chain.NextBaseFeeFromPremium(big.NewInt(tc.baseFee), big.NewInt(tc.premiumP))
		assert.Equal(t, big.NewInt(tc.expected).String(), got.String(),
			"baseFee=%d premiumP=%d", tc.baseFee, tc.premiumP)
	}
}
