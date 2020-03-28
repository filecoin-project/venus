package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-address"
	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	specst "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	paychtest "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel/testing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {
	tf.UnitTest(t)
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))

	pchMgr := makePaychMgr(context.Background(), t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"),
		abi.NewTokenAmount(10))
	rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr, nil)
	assert.NotZero(t, rpc)
}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	sectorID := rand.Uint64()
	fixtureFile := "../../../../../fixtures/constants.go"

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
	rmp := NewRetrievalMarketClientFakeAPI(t)
	pchMgr := makePaychMgr(ctx, t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"),
		abi.NewTokenAmount(10))
	rpc := NewRetrievalProviderConnector(rmnet, rmp, bs, pchMgr, nil)
	return rmp, rpc
}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	ctx := context.Background()

	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)

	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	pchan := specst.NewIDAddr(t, 100)
	clientAddr := specst.NewIDAddr(t, 101)
	minerAddr := specst.NewIDAddr(t, 102)
	root := gfmtut.GenerateCids(1)[0]
	tsk := block.NewTipSetKey(root)
	tok, err := encoding.Encode(tsk)
	require.NoError(t, err)

	voucher := &paych.SignedVoucher{
		Lane:            rand.Uint64(),
		Nonce:           rand.Uint64(),
		Amount:          big.NewInt(rand.Int63()),
		MinSettleHeight: abi.ChainEpoch(99),
		SecretPreimage:  []byte{},
	}
	proof := []byte("proof")

	t.Run("saves payment voucher and returns voucher amount if new", func(t *testing.T) {
		viewer, pchMgr := makeViewerAndManager(ctx, t, clientAddr, minerAddr, pchan, root)
		viewer.AddActorWithState(pchan, clientAddr, minerAddr, address.Undef)
		rmp := NewRetrievalMarketClientFakeAPI(t)
		// simulate creating payment channel
		rmp.ExpectedVouchers[pchan] = &pch.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr, nil)

		tokenamt, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount, tok)
		assert.NoError(t, err)
		assert.True(t, voucher.Amount.Equals(tokenamt))

		chinfo, err := pchMgr.GetPaymentChannelInfo(pchan)
		require.NoError(t, err)
		assert.True(t, chinfo.HasVoucher(voucher))
		rmp.Verify()
	})

	t.Run("errors if manager fails to save voucher, does not store new channel info", func(t *testing.T) {
		viewer, pchMgr := makeViewerAndManager(ctx, t, clientAddr, minerAddr, pchan, root)
		viewer.AddActorWithState(pchan, clientAddr, minerAddr, address.Undef)
		viewer.PaychActorPartiesErr = errors.New("boom")

		rmp := NewRetrievalMarketClientFakeAPI(t)
		rmp.ExpectedVouchers[pchan] = &pch.VoucherInfo{Voucher: voucher, Proof: proof}
		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr, nil)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount, tok)
		assert.EqualError(t, err, "boom")

		_, err = pchMgr.GetPaymentChannelInfo(pchan)
		require.EqualError(t, err, "No state for /t0100: datastore: key not found")
	})
}

func makeViewerAndManager(ctx context.Context, t *testing.T, client, miner, paych address.Address, root state.Root) (*paychtest.FakeStateViewer, *pch.Manager) {
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
	viewer := paychtest.NewFakeStateViewer(t)
	pchMgr := pch.NewManager(context.Background(), ds, testAPI, testAPI, viewer)
	blockHeight := uint64(1234)
	balance := types.NewAttoFILFromFIL(1000)

	testAPI.ExpectedMsgCid, testAPI.ExpectedResult = paychtest.GenCreatePaychActorMessage(t, client, miner, paych, balance, exitcode.Ok, blockHeight)
	return viewer, pchMgr
}
