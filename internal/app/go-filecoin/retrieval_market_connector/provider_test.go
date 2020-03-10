package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
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
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {
	tf.UnitTest(t)
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
	rpc := NewRetrievalProviderConnector(rmnet, pm, bs, rmp)
	assert.NotZero(t, rpc)
}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	sectorID := rand.Uint64()
	fixtureFile := "../../../../fixtures/constants.go"

	intSz := reflect.TypeOf(0).Size()*8 - 1
	maxOffset := uint64(1 << intSz)

	testCases := []struct {
		name                        string
		offset, length, expectedLen uint64
		unsealErr                   error
		expectedErr                 string
	}{
		{name: "happy path", offset: 10, length: 50, expectedLen: 50, expectedErr: ""},
		{name: "happy even if length more than file length", offset: 10, length: 9999, expectedLen: 4086, expectedErr: ""},
		{name: "returns error if Unseal errors", unsealErr: errors.New("boom"), expectedErr: "boom"},
		{name: "returns EOF if offset more than file length", offset: 9999, expectedErr: "EOF"},
		{name: "returns error if offset > int64", offset: maxOffset, expectedErr: "offset overflows int"},
		{name: "returns error if length > int64", length: 1 << 63, expectedErr: "length overflows int64"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rmp, rpc := unsealTestSetup(ctx, t)
			rmp.ExpectedSectorIDs[sectorID] = fixtureFile

			if tc.expectedErr != "" {
				rmp.UnsealErr = tc.unsealErr
				_, err := rpc.UnsealSector(ctx, sectorID, tc.offset, tc.length)
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				res, err := rpc.UnsealSector(ctx, sectorID, tc.offset, tc.length)
				require.NoError(t, err)
				readBytes := make([]byte, tc.length+1)
				readlen, err := res.Read(readBytes)
				require.NoError(t, err)
				assert.Equal(t, int(tc.expectedLen), readlen)

				// check that it read something & the offset worked
				assert.Equal(t, "xtures", string(readBytes[0:6]))
				rmp.Verify()
			}
		})
	}
}

func unsealTestSetup(ctx context.Context, t *testing.T) (*RetrievalMarketClientFakeAPI, *RetrievalProviderConnector) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
	rpc := NewRetrievalProviderConnector(rmnet, rmp, bs, rmp)
	return rmp, rpc
}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	tf.UnitTest(t)
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)

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
			From: clientAddr, To: minerAddr,
		}
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}
		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, rmp)

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
			From: clientAddr, To: minerAddr,
		}
		rmp.ExpectedVouchers[pchan] = &paymentchannel.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, rmp)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.EqualError(t, err, "boom")
	})

	t.Run("errors if voucher already stored", func(t *testing.T) {})
}
