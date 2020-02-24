package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	spec_test "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"

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
	pchan := spec_test.NewIDAddr(t, 100)
	clientAddr := spec_test.NewIDAddr(t, 101)
	minerAddr := spec_test.NewIDAddr(t, 102)
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
		// simulate creating payment channel
		rmp.ActualPmtChans[pchan] = true
		rmp.ExpectedPmtChans[pchan] = &paymentchannel.ChannelInfo{
			Owner: clientAddr,
			State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
		}
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}
		rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)

		tokenamt, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.NoError(t, err)
		assert.True(t, voucher.Amount.Equals(tokenamt))
		rmp.Verify()
	})

	t.Run("errors if manager fails to save voucher", func(t *testing.T) {
		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		rmp.SaveVoucherErr = errors.New("boom")
		rmp.ActualPmtChans[pchan] = true
		rmp.ExpectedPmtChans[pchan] = &paymentchannel.ChannelInfo{
			Owner: clientAddr,
			State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
		}
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, ps, bs, rmp)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.EqualError(t, err, "boom")
	})
}
