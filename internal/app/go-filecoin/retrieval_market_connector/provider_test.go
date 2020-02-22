package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	ps := gfmtut.NewTestPieceStore()
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
	rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)
	assert.NotZero(t, rpc)
}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {

}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	ps := gfmtut.NewTestPieceStore()
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	pchan, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	ctx := context.Background()

	voucher := &paych.SignedVoucher{
		Lane:            rand.Uint64(),
		Nonce:           rand.Uint64(),
		Amount:          big.NewInt(rand.Int63()),
		MinSettleHeight: abi.ChainEpoch(99),
	}
	proof := []byte("proof")

	t.Run("saves payment voucher and returns voucher amount if new", func(t *testing.T) {
		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}
		rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)

		tokenamt, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.NoError(t, err)
		assert.True(t, voucher.Amount.Equals(tokenamt))
		rmp.Verify()
	})

	t.Run("errors and does not overwrite voucher if already saved, regardless of other params", func(t *testing.T) {
		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		require.NoError(t, err)
		proof2 := []byte("newproof")
		pchan2, err := address.NewIDAddress(rand.Uint64())
		require.NoError(t, err)
		tokenamt2, err := rpc.SavePaymentVoucher(ctx, pchan2, voucher, proof2, voucher.Amount)
		assert.EqualError(t, err, "voucher exists")
		assert.Equal(t, abi.NewTokenAmount(0), tokenamt2)
	})
	t.Run("errors if manager fails to save voucher", func(t *testing.T) {
		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		rmp.SaveVoucherErr = errors.New("boom")
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.EqualError(t, err, "boom")
	})
}
